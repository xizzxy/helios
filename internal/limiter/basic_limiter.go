package limiter

import (
    "context"
    "sync"
    "time"
)

// basicLimiter: simple token-bucket per tenant, in-memory.
type basicLimiter struct {
	cfg   Config
	mu    sync.Mutex
	state map[string]*bucket
}

type bucket struct {
	tokens     float64
	lastRefill time.Time
}

func NewBasicLimiter(cfg Config) Limiter {
	return &basicLimiter{
		cfg:   cfg,
		state: make(map[string]*bucket),
	}
}

func (b *basicLimiter) Allow(ctx context.Context, key string, cost int64) (*Result, error) {
	tenant := key
	if tenant == "" {
		tenant = "default"
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	w := b.cfg.Window
	if w <= 0 {
		w = time.Minute
	}
	limit := b.cfg.Limit
	if limit <= 0 {
		limit = 100
	}
	burst := b.cfg.Burst
	if burst <= 0 {
		burst = limit
	}

	// refill rate: limit per window
	refillPerSec := float64(limit) / w.Seconds()

	bkt, ok := b.state[tenant]
	if !ok {
		bkt = &bucket{tokens: float64(burst), lastRefill: now}
		b.state[tenant] = bkt
	}

	// Refill
	elapsed := now.Sub(bkt.lastRefill).Seconds()
	bkt.tokens = minFloat(float64(burst), bkt.tokens+elapsed*refillPerSec)
	bkt.lastRefill = now

	need := float64(cost)
	allowed := bkt.tokens >= need
	if allowed {
		bkt.tokens -= need
	}
	remaining := int64(bkt.tokens)
	reset := now.Add(time.Duration((float64(limit)-bkt.tokens)/refillPerSec) * time.Second)

	result := &Result{
		Allowed:   allowed,
		Remaining: remaining,
		Limit:     limit,
		ResetTime: reset,
	}

	if !allowed {
		result.RetryAfterSeconds = int64(reset.Sub(now).Seconds())
	}

	return result, nil
}

func (b *basicLimiter) GetQuota(ctx context.Context, key string) (*Result, error) {
	tenant := key
	if tenant == "" {
		tenant = "default"
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	w := b.cfg.Window
	if w <= 0 {
		w = time.Minute
	}
	limit := b.cfg.Limit
	if limit <= 0 {
		limit = 100
	}
	burst := b.cfg.Burst
	if burst <= 0 {
		burst = limit
	}

	// refill rate: limit per window
	refillPerSec := float64(limit) / w.Seconds()

	bkt, ok := b.state[tenant]
	if !ok {
		bkt = &bucket{tokens: float64(burst), lastRefill: now}
		b.state[tenant] = bkt
	}

	// Refill
	elapsed := now.Sub(bkt.lastRefill).Seconds()
	bkt.tokens = minFloat(float64(burst), bkt.tokens+elapsed*refillPerSec)
	bkt.lastRefill = now

	remaining := int64(bkt.tokens)
	reset := now.Add(time.Duration((float64(limit)-bkt.tokens)/refillPerSec) * time.Second)

	return &Result{
		Allowed:   true,
		Remaining: remaining,
		Limit:     limit,
		ResetTime: reset,
	}, nil
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}



