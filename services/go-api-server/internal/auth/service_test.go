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

func TestValidateUsername(t *testing.T) {
	for _, ok := range []string{"alice", "a", "Bob_2", "j.doe-99", strings.Repeat("a", 32)} {
		require.NoError(t, ValidateUsername(ok), ok)
	}
	for _, bad := range []string{"", "admin ", " admin", "a b", "_admin", ".x", "admin​", "ädmin", "a@b", strings.Repeat("a", 33)} {
		require.ErrorIs(t, ValidateUsername(bad), ErrInvalidUsername, bad)
	}
	require.Equal(t, "admin", NormalizeUsername("  admin \t"))
}

func TestNormalizeProfile(t *testing.T) {
	email, name, err := NormalizeProfile("  a@example.com ", "  Jane Doe  ")
	require.NoError(t, err)
	require.Equal(t, "a@example.com", email)
	require.Equal(t, "Jane Doe", name)

	email, name, err = NormalizeProfile("", "")
	require.NoError(t, err)
	require.Empty(t, email)
	require.Empty(t, name)

	for _, bad := range []string{"not-an-email", "Jane <a@example.com>", strings.Repeat("a", 250) + "@x.io"} {
		_, _, err := NormalizeProfile(bad, "")
		require.ErrorIs(t, err, ErrInvalidEmail, bad)
	}
	for _, bad := range []string{strings.Repeat("é", 101), "line\nbreak", "bell\a"} {
		_, _, err := NormalizeProfile("", bad)
		require.ErrorIs(t, err, ErrInvalidFullName, bad)
	}
	_, _, err = NormalizeProfile("", strings.Repeat("é", 100))
	require.NoError(t, err)
}

func TestValidateCredentialsNormalizesBeforeCheck(t *testing.T) {
	// Register and CreateUser trim first; an untrimmed name is not valid as is.
	require.ErrorIs(t, validateCredentials("admin ", "password1"), ErrInvalidUsername)
	require.NoError(t, validateCredentials(NormalizeUsername("admin "), "password1"))
}
