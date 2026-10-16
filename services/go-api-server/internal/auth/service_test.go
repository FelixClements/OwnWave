package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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

func TestValidatePasswordBounds(t *testing.T) {
	require.ErrorIs(t, validatePassword("short"), ErrWeakPassword)
	require.ErrorIs(t, validatePassword(strings.Repeat("a", 73)), ErrPasswordTooLong)
	require.NoError(t, validatePassword(strings.Repeat("a", 72)))
	require.NoError(t, validatePassword("password123"))
}

func TestChangePasswordRejectsTooLongBeforeDB(t *testing.T) {
	s := &Service{}
	_, err := s.ChangePassword(context.Background(), User{ID: "1"}, "irrelevant", strings.Repeat("a", 100))
	require.ErrorIs(t, err, ErrPasswordTooLong)
}
