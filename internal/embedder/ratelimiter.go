package embedder

import (
	"sync"
	"time"
)

// RateLimiter implements a sliding window rate limiter.
type RateLimiter struct {
	mu      sync.Mutex
	tokens  []time.Time
	maxReqs int
	window  time.Duration
}

// NewRateLimiter creates a rate limiter that allows maxReqs requests per window.
func NewRateLimiter(maxReqs int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		tokens:  make([]time.Time, 0, maxReqs),
		maxReqs: maxReqs,
		window:  window,
	}
}

// Allow returns true if the request is within the rate limit, false otherwise.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Remove expired tokens.
	valid := rl.tokens[:0]
	for _, t := range rl.tokens {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.tokens = valid

	if len(rl.tokens) >= rl.maxReqs {
		return false
	}
	rl.tokens = append(rl.tokens, now)
	return true
}

// Wait blocks until a request is allowed by the rate limiter.
func (rl *RateLimiter) Wait() {
	for !rl.Allow() {
		time.Sleep(100 * time.Millisecond)
	}
}
