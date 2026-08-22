package auth

import (
	"context"
	"errors"
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
)

// User is an authenticated OwnWave account.
type User struct {
	ID           string  `json:"id"`
	Username     string  `json:"username"`
	Email        *string `json:"email"`
	FullName     *string `json:"full_name"`
	IsAdmin      bool    `json:"is_admin"`
	PasswordHash string  `json:"-"`
}

// Service handles session Bearer auth (opaque tokens stored hashed in Postgres).
type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) Register(ctx context.Context, username, password string) (string, User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", User{}, err
	}

	userID, err := s.createUser(ctx, username, string(hash))
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

	return token, User{ID: userID, Username: username}, nil
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

func (s *Service) createUser(ctx context.Context, username, passwordHash string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash)
		VALUES ($1, $2)
		ON CONFLICT (username) DO NOTHING
		RETURNING id::text
	`, username, passwordHash).Scan(&id)
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
		VALUES ($1, $2, $3)
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
