//go:build full
// +build full

package store

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	redis *redis.Client
}

func NewClientFromEnv() (*Client, error) {
	addr := getEnv("HELIOS_REDIS_ADDRESS", "localhost:6379")
	password := getEnv("HELIOS_REDIS_PASSWORD", "")
	db := getEnvInt("HELIOS_REDIS_DATABASE", 0)

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     getEnvInt("HELIOS_REDIS_POOL_SIZE", 100),
		MinIdleConns: getEnvInt("HELIOS_REDIS_MIN_IDLE_CONNS", 10),
		MaxRetries:   getEnvInt("HELIOS_REDIS_MAX_RETRIES", 3),
		DialTimeout:  getEnvDuration("HELIOS_REDIS_DIAL_TIMEOUT", 5*time.Second),
		ReadTimeout:  getEnvDuration("HELIOS_REDIS_READ_TIMEOUT", 3*time.Second),
		WriteTimeout: getEnvDuration("HELIOS_REDIS_WRITE_TIMEOUT", 3*time.Second),
	})

	return &Client{redis: client}, nil
}

func (c *Client) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.redis.Ping(ctx).Err()
}

func (c *Client) Close() error {
	return c.redis.Close()
}

func (c *Client) Stats() map[string]any {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := c.redis.Info(ctx, "stats").Result()
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	stats := make(map[string]any)
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				stats[key] = value
			}
		}
	}

	return stats
}

// GetStats is an alias for Stats for compatibility
func (c *Client) GetStats(ctx context.Context) (map[string]any, error) {
	return c.Stats(), nil
}

// TokenBucketAllow implements atomic token bucket using Redis Lua script
func (c *Client) TokenBucketAllow(ctx context.Context, key string, limit, windowSec int64, cost int, burst int64) (bool, int64, time.Time, error) {
	// Lua script for atomic token bucket operations
	script := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local limit = tonumber(ARGV[2])
		local window = tonumber(ARGV[3])
		local cost = tonumber(ARGV[4])
		local burst = tonumber(ARGV[5])
		
		-- Get current bucket state
		local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
		local tokens = tonumber(bucket[1]) or burst
		local last_refill = tonumber(bucket[2]) or now
		
		-- Calculate tokens to add
		local elapsed = (now - last_refill) / 1000.0
		local tokens_to_add = elapsed * limit / window
		tokens = math.min(tokens + tokens_to_add, burst)
		
		-- Check if we can allow the request
		if tokens >= cost then
			tokens = tokens - cost
			
			-- Update bucket state
			redis.call('HMSET', key, 'tokens', tokens, 'last_refill', now)
			redis.call('EXPIRE', key, window * 2)
			
			return {1, tokens, now + (window * 1000)}
		else
			-- Update last refill time even if request is denied
			redis.call('HMSET', key, 'tokens', tokens, 'last_refill', now)
			redis.call('EXPIRE', key, window * 2)
			
			-- Calculate retry after
			local tokens_needed = cost - tokens
			local retry_after = math.ceil(tokens_needed * window / limit)
			
			return {0, tokens, now + (retry_after * 1000)}
		end
	`

	now := time.Now().UnixMilli()
	result, err := c.redis.Eval(ctx, script, []string{key}, now, limit, windowSec, cost, burst).Result()
	if err != nil {
		return false, 0, time.Time{}, fmt.Errorf("redis token bucket eval: %w", err)
	}

	res := result.([]interface{})
	allowed := res[0].(int64) == 1
	remaining := res[1].(int64)
	resetTimeMs := res[2].(int64)
	resetTime := time.UnixMilli(resetTimeMs)

	return allowed, remaining, resetTime, nil
}

// SlidingWindowAllow implements atomic sliding window using Redis Lua script
func (c *Client) SlidingWindowAllow(ctx context.Context, key string, limit, windowSec int64, cost int) (bool, int64, time.Time, error) {
	// Lua script for atomic sliding window operations
	script := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local limit = tonumber(ARGV[2])
		local window = tonumber(ARGV[3])
		local cost = tonumber(ARGV[4])
		
		-- Remove expired entries
		local window_start = now - (window * 1000)
		redis.call('ZREMRANGEBYSCORE', key, 0, window_start)
		
		-- Get current count
		local current_count = redis.call('ZCARD', key)
		
		-- Check if we can allow the request
		if current_count + cost <= limit then
			-- Add entries for the cost
			for i = 1, cost do
				redis.call('ZADD', key, now, now .. ':' .. i)
			end
			
			-- Set expiration
			redis.call('EXPIRE', key, window)
			
			local remaining = limit - current_count - cost
			return {1, remaining, now + (window * 1000)}
		else
			-- Calculate next available time
			local oldest_score = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
			local retry_time = now + (window * 1000)
			
			if #oldest_score > 0 then
				retry_time = oldest_score[2] + (window * 1000)
			end
			
			local remaining = math.max(0, limit - current_count)
			return {0, remaining, retry_time}
		end
	`

	now := time.Now().UnixMilli()
	result, err := c.redis.Eval(ctx, script, []string{key}, now, limit, windowSec, cost).Result()
	if err != nil {
		return false, 0, time.Time{}, fmt.Errorf("redis sliding window eval: %w", err)
	}

	res := result.([]interface{})
	allowed := res[0].(int64) == 1
	remaining := res[1].(int64)
	resetTimeMs := res[2].(int64)
	resetTime := time.UnixMilli(resetTimeMs)

	return allowed, remaining, resetTime, nil
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

