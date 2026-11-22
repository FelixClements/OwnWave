package main

import (
	"context"
	"crypto/rand"
	"log/slog"
	"strings"
	"sync"
)

// minSetupTokenLen is the shortest OWNWAVE_SETUP_TOKEN accepted without a
// warning.
const minSetupTokenLen = 16

// setupToken is the secret that creates the first (admin) account while no
// account exists.
type setupToken struct {
	value string
	// generated: made at startup because OWNWAVE_SETUP_TOKEN was unset.
	generated bool
	announce  sync.Once
}

func newSetupToken(configured string) *setupToken {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		if len(configured) < minSetupTokenLen {
			slog.Warn("OWNWAVE_SETUP_TOKEN is short; use at least 16 random characters", "length", len(configured))
		}
		return &setupToken{value: configured}
	}
	return &setupToken{value: rand.Text(), generated: true}
}

// announceOnce tells the operator, once per run, how to create the first
// account. A generated token is written to the log, as it exists nowhere
// else; a configured one never is.
func (t *setupToken) announceOnce() {
	if t == nil {
		return
	}
	t.announce.Do(func() {
		if t.generated {
			slog.Warn("SETUP TOKEN: no accounts exist yet. Enter this token on the setup page to create the admin account. "+
				"A new token is made at each restart; set OWNWAVE_SETUP_TOKEN to choose your own.",
				"setup_token", t.value)
			return
		}
		slog.Info("no accounts exist yet: create the admin account on the setup page with the OWNWAVE_SETUP_TOKEN value")
	})
}

// announceSetupTokenIfOpen announces the setup token when no account exists.
func (h *Handler) announceSetupTokenIfOpen(ctx context.Context) {
	count, err := h.auth.CountUsers(ctx)
	if err != nil {
		slog.Error("setup token: count users", "error", err)
		return
	}
	if count == 0 {
		h.setup.announceOnce()
	}
}
