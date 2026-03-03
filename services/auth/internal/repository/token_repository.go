package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/aman-churiwal/gridflow-auth/internal/model"
)

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

type ITokenRepository interface {
	StoreRefreshToken(ctx context.Context, userId, tokenHash string, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, tokenHash string) (*model.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenId string) error
}

type TokenRepository struct {
	db *sql.DB
}

func NewTokenRepository(db *sql.DB) *TokenRepository {
	return &TokenRepository{db: db}
}

func (r *TokenRepository) StoreRefreshToken(ctx context.Context, userId, tokenHash string, expiresAt time.Time) error {
	query := `INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`

	_, err := r.db.ExecContext(ctx, query, userId, tokenHash, expiresAt)
	if err != nil {
		return err
	}

	return nil
}

func (r *TokenRepository) GetRefreshToken(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	var refreshToken model.RefreshToken
	query := `SELECT id, user_id, token_hash, expires_at, revoked FROM refresh_tokens WHERE token_hash = $1`
	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(&refreshToken.ID, &refreshToken.UserID, &refreshToken.TokenHash, &refreshToken.ExpiresAt, &refreshToken.Revoked)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRefreshTokenNotFound
		}

		return nil, err
	}

	return &refreshToken, nil
}

func (r *TokenRepository) RevokeRefreshToken(ctx context.Context, tokenId string) error {
	query := `UPDATE refresh_tokens SET revoked = true WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, tokenId)
	if err != nil {
		return err
	}

	return nil
}
