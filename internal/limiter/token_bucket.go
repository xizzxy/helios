package limiter

import (
	"context"
	"sync"
	"time"
)

// TokenBucketLimiter implements token bucket algorithm in memory
type TokenBucketLimiter struct {
	cfg   Config
	mu    sync.Mutex
	state map[string]*tokenBucket
}

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

func NewTokenBucketLimiter(cfg Config) Limiter {
	return &TokenBucketLimiter{
		cfg:   cfg,
		state: make(map[string]*tokenBucket),
	}
}

func (t *TokenBucketLimiter) Allow(ctx context.Context, key string, cost int64) (*Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	window := t.cfg.Window
	if window <= 0 {
		window = time.Minute
	}
	limit := t.cfg.Limit
	if limit <= 0 {
		limit = 100
	}
	burst := t.cfg.Burst
	if burst <= 0 {
		burst = limit
	}

	// Refill rate: limit per window
	refillPerSec := float64(limit) / window.Seconds()

	bucket, exists := t.state[key]
	if !exists {
		bucket = &tokenBucket{
			tokens:     float64(burst),
			lastRefill: now,
		}
		t.state[key] = bucket
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens = min(float64(burst), bucket.tokens+elapsed*refillPerSec)
	bucket.lastRefill = now

	// Check if we can consume the requested tokens
	costFloat := float64(cost)
	allowed := bucket.tokens >= costFloat
	if allowed {
		bucket.tokens -= costFloat
	}

	remaining := int64(bucket.tokens)
	resetTime := now.Add(time.Duration((costFloat-bucket.tokens)/refillPerSec) * time.Second)

	result := &Result{
		Allowed:   allowed,
		Remaining: remaining,
		Limit:     limit,
		ResetTime: resetTime,
	}

	if !allowed {
		result.RetryAfterSeconds = int64(resetTime.Sub(now).Seconds())
	}

	return result, nil
}

func (t *TokenBucketLimiter) GetQuota(ctx context.Context, key string) (*Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	window := t.cfg.Window
	if window <= 0 {
		window = time.Minute
	}
	limit := t.cfg.Limit
	if limit <= 0 {
		limit = 100
	}
	burst := t.cfg.Burst
	if burst <= 0 {
		burst = limit
	}

	// Refill rate: limit per window
	refillPerSec := float64(limit) / window.Seconds()

	bucket, exists := t.state[key]
	if !exists {
		bucket = &tokenBucket{
			tokens:     float64(burst),
			lastRefill: now,
		}
		t.state[key] = bucket
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens = min(float64(burst), bucket.tokens+elapsed*refillPerSec)
	bucket.lastRefill = now

	remaining := int64(bucket.tokens)
	resetTime := now.Add(time.Duration((float64(limit)-bucket.tokens)/refillPerSec) * time.Second)

	return &Result{
		Allowed:   true,
		Remaining: remaining,
		Limit:     limit,
		ResetTime: resetTime,
	}, nil
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
