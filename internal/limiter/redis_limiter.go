//go:build full
// +build full

package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/xizzxy/helios/internal/store"
)

type RedisTokenBucket struct {
	config *Config
	store  *store.RedisStore
}

func NewRedisTokenBucket(config *Config, store *store.RedisStore) *RedisTokenBucket {
	return &RedisTokenBucket{
		config: config,
		store:  store,
	}
}

func (rtb *RedisTokenBucket) Allow(ctx context.Context, key string, cost int64) (*Result, error) {
	allowed, remaining, resetTime, err := rtb.store.TokenBucketAllow(
		ctx,
		key,
		rtb.config.Limit,
		rtb.config.WindowSeconds,
		cost,
		rtb.config.BurstLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("redis token bucket allow: %w", err)
	}

	result := &Result{
		Allowed:   allowed,
		Remaining: remaining,
		Limit:     rtb.config.Limit,
		ResetTime: resetTime,
	}

	if !allowed {
		// Calculate retry after from reset time
		now := time.Now()
		if resetTime.After(now) {
			result.RetryAfterSeconds = int64(resetTime.Sub(now).Seconds())
		}
	}

	return result, nil
}

func (rtb *RedisTokenBucket) GetQuota(ctx context.Context, key string) (*Result, error) {
	remaining, resetTime, err := rtb.store.GetTokenBucketQuota(
		ctx,
		key,
		rtb.config.Limit,
		rtb.config.WindowSeconds,
		rtb.config.BurstLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("redis token bucket quota: %w", err)
	}

	return &Result{
		Allowed:   true,
		Remaining: remaining,
		Limit:     rtb.config.Limit,
		ResetTime: resetTime,
	}, nil
}

type RedisSlidingWindow struct {
	config *Config
	store  *store.RedisStore
}

func NewRedisSlidingWindow(config *Config, store *store.RedisStore) *RedisSlidingWindow {
	return &RedisSlidingWindow{
		config: config,
		store:  store,
	}
}

func (rsw *RedisSlidingWindow) Allow(ctx context.Context, key string, cost int64) (*Result, error) {
	allowed, remaining, resetTime, err := rsw.store.SlidingWindowAllow(
		ctx,
		key,
		rsw.config.Limit,
		rsw.config.WindowSeconds,
		cost,
	)
	if err != nil {
		return nil, fmt.Errorf("redis sliding window allow: %w", err)
	}

	result := &Result{
		Allowed:   allowed,
		Remaining: remaining,
		Limit:     rsw.config.Limit,
		ResetTime: resetTime,
	}

	if !allowed {
		// Calculate retry after from reset time
		now := time.Now()
		if resetTime.After(now) {
			result.RetryAfterSeconds = int64(resetTime.Sub(now).Seconds())
		}
	}

	return result, nil
}

func (rsw *RedisSlidingWindow) GetQuota(ctx context.Context, key string) (*Result, error) {
	remaining, resetTime, err := rsw.store.GetSlidingWindowQuota(
		ctx,
		key,
		rsw.config.Limit,
		rsw.config.WindowSeconds,
	)
	if err != nil {
		return nil, fmt.Errorf("redis sliding window quota: %w", err)
	}

	return &Result{
		Allowed:   true,
		Remaining: remaining,
		Limit:     rsw.config.Limit,
		ResetTime: resetTime,
	}, nil
}

// RedisManager manages Redis-backed rate limiters
type RedisManager struct {
	store    *store.RedisStore
	limiters map[string]Limiter
	configs  map[string]*Config
}

func NewRedisManager(store *store.RedisStore) *RedisManager {
	return &RedisManager{
		store:    store,
		limiters: make(map[string]Limiter),
		configs:  make(map[string]*Config),
	}
}

func (rm *RedisManager) GetLimiter(tenantID, resource string) (Limiter, error) {
	key := fmt.Sprintf("%s:%s", tenantID, resource)

	limiter, exists := rm.limiters[key]
	if exists {
		return limiter, nil
	}

	// Get config for this tenant/resource
	config, exists := rm.configs[key]
	if !exists {
		// Default config
		config = &Config{
			Algorithm:     TokenBucket,
			Limit:         100,
			WindowSeconds: 60,
			BurstLimit:    120,
		}
		rm.configs[key] = config
	}

	// Create Redis-backed limiter
	switch config.Algorithm {
	case TokenBucket:
		limiter = NewRedisTokenBucket(config, rm.store)
	case SlidingWindow:
		limiter = NewRedisSlidingWindow(config, rm.store)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", config.Algorithm)
	}

	rm.limiters[key] = limiter
	return limiter, nil
}

func (rm *RedisManager) UpdateConfig(tenantID, resource string, config *Config) error {
	key := fmt.Sprintf("%s:%s", tenantID, resource)
	rm.configs[key] = config

	// Remove existing limiter to force recreation with new config
	delete(rm.limiters, key)

	return nil
}

func (rm *RedisManager) Close() error {
	return rm.store.Close()
}
