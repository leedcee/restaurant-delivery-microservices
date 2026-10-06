// Package identity implements customer registration, authentication and token rotation.
package identity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"restaurant-delivery-system/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTTL  = 15 * time.Minute
	refreshTTL = 30 * 24 * time.Hour
)

type Service struct {
	db     *pgxpool.Pool
	secret []byte
	now    func() time.Time
}

type Registration struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type Login struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenClaims struct {
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func New(db *pgxpool.Pool, secret string) *Service {
	return &Service{db: db, secret: []byte(secret), now: time.Now}
}

func (s *Service) Register(ctx context.Context, input Registration) (domain.AuthSession, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	if !strings.Contains(input.Email, "@") || len(input.Password) < 8 || input.Name == "" {
		return domain.AuthSession{}, fmt.Errorf("%w: name, valid email and password of at least 8 characters are required", domain.ErrValidation)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.AuthSession{}, fmt.Errorf("hash password: %w", err)
	}
	var user domain.User
	err = s.db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ($1, $2, $3)
		RETURNING id, email, name, created_at`, input.Email, string(hash), input.Name).
		Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.AuthSession{}, fmt.Errorf("%w: email is already registered", domain.ErrConflict)
	}
	if err != nil {
		return domain.AuthSession{}, fmt.Errorf("create user: %w", err)
	}
	return s.issueSession(ctx, user)
}

func (s *Service) Login(ctx context.Context, input Login) (domain.AuthSession, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	var user domain.User
	var passwordHash string
	err := s.db.QueryRow(ctx, `SELECT id, email, name, created_at, password_hash FROM users WHERE email = $1`, email).
		Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt, &passwordHash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil) {
		return domain.AuthSession{}, domain.ErrUnauthorized
	}
	if err != nil {
		return domain.AuthSession{}, fmt.Errorf("load user: %w", err)
	}
	return s.issueSession(ctx, user)
}

func (s *Service) Refresh(ctx context.Context, token string) (domain.AuthSession, error) {
	hash := tokenHash(token)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.AuthSession{}, fmt.Errorf("begin refresh: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var user domain.User
	var tokenID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT t.id, u.id, u.email, u.name, u.created_at
		FROM refresh_tokens t JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = $1 AND t.revoked_at IS NULL AND t.expires_at > now()
		FOR UPDATE OF t`, hash).Scan(&tokenID, &user.ID, &user.Email, &user.Name, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AuthSession{}, domain.ErrUnauthorized
	}
	if err != nil {
		return domain.AuthSession{}, fmt.Errorf("load refresh token: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`, tokenID); err != nil {
		return domain.AuthSession{}, fmt.Errorf("revoke refresh token: %w", err)
	}
	refreshToken, err := randomToken()
	if err != nil {
		return domain.AuthSession{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`, user.ID, tokenHash(refreshToken), s.now().Add(refreshTTL)); err != nil {
		return domain.AuthSession{}, fmt.Errorf("rotate refresh token: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.AuthSession{}, fmt.Errorf("commit refresh: %w", err)
	}
	accessToken, err := s.signAccess(user.ID)
	if err != nil {
		return domain.AuthSession{}, err
	}
	return domain.AuthSession{User: user, AccessToken: accessToken, RefreshToken: refreshToken, ExpiresIn: int64(accessTTL.Seconds())}, nil
}

func (s *Service) AuthenticateAccess(token string) (uuid.UUID, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return uuid.Nil, domain.ErrUnauthorized
	}
	signed := parts[0] + "." + parts[1]
	expected := hmac.New(sha256.New, s.secret)
	_, _ = expected.Write([]byte(signed))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, expected.Sum(nil)) {
		return uuid.Nil, domain.ErrUnauthorized
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return uuid.Nil, domain.ErrUnauthorized
	}
	var claims tokenClaims
	if json.Unmarshal(payload, &claims) != nil || s.now().Unix() >= claims.ExpiresAt {
		return uuid.Nil, domain.ErrUnauthorized
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, domain.ErrUnauthorized
	}
	return userID, nil
}

func (s *Service) GetUser(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	var user domain.User
	err := s.db.QueryRow(ctx, `SELECT id, email, name, created_at FROM users WHERE id = $1`, userID).
		Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (s *Service) issueSession(ctx context.Context, user domain.User) (domain.AuthSession, error) {
	accessToken, err := s.signAccess(user.ID)
	if err != nil {
		return domain.AuthSession{}, err
	}
	refreshToken, err := randomToken()
	if err != nil {
		return domain.AuthSession{}, err
	}
	_, err = s.db.Exec(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`, user.ID, tokenHash(refreshToken), s.now().Add(refreshTTL))
	if err != nil {
		return domain.AuthSession{}, fmt.Errorf("store refresh token: %w", err)
	}
	return domain.AuthSession{User: user, AccessToken: accessToken, RefreshToken: refreshToken, ExpiresIn: int64(accessTTL.Seconds())}, nil
}

func (s *Service) signAccess(userID uuid.UUID) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	now := s.now()
	payload, err := json.Marshal(tokenClaims{Subject: userID.String(), IssuedAt: now.Unix(), ExpiresAt: now.Add(accessTTL).Unix()})
	if err != nil {
		return "", fmt.Errorf("marshal access token: %w", err)
	}
	signed := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(signed))
	return signed + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
