package limiter

import (
	"context"
	"sync"
	"time"
)

// SlidingWindowLimiter implements sliding window algorithm in memory
type SlidingWindowLimiter struct {
	cfg     Config
	mu      sync.Mutex
	windows map[string]*slidingWindow
}

type slidingWindow struct {
	requests []time.Time
}

func NewSlidingWindowLimiter(cfg Config) Limiter {
	return &SlidingWindowLimiter{
		cfg:     cfg,
		windows: make(map[string]*slidingWindow),
	}
}

func (s *SlidingWindowLimiter) Allow(ctx context.Context, key string, cost int64) (*Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	window := s.cfg.Window
	if window <= 0 {
		window = time.Minute
	}
	limit := s.cfg.Limit
	if limit <= 0 {
		limit = 100
	}

	windowStart := now.Add(-window)

	// Get or create window for key
	w, exists := s.windows[key]
	if !exists {
		w = &slidingWindow{
			requests: make([]time.Time, 0),
		}
		s.windows[key] = w
	}

	// Remove expired requests
	validRequests := make([]time.Time, 0)
	for _, reqTime := range w.requests {
		if reqTime.After(windowStart) {
			validRequests = append(validRequests, reqTime)
		}
	}
	w.requests = validRequests

	// Check if adding cost would exceed limit
	currentCount := int64(len(w.requests))
	if currentCount+cost > limit {
		// Calculate reset time (when oldest request expires)
		resetTime := now.Add(window)
		if len(w.requests) > 0 {
			resetTime = w.requests[0].Add(window)
		}

		remaining := maxInt64(0, limit-currentCount)
		retryAfter := int64(resetTime.Sub(now).Seconds())

		return &Result{
			Allowed:           false,
			Remaining:         remaining,
			Limit:             limit,
			ResetTime:         resetTime,
			RetryAfterSeconds: retryAfter,
		}, nil
	}

	// Add requests for the cost
	for i := int64(0); i < cost; i++ {
		w.requests = append(w.requests, now)
	}

	remaining := limit - currentCount - cost
	resetTime := now.Add(window)

	return &Result{
		Allowed:   true,
		Remaining: remaining,
		Limit:     limit,
		ResetTime: resetTime,
	}, nil
}

func (s *SlidingWindowLimiter) GetQuota(ctx context.Context, key string) (*Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	window := s.cfg.Window
	if window <= 0 {
		window = time.Minute
	}
	limit := s.cfg.Limit
	if limit <= 0 {
		limit = 100
	}

	windowStart := now.Add(-window)

	// Get or create window for key
	w, exists := s.windows[key]
	if !exists {
		return &Result{
			Allowed:   true,
			Remaining: limit,
			Limit:     limit,
			ResetTime: now.Add(window),
		}, nil
	}

	// Remove expired requests
	validRequests := make([]time.Time, 0)
	for _, reqTime := range w.requests {
		if reqTime.After(windowStart) {
			validRequests = append(validRequests, reqTime)
		}
	}
	w.requests = validRequests

	currentCount := int64(len(w.requests))
	remaining := limit - currentCount
	resetTime := now.Add(window)

	return &Result{
		Allowed:   true,
		Remaining: remaining,
		Limit:     limit,
		ResetTime: resetTime,
	}, nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
