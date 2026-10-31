package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials  = errors.New("invalid username or password")
	ErrInvalidToken        = errors.New("invalid token")
	ErrTokenExpired        = errors.New("token has expired")
	ErrMissingEnvVars      = errors.New("missing required environment variables")
	ErrInvalidPasswordHash = errors.New("ADMIN_PASSWORD must be a valid bcrypt hash. Generate one with: htpasswd -bnBC 10 '' yourPassword | tr -d ':'")
)

// User roles. They mean the same as API token scopes, so the same checks gate
// a read-only user and a read-scoped token.
const (
	RoleAdmin = APITokenScopeAdmin
	RoleRead  = APITokenScopeRead
)

type Service struct {
	jwtSecret []byte
	// accounts maps a username to its local account. It is built once per
	// config change and never modified, so requests read it without locking.
	accounts map[string]account
	// envAdmin is the username of the admin set by ADMIN_USERNAME, if any.
	envAdmin        string
	oidc            *oidcClient // nil when OIDC sign-in is off
	tokenExpiration time.Duration
}

type account struct {
	passwordHash string
	salt         string
	role         string
	// stamp ties a session to the password it was issued under, so resetting
	// the password ends the sessions signed in with the old one.
	stamp string
}

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	// Stamp must match the account's current stamp for the session to count.
	Stamp string `json:"stamp,omitempty"`
	// OIDC marks a session signed in through the OIDC provider rather than
	// with a local account.
	OIDC bool `json:"oidc,omitempty"`
	jwt.RegisteredClaims
}

// NewService builds the auth service. JWT_SECRET, ADMIN_USERNAME and
// ADMIN_PASSWORD configure auth with a fixed admin; without them the config
// file's auth section does. The file's users and the given OIDC settings
// apply either way. Returns nil (no error) when auth is disabled.
func NewService(fileAuth *config.FileAuthConfig, oidcCfg *config.OIDCConfig) (*Service, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	adminUsername := os.Getenv("ADMIN_USERNAME")
	adminPasswordHash := os.Getenv("ADMIN_PASSWORD")
	sha256Salt := os.Getenv("ADMIN_PASSWORD_SALT")

	var s *Service
	switch {
	case jwtSecret == "" && adminUsername == "" && (adminPasswordHash == "" && sha256Salt == ""):
		if fileAuth == nil || !fileAuth.Enabled || fileAuth.JWTSecret == "" {
			return nil, nil
		}
		s = newService(fileAuth.JWTSecret)

	// If some but not all are set, return an error
	case jwtSecret == "" || adminUsername == "" || (adminPasswordHash == "" && sha256Salt == ""):
		return nil, ErrMissingEnvVars

	default:
		// Without a salt, the password must be a bcrypt hash; validate it up
		// front so a malformed hash fails at startup instead of on every login.
		if sha256Salt == "" {
			if _, err := bcrypt.Cost([]byte(adminPasswordHash)); err != nil {
				return nil, ErrInvalidPasswordHash
			}
		}
		s = newService(jwtSecret)
		s.envAdmin = adminUsername
		s.addAccount(adminUsername, adminPasswordHash, sha256Salt, RoleAdmin)
	}

	if fileAuth != nil {
		for _, u := range fileAuth.Users {
			// The env admin wins a name clash, as env settings do elsewhere.
			if _, taken := s.accounts[u.Username]; !taken {
				s.addAccount(u.Username, u.PasswordHash, u.PasswordSalt, u.Role)
			}
		}
	}
	if oidcCfg != nil && oidcCfg.Enabled {
		s.oidc = newOIDCClient(*oidcCfg, s)
	}
	return s, nil
}

func newService(jwtSecret string) *Service {
	return &Service{
		jwtSecret:       []byte(jwtSecret),
		accounts:        make(map[string]account),
		tokenExpiration: 7 * 24 * time.Hour, // 7 days
	}
}

func (s *Service) addAccount(username, passwordHash, salt, role string) {
	s.accounts[username] = account{
		passwordHash: passwordHash,
		salt:         salt,
		role:         normalizeRole(role),
		stamp:        s.stamp(passwordHash, salt),
	}
}

// stamp derives a short, secret-keyed fingerprint of its parts.
func (s *Service) stamp(parts ...string) string {
	mac := hmac.New(sha256.New, s.jwtSecret)
	mac.Write([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(mac.Sum(nil)[:8])
}

// normalizeRole fails closed: anything but "admin" (a typo in a hand-edited
// config, say) is read-only.
func normalizeRole(role string) string {
	if role == RoleAdmin {
		return RoleAdmin
	}
	return RoleRead
}

// EnvAdmin returns the username of the admin configured by ADMIN_USERNAME, or
// "" when auth comes from the config file.
func (s *Service) EnvAdmin() string {
	return s.envAdmin
}

// dummyHash is checked when there is no bcrypt hash to check, so every login
// attempt costs the same and the response time never reveals whether a
// username exists.
var dummyHash = sync.OnceValue(func() []byte {
	hash, _ := bcrypt.GenerateFromPassword([]byte("logdeck"), bcrypt.DefaultCost)
	return hash
})

// ValidateCredentials checks a username and password against the local
// accounts and returns the user they sign in. Bcrypt hashes and the legacy
// SHA256+salt hashes are both supported. An unknown username and a wrong
// password fail alike.
func (s *Service) ValidateCredentials(username, password string) (models.User, error) {
	acct, ok := s.accounts[username]

	var passwordMatch bool
	if ok && isBcryptHash(acct.passwordHash) {
		passwordMatch = bcrypt.CompareHashAndPassword([]byte(acct.passwordHash), []byte(password)) == nil
	} else {
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
		if ok {
			hash := HashPasswordSHA256(password, acct.salt)
			passwordMatch = subtle.ConstantTimeCompare([]byte(hash), []byte(acct.passwordHash)) == 1
		}
	}

	if !passwordMatch {
		return models.User{}, ErrInvalidCredentials
	}
	return models.User{Username: username, Role: acct.role}, nil
}

func isBcryptHash(hash string) bool {
	return strings.HasPrefix(hash, "$2a$") ||
		strings.HasPrefix(hash, "$2b$") ||
		strings.HasPrefix(hash, "$2y$")
}

// GenerateToken creates a session token for a local account.
func (s *Service) GenerateToken(username string) (string, error) {
	acct, ok := s.accounts[username]
	if !ok {
		return "", ErrInvalidCredentials
	}
	return s.signSession(Claims{Username: username, Role: acct.role, Stamp: acct.stamp})
}

func (s *Service) signSession(claims Claims) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(s.tokenExpiration)),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Issuer:    "logdeck",
		Subject:   claims.Username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	return token.SignedString(s.jwtSecret)
}

// VerifyToken validates a JWT token and returns the claims
func (s *Service) VerifyToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// SessionUser resolves verified claims to the user they belong to now. A
// local session takes its role from the current account, not the token, so
// deleting a user, changing their role or resetting their password applies
// on their next request. An OIDC session ends when OIDC is turned off or its
// settings change. It is a map lookup, cheap enough for every request.
func (s *Service) SessionUser(claims *Claims) (models.User, bool) {
	var stamp, role string
	if claims.OIDC {
		if s.oidc == nil {
			return models.User{}, false
		}
		stamp, role = s.oidc.stamp, normalizeRole(claims.Role)
	} else {
		acct, ok := s.accounts[claims.Username]
		if !ok {
			return models.User{}, false
		}
		stamp, role = acct.stamp, acct.role
	}
	if subtle.ConstantTimeCompare([]byte(claims.Stamp), []byte(stamp)) != 1 {
		return models.User{}, false
	}
	return models.User{Username: claims.Username, Role: role}, true
}

// HashPassword generates a bcrypt hash from a plain password
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// HashPasswordSHA256 computes SHA256(password + salt) and returns the hex-encoded hash.
func HashPasswordSHA256(password, salt string) string {
	h := sha256.Sum256([]byte(password + salt))
	return hex.EncodeToString(h[:])
}

// GenerateRandomHex generates a cryptographically random hex string of the given byte length.
func GenerateRandomHex(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
