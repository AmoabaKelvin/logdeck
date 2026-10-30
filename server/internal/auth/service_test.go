package auth

import (
	"errors"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/config"
)

// clearAuthEnv unsets the auth env vars so NewService reads only its
// arguments.
func clearAuthEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD", "ADMIN_PASSWORD_SALT"} {
		t.Setenv(key, "")
	}
}

// newFileService builds a service from a file auth section holding users.
func newFileService(t *testing.T, users ...config.User) *Service {
	t.Helper()
	clearAuthEnv(t)
	svc, err := NewService(&config.FileAuthConfig{Enabled: true, JWTSecret: "test-secret", Users: users}, nil)
	if err != nil || svc == nil {
		t.Fatalf("NewService = %v, %v", svc, err)
	}
	return svc
}

func TestValidateCredentialsBcrypt(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	s := newFileService(t, config.User{Username: "admin", PasswordHash: hash, Role: RoleAdmin})

	if user, err := s.ValidateCredentials("admin", "correct-password"); err != nil || user.Role != RoleAdmin {
		t.Errorf("expected valid bcrypt credentials to sign in an admin, got %+v, %v", user, err)
	}

	if _, err := s.ValidateCredentials("admin", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for wrong password, got: %v", err)
	}

	if _, err := s.ValidateCredentials("intruder", "correct-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for wrong username, got: %v", err)
	}
}

func TestValidateCredentialsSHA256(t *testing.T) {
	salt := "test-salt"
	s := newFileService(t, config.User{Username: "admin", PasswordHash: HashPasswordSHA256("correct-password", salt), PasswordSalt: salt, Role: RoleAdmin})

	if _, err := s.ValidateCredentials("admin", "correct-password"); err != nil {
		t.Errorf("expected valid SHA256 credentials to pass, got: %v", err)
	}

	if _, err := s.ValidateCredentials("admin", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for wrong password, got: %v", err)
	}

	if _, err := s.ValidateCredentials("intruder", "correct-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for wrong username, got: %v", err)
	}
}

func TestNewServiceRejectsMalformedBcryptHash(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD_SALT", "")

	t.Setenv("ADMIN_PASSWORD", "not-a-bcrypt-hash")
	if _, err := NewService(nil, nil); !errors.Is(err, ErrInvalidPasswordHash) {
		t.Errorf("expected ErrInvalidPasswordHash for malformed hash, got: %v", err)
	}

	hash, err := HashPassword("password")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	t.Setenv("ADMIN_PASSWORD", hash)
	svc, err := NewService(nil, nil)
	if err != nil {
		t.Errorf("expected valid bcrypt hash to be accepted, got: %v", err)
	}
	if svc == nil {
		t.Error("expected a service for a valid bcrypt hash")
	}
}

func TestNewServiceEnvAdminAlongsideFileUsers(t *testing.T) {
	envHash, _ := HashPassword("env-password")
	fileHash, _ := HashPassword("file-password")
	t.Setenv("JWT_SECRET", "env-secret")
	t.Setenv("ADMIN_USERNAME", "root")
	t.Setenv("ADMIN_PASSWORD", envHash)
	t.Setenv("ADMIN_PASSWORD_SALT", "")

	// The file's enabled flag and secret are ignored under env auth; its users
	// are not. A file user named like the env admin is shadowed.
	svc, err := NewService(&config.FileAuthConfig{Users: []config.User{
		{Username: "root", PasswordHash: fileHash, Role: RoleRead},
		{Username: "viewer", PasswordHash: fileHash, Role: RoleRead},
	}}, nil)
	if err != nil || svc == nil {
		t.Fatalf("NewService = %v, %v", svc, err)
	}
	if svc.EnvAdmin() != "root" {
		t.Errorf("EnvAdmin = %q, want root", svc.EnvAdmin())
	}
	if user, err := svc.ValidateCredentials("root", "env-password"); err != nil || user.Role != RoleAdmin {
		t.Errorf("env admin sign-in = %+v, %v", user, err)
	}
	if _, err := svc.ValidateCredentials("root", "file-password"); err == nil {
		t.Error("a file user must not shadow the env admin")
	}
	if user, err := svc.ValidateCredentials("viewer", "file-password"); err != nil || user.Role != RoleRead {
		t.Errorf("file user sign-in = %+v, %v", user, err)
	}
}

func TestNewServiceDisabled(t *testing.T) {
	clearAuthEnv(t)
	for _, fa := range []*config.FileAuthConfig{nil, {Enabled: false, JWTSecret: "s"}} {
		if svc, err := NewService(fa, nil); svc != nil || err != nil {
			t.Errorf("NewService(%+v) = %v, %v; want nil, nil", fa, svc, err)
		}
	}
}

// A session's role and validity come from the current accounts, not the
// token, so admin changes apply on the user's next request.
func TestSessionUserFollowsAccountChanges(t *testing.T) {
	hash, _ := HashPassword("password")
	alice := config.User{Username: "alice", PasswordHash: hash, Role: RoleAdmin}
	before := newFileService(t, alice)
	token, err := before.GenerateToken("alice")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	sessionUser := func(svc *Service) (string, bool) {
		claims, err := svc.VerifyToken(token)
		if err != nil {
			t.Fatalf("VerifyToken: %v", err)
		}
		user, ok := svc.SessionUser(claims)
		return user.Role, ok
	}

	if role, ok := sessionUser(before); !ok || role != RoleAdmin {
		t.Fatalf("fresh session = %q, %v", role, ok)
	}

	demoted := alice
	demoted.Role = RoleRead
	if role, ok := sessionUser(newFileService(t, demoted)); !ok || role != RoleRead {
		t.Errorf("after demotion = %q, %v; want read", role, ok)
	}

	reset := alice
	reset.PasswordHash, _ = HashPassword("new-password")
	if _, ok := sessionUser(newFileService(t, reset)); ok {
		t.Error("a password reset must end the old sessions")
	}

	if _, ok := sessionUser(newFileService(t)); ok {
		t.Error("a deleted user's session must stop working")
	}
}

func TestSessionUserRejectsTokenWithoutStamp(t *testing.T) {
	hash, _ := HashPassword("password")
	svc := newFileService(t, config.User{Username: "admin", PasswordHash: hash, Role: RoleAdmin})
	// A session issued before stamps existed carries none.
	token, err := svc.signSession(Claims{Username: "admin", Role: RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := svc.VerifyToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.SessionUser(claims); ok {
		t.Error("a token without a stamp must not count")
	}
}

func TestIsBcryptHash(t *testing.T) {
	for _, hash := range []string{"$2a$10$abc", "$2b$10$abc", "$2y$10$abc"} {
		if !isBcryptHash(hash) {
			t.Errorf("expected %q to be detected as bcrypt", hash)
		}
	}
	for _, hash := range []string{"", "abc123", "$1$notbcrypt", "e3b0c44298fc1c149afbf4c8996fb924"} {
		if isBcryptHash(hash) {
			t.Errorf("expected %q to not be detected as bcrypt", hash)
		}
	}
}
