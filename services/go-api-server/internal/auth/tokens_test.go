package auth

import (
	"encoding/hex"
	"testing"
)

func TestHashToken(t *testing.T) {
	tok := "test-token"
	h1 := HashToken(tok)
	h2 := HashToken(tok)
	if h1 != h2 {
		t.Errorf("HashToken not deterministic: %q vs %q", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("HashToken length = %d, want 64", len(h1))
	}
	if _, err := hex.DecodeString(h1); err != nil {
		t.Errorf("HashToken not hex: %v", err)
	}
}
