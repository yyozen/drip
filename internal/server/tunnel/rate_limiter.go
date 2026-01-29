package tunnel

import (
	"sync"
	"time"

	"drip/internal/server/metrics"
	"go.uber.org/zap"
)

// rateLimitEntry tracks registration attempts per IP
type rateLimitEntry struct {
	count     int
	windowEnd time.Time
}

// RateLimiter manages rate limiting for tunnel registrations.
type RateLimiter struct {
	mu              sync.RWMutex
	rateLimits      map[string]*rateLimitEntry
	rateLimit       int
	rateLimitWindow time.Duration
	logger          *zap.Logger
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(rateLimit int, rateLimitWindow time.Duration, logger *zap.Logger) *RateLimiter {
	return &RateLimiter{
		rateLimits:      make(map[string]*rateLimitEntry),
		rateLimit:       rateLimit,
		rateLimitWindow: rateLimitWindow,
		logger:          logger,
	}
}

// CheckAndIncrement checks if the IP has exceeded rate limit and increments the counter.
func (rl *RateLimiter) CheckAndIncrement(ip string) bool {
	if ip == "" {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.rateLimits[ip]

	if !exists || now.After(entry.windowEnd) {
		// New window
		rl.rateLimits[ip] = &rateLimitEntry{
			count:     1,
			windowEnd: now.Add(rl.rateLimitWindow),
		}
		return true
	}

	if entry.count >= rl.rateLimit {
		rl.logger.Warn("Rate limit exceeded",
			zap.String("ip", ip),
			zap.Int("limit", rl.rateLimit),
		)
		metrics.RateLimitRejections.WithLabelValues("registration", ip).Inc()
		return false
	}

	entry.count++
	return true
}

// Cleanup removes expired rate limit entries.
func (rl *RateLimiter) Cleanup() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	removed := 0

	for ip, entry := range rl.rateLimits {
		if now.After(entry.windowEnd) {
			delete(rl.rateLimits, ip)
			removed++
		}
	}

	return removed
}
