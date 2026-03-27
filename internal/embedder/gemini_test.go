package embedder

import (
	"testing"
	"time"
)

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Second)
	for i := 0; i < 5; i++ {
		if !rl.Allow() {
			t.Fatalf("request %d should have been allowed", i)
		}
	}
	if rl.Allow() {
		t.Fatal("6th request should have been denied")
	}
}

func TestRateLimiter_AllowsAfterWindowExpires(t *testing.T) {
	rl := NewRateLimiter(2, 100*time.Millisecond)
	rl.Allow()
	rl.Allow()
	if rl.Allow() {
		t.Fatal("3rd request should be denied")
	}
	time.Sleep(150 * time.Millisecond)
	if !rl.Allow() {
		t.Fatal("request after window should be allowed")
	}
}
