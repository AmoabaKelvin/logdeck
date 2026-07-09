package auth

import (
	"errors"
	"testing"
)

func TestValidateCredentialsBcrypt(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	s := &Service{
		adminUsername:     "admin",
		adminPasswordHash: hash,
	}

	if err := s.ValidateCredentials("admin", "correct-password"); err != nil {
		t.Errorf("expected valid bcrypt credentials to pass, got: %v", err)
	}

	if err := s.ValidateCredentials("admin", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for wrong password, got: %v", err)
	}

	if err := s.ValidateCredentials("intruder", "correct-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for wrong username, got: %v", err)
	}
}

func TestValidateCredentialsSHA256(t *testing.T) {
	salt := "test-salt"
	s := &Service{
		adminUsername:     "admin",
		adminPasswordHash: HashPasswordSHA256("correct-password", salt),
		sha256Salt:        salt,
	}

	if err := s.ValidateCredentials("admin", "correct-password"); err != nil {
		t.Errorf("expected valid SHA256 credentials to pass, got: %v", err)
	}

	if err := s.ValidateCredentials("admin", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for wrong password, got: %v", err)
	}

	if err := s.ValidateCredentials("intruder", "correct-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for wrong username, got: %v", err)
	}
}

func TestNewServiceRejectsMalformedBcryptHash(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD_SALT", "")

	t.Setenv("ADMIN_PASSWORD", "not-a-bcrypt-hash")
	if _, err := NewService(); !errors.Is(err, ErrInvalidPasswordHash) {
		t.Errorf("expected ErrInvalidPasswordHash for malformed hash, got: %v", err)
	}

	hash, err := HashPassword("password")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	t.Setenv("ADMIN_PASSWORD", hash)
	svc, err := NewService()
	if err != nil {
		t.Errorf("expected valid bcrypt hash to be accepted, got: %v", err)
	}
	if svc == nil {
		t.Error("expected a service for a valid bcrypt hash")
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
