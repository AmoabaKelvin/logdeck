package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"os"
	"strings"
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

type Service struct {
	jwtSecret         []byte
	adminUsername     string
	adminPasswordHash string
	sha256Salt        string
	tokenExpiration   time.Duration
}

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// NewService creates a new auth service
// Returns nil (no error) if auth environment variables are not set, indicating auth is disabled
func NewService() (*Service, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	adminUsername := os.Getenv("ADMIN_USERNAME")
	adminPasswordHash := os.Getenv("ADMIN_PASSWORD")
	sha256Salt := os.Getenv("ADMIN_PASSWORD_SALT")

	// If none of the auth variables are set, return nil to indicate auth is disabled
	if jwtSecret == "" && adminUsername == "" && (adminPasswordHash == "" && sha256Salt == "") {
		return nil, nil
	}

	// If some but not all are set, return an error
	if jwtSecret == "" || adminUsername == "" || (adminPasswordHash == "" && sha256Salt == "") {
		return nil, ErrMissingEnvVars
	}

	// Without a salt, the password must be a bcrypt hash; validate it up
	// front so a malformed hash fails at startup instead of on every login.
	if sha256Salt == "" {
		if _, err := bcrypt.Cost([]byte(adminPasswordHash)); err != nil {
			return nil, ErrInvalidPasswordHash
		}
	}

	return &Service{
		jwtSecret:         []byte(jwtSecret),
		adminUsername:     adminUsername,
		adminPasswordHash: adminPasswordHash,
		sha256Salt:        sha256Salt,
		tokenExpiration:   7 * 24 * time.Hour, // 7 days
	}, nil
}

// ValidateCredentials checks if the provided credentials are valid.
// Bcrypt hashes (env-configured) and SHA256+salt hashes (file-configured)
// are both supported.
func (s *Service) ValidateCredentials(username, password string) error {
	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(s.adminUsername)) == 1

	var passwordMatch bool
	if isBcryptHash(s.adminPasswordHash) {
		passwordMatch = bcrypt.CompareHashAndPassword([]byte(s.adminPasswordHash), []byte(password)) == nil
	} else {
		hash := HashPasswordSHA256(password, s.sha256Salt)
		passwordMatch = subtle.ConstantTimeCompare([]byte(hash), []byte(s.adminPasswordHash)) == 1
	}

	if !usernameMatch || !passwordMatch {
		return ErrInvalidCredentials
	}

	return nil
}

func isBcryptHash(hash string) bool {
	return strings.HasPrefix(hash, "$2a$") ||
		strings.HasPrefix(hash, "$2b$") ||
		strings.HasPrefix(hash, "$2y$")
}

// GenerateToken creates a new JWT token for the user
func (s *Service) GenerateToken(username string) (string, error) {
	now := time.Now()
	expirationTime := now.Add(s.tokenExpiration)

	claims := &Claims{
		Username: username,
		Role:     "admin", // For now, all users are admins
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "logdeck",
			Subject:   username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
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

// GetUserFromClaims extracts user information from claims
func GetUserFromClaims(claims *Claims) models.User {
	return models.User{
		Username: claims.Username,
		Role:     claims.Role,
	}
}

// HashPassword generates a bcrypt hash from a plain password
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// NewServiceFromFileConfig creates an auth service from file-based config.
// Returns nil if auth is not enabled.
func NewServiceFromFileConfig(cfg *config.FileAuthConfig) *Service {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	if cfg.JWTSecret == "" || cfg.AdminUsername == "" || cfg.AdminPasswordHash == "" {
		return nil
	}
	return &Service{
		jwtSecret:         []byte(cfg.JWTSecret),
		adminUsername:     cfg.AdminUsername,
		adminPasswordHash: cfg.AdminPasswordHash,
		sha256Salt:        cfg.AdminPasswordSalt,
		tokenExpiration:   7 * 24 * time.Hour,
	}
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
