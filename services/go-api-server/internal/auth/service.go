package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUsernameTaken      = errors.New("username taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidInvite      = errors.New("invalid invite")
	ErrRegistrationClosed = errors.New("registration closed")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrInvalidUsername    = errors.New("username required")
)

const minPasswordLen = 8

// User is an authenticated OwnWave account.
type User struct {
	ID           string  `json:"id"`
	Username     string  `json:"username"`
	Email        *string `json:"email"`
	FullName     *string `json:"full_name"`
	IsAdmin      bool    `json:"is_admin"`
	PasswordHash string  `json:"-"`
}

// Invite represents a pending user invitation.
type Invite struct {
	ID        string     `json:"id"`
	Username  *string    `json:"username,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Service handles session Bearer auth (opaque tokens stored hashed in Postgres).
type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func validateCredentials(username, password string) error {
	if strings.TrimSpace(username) == "" {
		return ErrInvalidUsername
	}
	if len(password) < minPasswordLen {
		return ErrWeakPassword
	}
	return nil
}

func (s *Service) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (s *Service) Register(ctx context.Context, username, password, inviteToken string) (string, User, error) {
	if err := validateCredentials(username, password); err != nil {
		return "", User{}, err
	}

	count, err := s.CountUsers(ctx)
	if err != nil {
		return "", User{}, err
	}

	switch {
	case count == 0:
		return s.registerUser(ctx, username, password, true, "")
	case inviteToken == "":
		return "", User{}, ErrRegistrationClosed
	default:
		return s.registerWithInvite(ctx, username, password, inviteToken)
	}
}

func (s *Service) RegisterFirstAdmin(ctx context.Context, username, password string) (string, User, error) {
	count, err := s.CountUsers(ctx)
	if err != nil {
		return "", User{}, err
	}
	if count > 0 {
		return "", User{}, ErrForbidden
	}
	return s.registerUser(ctx, username, password, true, "")
}

func (s *Service) registerWithInvite(ctx context.Context, username, password, inviteToken string) (string, User, error) {
	inviteID, presetUsername, err := s.consumeInvite(ctx, inviteToken)
	if err != nil {
		return "", User{}, err
	}
	if presetUsername != "" && presetUsername != username {
		return "", User{}, ErrInvalidInvite
	}
	_ = inviteID
	return s.registerUser(ctx, username, password, false, "")
}

func (s *Service) registerUser(ctx context.Context, username, password string, isAdmin bool, passwordHash string) (string, User, error) {
	var hash []byte
	var err error
	if passwordHash == "" {
		hash, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return "", User{}, err
		}
	} else {
		hash = []byte(passwordHash)
	}

	userID, err := s.createUser(ctx, username, string(hash), isAdmin)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", User{}, ErrUsernameTaken
		}
		return "", User{}, err
	}

	token, err := s.createSession(ctx, userID)
	if err != nil {
		return "", User{}, err
	}

	return token, User{ID: userID, Username: username, IsAdmin: isAdmin}, nil
}

func (s *Service) CreateUser(ctx context.Context, adminID, username, password string) (User, error) {
	if err := validateCredentials(username, password); err != nil {
		return User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	userID, err := s.createUser(ctx, username, string(hash), false)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUsernameTaken
		}
		return User{}, err
	}
	_ = adminID
	return User{ID: userID, Username: username, IsAdmin: false}, nil
}

func (s *Service) CreateInvite(ctx context.Context, adminID string, username *string, ttl time.Duration) (string, Invite, error) {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	token, err := GenerateToken()
	if err != nil {
		return "", Invite{}, err
	}
	expiresAt := time.Now().Add(ttl)
	var invite Invite
	err = s.pool.QueryRow(ctx, `
		INSERT INTO user_invites (token_hash, username, created_by, expires_at)
		VALUES ($1, $2, $3::uuid, $4)
		RETURNING id::text, username, expires_at, created_at
	`, HashToken(token), username, adminID, expiresAt).Scan(&invite.ID, &invite.Username, &invite.ExpiresAt, &invite.CreatedAt)
	if err != nil {
		return "", Invite{}, err
	}
	return token, invite, nil
}

func (s *Service) consumeInvite(ctx context.Context, rawToken string) (inviteID, presetUsername string, err error) {
	tokenHash := HashToken(rawToken)
	var username *string
	err = s.pool.QueryRow(ctx, `
		UPDATE user_invites
		SET used_at = NOW()
		WHERE token_hash = $1
		  AND used_at IS NULL
		  AND expires_at > NOW()
		RETURNING id::text, username
	`, tokenHash).Scan(&inviteID, &username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrInvalidInvite
		}
		return "", "", err
	}
	if username != nil {
		presetUsername = *username
	}
	return inviteID, presetUsername, nil
}

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, username, email, full_name, is_admin
		FROM users
		ORDER BY username
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.FullName, &u.IsAdmin); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Service) DeleteUser(ctx context.Context, actorID, targetID string) error {
	if actorID == targetID {
		return fmt.Errorf("cannot delete own account")
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1::uuid`, targetID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Service) Login(ctx context.Context, username, password string) (string, User, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return "", User{}, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", User{}, ErrInvalidCredentials
	}

	token, err := s.createSession(ctx, user.ID)
	if err != nil {
		return "", User{}, err
	}

	user.PasswordHash = ""
	return token, user, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return ErrUnauthorized
	}
	return s.deleteSession(ctx, HashToken(rawToken))
}

func (s *Service) UserFromRequest(r *http.Request) (User, bool) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return User{}, false
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return User{}, false
	}
	user, err := s.getUserByTokenHash(r.Context(), HashToken(token))
	if err != nil {
		return User{}, false
	}
	return user, true
}

func (s *Service) createSession(ctx context.Context, userID string) (string, error) {
	token, err := GenerateToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if err := s.insertSession(ctx, userID, HashToken(token), expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) createUser(ctx context.Context, username, passwordHash string, isAdmin bool) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, is_admin)
		VALUES ($1, $2, $3)
		ON CONFLICT (username) DO NOTHING
		RETURNING id::text
	`, username, passwordHash, isAdmin).Scan(&id)
	return id, err
}

func (s *Service) getUserByUsername(ctx context.Context, username string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, username, email, full_name, is_admin, password_hash
		FROM users
		WHERE username = $1
	`, username).Scan(&u.ID, &u.Username, &u.Email, &u.FullName, &u.IsAdmin, &u.PasswordHash)
	return u, err
}

func (s *Service) insertSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1::uuid, $2, $3)
	`, userID, tokenHash, expiresAt)
	return err
}

func (s *Service) getUserByTokenHash(ctx context.Context, tokenHash string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id::text, u.username, u.email, u.full_name, u.is_admin, u.password_hash
		FROM users u
		JOIN sessions s ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > NOW()
	`, tokenHash).Scan(&u.ID, &u.Username, &u.Email, &u.FullName, &u.IsAdmin, &u.PasswordHash)
	return u, err
}

func (s *Service) deleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM sessions
		WHERE token_hash = $1
	`, tokenHash)
	return err
}

func (s *Service) ChangePassword(ctx context.Context, user User, currentPassword, newPassword string) error {
	if len(newPassword) < minPasswordLen {
		return ErrWeakPassword
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE users SET password_hash = $2 WHERE id = $1::uuid
	`, user.ID, string(hash))
	return err
}
