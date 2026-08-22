package streamauth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestGetString(t *testing.T) {
	claims := jwt.MapClaims{
		"track_id": "abc",
		"number":   float64(42),
	}
	if got := getString(claims, "track_id"); got != "abc" {
		t.Errorf("getString(track_id) = %q, want abc", got)
	}
	if got := getString(claims, "number"); got != "42" {
		t.Errorf("getString(number) = %q, want 42", got)
	}
	if got := getString(claims, "missing"); got != "" {
		t.Errorf("getString(missing) = %q, want empty", got)
	}
}
