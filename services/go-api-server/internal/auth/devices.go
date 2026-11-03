package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	// deviceTTL is how long an unused device token stays valid; every use
	// (at most once per touchInterval) restarts it.
	deviceTTL     = 365 * 24 * time.Hour
	touchInterval = time.Hour
	maxDeviceName = 80
)

// ErrDeviceNotFound is returned when revoking a device the user does not have.
var ErrDeviceNotFound = errors.New("device not found")

// Device is an app signed in with a device token.
type Device struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	// Current is true for the device making the request.
	Current bool `json:"current"`
}

func cleanDeviceName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		name = "Unnamed device"
	}
	if r := []rune(name); len(r) > maxDeviceName {
		name = string(r[:maxDeviceName])
	}
	return name
}

// LoginDevice checks the credentials and creates a device session. The
// returned token is sent as "Authorization: Bearer <token>".
func (s *Service) LoginDevice(ctx context.Context, username, password, deviceName string) (string, User, Device, error) {
	user, err := s.checkCredentials(ctx, username, password)
	if err != nil {
		return "", User{}, Device{}, err
	}
	token, err := GenerateToken()
	if err != nil {
		return "", User{}, Device{}, err
	}
	d := Device{Name: cleanDeviceName(deviceName), Current: true}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at, kind, device_name, last_used_at)
		VALUES ($1::uuid, $2, NOW() + make_interval(secs => $3), 'device', $4, NOW())
		RETURNING id::text, created_at, last_used_at, expires_at
	`, user.ID, HashToken(token), deviceTTL.Seconds(), d.Name).Scan(&d.ID, &d.CreatedAt, &d.LastUsedAt, &d.ExpiresAt)
	if err != nil {
		return "", User{}, Device{}, err
	}
	return token, user, d, nil
}

// ListDevices returns the user's signed-in devices, most recently used
// first. currentToken (may be empty) marks the requesting device.
func (s *Service) ListDevices(ctx context.Context, userID, currentToken string) ([]Device, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, COALESCE(device_name, ''), created_at, last_used_at, expires_at, token_hash = $2
		FROM sessions
		WHERE user_id = $1::uuid AND kind = 'device' AND expires_at > NOW()
		ORDER BY COALESCE(last_used_at, created_at) DESC
	`, userID, HashToken(currentToken))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Device, error) {
		var d Device
		err := row.Scan(&d.ID, &d.Name, &d.CreatedAt, &d.LastUsedAt, &d.ExpiresAt, &d.Current)
		return d, err
	})
}

// RevokeDevice signs a device out.
func (s *Service) RevokeDevice(ctx context.Context, userID, deviceID string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM sessions WHERE id = $1::uuid AND user_id = $2::uuid AND kind = 'device'
	`, deviceID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDeviceNotFound
	}
	return nil
}
