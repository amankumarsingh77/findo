package embedder

import (
	"testing"
	"time"
)

func TestRotation_FullCycle(t *testing.T) {
	keys := []*ManagedKey{
		NewManagedKey("k1"),
		NewManagedKey("k2"),
		NewManagedKey("k3"),
	}
	pool := NewKeyPool(keys, nil)

	// Exhaust k1.
	keys[0].RecordFailure()
	keys[0].RecordFailure()
	keys[0].RecordFailure()

	// Cool k2.
	keys[1].MarkCooling()

	// Only k3 should be available.
	mk, idx, err := pool.NextHealthy()
	if err != nil {
		t.Fatal(err)
	}
	if idx != 2 {
		t.Fatalf("expected key index 2, got %d", idx)
	}
	_ = mk

	// Exhaust k3 too — should get error.
	keys[2].RecordFailure()
	keys[2].RecordFailure()
	keys[2].RecordFailure()

	_, _, err = pool.NextHealthy()
	if err == nil {
		t.Fatal("expected error, all keys should be unavailable")
	}

	// Simulate cooldown expiry for k2.
	keys[1].mu.Lock()
	keys[1].cooldownAt = time.Now().Add(-61 * time.Second)
	keys[1].mu.Unlock()

	// k2 should recover.
	mk, idx, err = pool.NextHealthy()
	if err != nil {
		t.Fatal(err)
	}
	if idx != 1 {
		t.Fatalf("expected key index 1 after cooldown, got %d", idx)
	}
	_ = mk

	// Reset all — everyone healthy.
	pool.ResetAll()
	healthy := 0
	for _, k := range pool.Keys() {
		if k.State() == KeyHealthy {
			healthy++
		}
	}
	if healthy != 3 {
		t.Fatalf("expected 3 healthy after reset, got %d", healthy)
	}
}

func TestRotation_CoolingAutoRecovers(t *testing.T) {
	mk := NewManagedKey("k1")
	mk.RecordFailure()
	if mk.State() != KeyCooling {
		t.Fatalf("expected COOLING, got %v", mk.State())
	}

	mk.mu.Lock()
	mk.cooldownAt = time.Now().Add(-61 * time.Second)
	mk.mu.Unlock()

	if mk.State() != KeyHealthy {
		t.Fatalf("expected HEALTHY after cooldown, got %v", mk.State())
	}
}
