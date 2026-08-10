package main

import (
	"encoding/hex"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestHashToken(t *testing.T) {
	tok := "test-token"
	h1 := hashToken(tok)
	h2 := hashToken(tok)
	if h1 != h2 {
		t.Errorf("hashToken not deterministic: %q vs %q", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("hashToken length = %d, want 64", len(h1))
	}
	if _, err := hex.DecodeString(h1); err != nil {
		t.Errorf("hashToken not hex: %v", err)
	}
}

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
