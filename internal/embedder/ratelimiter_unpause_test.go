package embedder

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestWaitForUnpause_ImmediateWhenNotPaused verifies that WaitForUnpause returns
// nil immediately when no pause is active (REQ-021a).
func TestWaitForUnpause_ImmediateWhenNotPaused(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	start := time.Now()
	err := rl.WaitForUnpause(context.Background())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("expected immediate return, took %v", elapsed)
	}
}

// TestWaitForUnpause_WakeOnDeadlineReached verifies that WaitForUnpause wakes
// within 50 ms of the pause deadline expiring (REQ-021a).
func TestWaitForUnpause_WakeOnDeadlineReached(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	deadline := time.Now().Add(200 * time.Millisecond)
	rl.PauseUntil(deadline)

	start := time.Now()
	err := rl.WaitForUnpause(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("expected wake within 250ms, took %v", elapsed)
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("woke too early (%v), pause was 200ms", elapsed)
	}
}

// TestWaitForUnpause_WakeOnExplicitClear verifies that WaitForUnpause wakes
// within 50 ms of PauseUntil being called with a past timestamp (REQ-021a).
func TestWaitForUnpause_WakeOnExplicitClear(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	rl.PauseUntil(time.Now().Add(2 * time.Second))

	done := make(chan error, 1)
	go func() {
		done <- rl.WaitForUnpause(context.Background())
	}()

	time.Sleep(50 * time.Millisecond)
	clearTime := time.Now()
	rl.PauseUntil(time.Now().Add(-time.Second))

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		wakeDelay := time.Since(clearTime)
		if wakeDelay > 50*time.Millisecond {
			t.Fatalf("waiter took %v to wake after explicit clear, want <50ms", wakeDelay)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("waiter did not wake after explicit clear within 500ms")
	}
}

// TestWaitForUnpause_ExtensionDoesNotWakeEarly verifies that when PauseUntil
// extends an active pause, the waiter does NOT wake at the original deadline —
// it stays blocked until the new (longer) deadline (REQ-021a).
func TestWaitForUnpause_ExtensionDoesNotWakeEarly(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	rl.PauseUntil(time.Now().Add(150 * time.Millisecond))

	done := make(chan struct{}, 1)
	var wakeTime time.Time
	go func() {
		_ = rl.WaitForUnpause(context.Background())
		wakeTime = time.Now()
		done <- struct{}{}
	}()

	time.Sleep(50 * time.Millisecond)
	extendedDeadline := time.Now().Add(400 * time.Millisecond)
	rl.PauseUntil(extendedDeadline)

	time.Sleep(150 * time.Millisecond)
	select {
	case <-done:
		if wakeTime.Before(extendedDeadline.Add(-50 * time.Millisecond)) {
			t.Fatalf("waiter woke at %v, before extended deadline %v", wakeTime, extendedDeadline)
		}
	default:
	}

	select {
	case <-done:
		if wakeTime.Before(extendedDeadline.Add(-60 * time.Millisecond)) {
			t.Fatalf("final wake at %v, extended deadline was %v", wakeTime, extendedDeadline)
		}
	case <-time.After(600 * time.Millisecond):
		t.Fatal("waiter did not wake after extended deadline")
	}
}

// TestWaitForUnpause_CtxCancellation verifies that WaitForUnpause returns
// ctx.Err() promptly on context cancellation (REQ-021a).
func TestWaitForUnpause_CtxCancellation(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	rl.PauseUntil(time.Now().Add(10 * time.Second))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- rl.WaitForUnpause(ctx)
	}()

	time.Sleep(30 * time.Millisecond)
	cancelTime := time.Now()
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected non-nil error on ctx cancellation")
		}
		if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
		wakeDelay := time.Since(cancelTime)
		if wakeDelay > 50*time.Millisecond {
			t.Fatalf("cancellation took %v to propagate, want <50ms", wakeDelay)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("WaitForUnpause did not return on ctx cancellation within 500ms")
	}
}

// TestWaitForUnpause_ConcurrentWaiters verifies that 10 concurrent goroutines
// all wake within 50 ms of a pause expiring (REQ-021a).
func TestWaitForUnpause_ConcurrentWaiters(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	deadline := time.Now().Add(200 * time.Millisecond)
	rl.PauseUntil(deadline)

	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)
	wakeErrors := make([]error, n)
	wakeTimes := make([]time.Time, n)

	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			wakeErrors[i] = rl.WaitForUnpause(context.Background())
			wakeTimes[i] = time.Now()
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(400 * time.Millisecond):
		t.Fatal("concurrent waiters did not all wake within 400ms")
	}

	for i, err := range wakeErrors {
		if err != nil {
			t.Errorf("goroutine %d: unexpected error %v", i, err)
		}
	}
	for i, wt := range wakeTimes {
		delay := wt.Sub(deadline)
		if delay > 50*time.Millisecond {
			t.Errorf("goroutine %d: woke %v after deadline, want <50ms", i, delay)
		}
		if wt.Before(deadline.Add(-20 * time.Millisecond)) {
			t.Errorf("goroutine %d: woke too early (%v before deadline)", i, deadline.Sub(wt))
		}
	}
}
