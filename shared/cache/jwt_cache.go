package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type CachedClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

type IJwtCache interface {
	Get(ctx context.Context, tokenHash string) (*CachedClaims, error)
	Set(ctx context.Context, tokenHash string, claims *CachedClaims, ttl time.Duration) error
}

type JwtCache struct {
	client *redis.Client
}

func NewJwtCache(client *redis.Client) *JwtCache {
	return &JwtCache{client: client}
}

func (c *JwtCache) Get(ctx context.Context, tokenHash string) (*CachedClaims, error) {
	redisKey := "jwt:" + tokenHash
	var claims CachedClaims

	val, err := c.client.Get(ctx, redisKey).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(val), &claims); err != nil {
		return nil, err
	}
	return &claims, nil
}

func (c *JwtCache) Set(ctx context.Context, tokenHash string, claims *CachedClaims, ttl time.Duration) error {
	redisKey := "jwt:" + tokenHash

	claimsJson, err := json.Marshal(claims)
	if err != nil {
		return err
	}

	if err := c.client.Set(ctx, redisKey, claimsJson, ttl).Err(); err != nil {
		return err
	}

	return nil
}
