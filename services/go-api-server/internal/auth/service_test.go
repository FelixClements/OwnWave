package auth

import "testing"

func TestValidateCredentials(t *testing.T) {
	if err := validateCredentials("alice", "password1"); err != nil {
		t.Fatalf("expected valid credentials, got %v", err)
	}
	if err := validateCredentials("", "password1"); err != ErrInvalidUsername {
		t.Fatalf("expected ErrInvalidUsername, got %v", err)
	}
	if err := validateCredentials("alice", "short"); err != ErrWeakPassword {
		t.Fatalf("expected ErrWeakPassword, got %v", err)
	}
}
