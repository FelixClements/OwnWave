package auth

import (
	"context"
	"errors"
	"testing"
)

func TestSetAdminRefusesOwnAccount(t *testing.T) {
	s := NewService(nil)
	if _, err := s.SetAdmin(context.Background(), "u1", "u1", false); !errors.Is(err, ErrCannotChangeOwnAdmin) {
		t.Fatalf("want ErrCannotChangeOwnAdmin, got %v", err)
	}
}
