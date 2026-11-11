package main

import (
	"strings"
	"sync"
	"time"
)

// loginThrottle limits failed logins per username, independent of the
// per-IP limiter, so spreading guesses across many addresses does not help.
// A nil *loginThrottle allows everything.
type loginThrottle struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	failures map[string]*loginFailures
	now      func() time.Time
}

type loginFailures struct {
	count int
	reset time.Time
}

// maxTrackedUsernames bounds memory; expired entries are pruned past it.
const maxTrackedUsernames = 10000

func newLoginThrottle(max int, window time.Duration) *loginThrottle {
	return &loginThrottle{max: max, window: window, failures: map[string]*loginFailures{}, now: time.Now}
}

func throttleKey(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// Blocked reports whether username has used up its failed attempts.
func (t *loginThrottle) Blocked(username string) bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	f, ok := t.failures[throttleKey(username)]
	if !ok {
		return false
	}
	if !t.now().Before(f.reset) {
		delete(t.failures, throttleKey(username))
		return false
	}
	return f.count >= t.max
}

// Fail records a failed attempt for username.
func (t *loginThrottle) Fail(username string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	key := throttleKey(username)
	f, ok := t.failures[key]
	if !ok || !now.Before(f.reset) {
		if len(t.failures) >= maxTrackedUsernames {
			t.pruneLocked(now)
		}
		f = &loginFailures{reset: now.Add(t.window)}
		t.failures[key] = f
	}
	f.count++
}

// Succeed clears the failures for username.
func (t *loginThrottle) Succeed(username string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.failures, throttleKey(username))
}

func (t *loginThrottle) pruneLocked(now time.Time) {
	for k, f := range t.failures {
		if !now.Before(f.reset) {
			delete(t.failures, k)
		}
	}
}
