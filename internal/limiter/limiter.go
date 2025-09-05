package limiter

import (
	"context"
	"time"
)

type Algorithm string

const (
	AlgoTokenBucket   Algorithm = "token_bucket"
	AlgoSlidingWindow Algorithm = "sliding_window"
)

type Config struct {
	Limit  int64
	Burst  int64
	Window time.Duration
	// Algorithm is ignored in the demo limiter but kept for compatibility
	Algorithm Algorithm
}

type Limiter interface {
	Allow(ctx context.Context, key string, cost int64) (*Result, error)
	GetQuota(ctx context.Context, key string) (*Result, error)
}

// Result represents the outcome of a rate limit check
type Result struct {
	Allowed           bool      `json:"allowed"`
	Remaining         int64     `json:"remaining"`
	Limit             int64     `json:"limit"`
	ResetTime         time.Time `json:"reset_time"`
	RetryAfterSeconds int64     `json:"retry_after_seconds,omitempty"`
}
