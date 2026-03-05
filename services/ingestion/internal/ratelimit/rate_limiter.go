package ratelimit

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var rateLimitLuaScript = `
	local key = KEYS[1]
	local now = tonumber(ARGV[1])
	local window = tonumber(ARGV[2])
	local limit = tonumber(ARGV[3])
	local ttl = tonumber(ARGV[4])
	local member = ARGV[5]
	
	redis.call("ZREMRANGEBYSCORE", key, "-inf", now - window)
	
	local count = redis.call("ZCARD", key)
	
	if count >= limit then
		return 0
	end
	
	redis.call("ZADD", key, now, member)
	redis.call("EXPIRE", key, ttl)
	
	return 1
`

type IRateLimiter interface {
	Allow(ctx context.Context, vehicleID string) (bool, error)
}

type redisEvalClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
}

type RateLimiter struct {
	client              redisEvalClient
	rateLimitMaxPings   int64
	rateLimitWindowSecs int64
}

func NewRateLimiter(client redisEvalClient, rateLimitMaxPings, rateLimitWindowSecs int64) *RateLimiter {
	return &RateLimiter{
		client:              client,
		rateLimitMaxPings:   rateLimitMaxPings,
		rateLimitWindowSecs: rateLimitWindowSecs,
	}
}

func (r *RateLimiter) Allow(ctx context.Context, vehicleID string) (bool, error) {
	redisKey := "ratelimit:vehicle:" + vehicleID
	windowEnd := time.Now().UnixMilli()

	result, err := r.client.Eval(ctx, rateLimitLuaScript, []string{redisKey}, windowEnd, r.rateLimitWindowSecs*1000, r.rateLimitMaxPings, r.rateLimitWindowSecs, uuid.New().String()).Int()

	if err != nil {
		return false, err
	}

	return result == 1, nil
}
