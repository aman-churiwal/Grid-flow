package repository

import (
	"context"
	"database/sql"
	"time"
)

type ITokenRepository interface {
	StoreRefreshToken(ctx context.Context, userId, tokenHash string, expiresAt time.Time) error
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
