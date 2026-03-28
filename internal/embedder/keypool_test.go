package embedder

import (
	"testing"
	"time"
)

func TestManagedKey_InitialState(t *testing.T) {
	mk := NewManagedKey("test-key-1")
	if mk.State() != KeyHealthy {
		t.Fatalf("expected HEALTHY, got %v", mk.State())
	}
}

func TestManagedKey_CooldownAndRecover(t *testing.T) {
	mk := NewManagedKey("test-key-1")
	mk.MarkCooling()
	if mk.State() != KeyCooling {
		t.Fatalf("expected COOLING, got %v", mk.State())
	}
	mk.cooldownAt = time.Now().Add(-61 * time.Second)
	if mk.State() != KeyHealthy {
		t.Fatalf("expected HEALTHY after cooldown, got %v", mk.State())
	}
}

func TestManagedKey_EscalateToExhausted(t *testing.T) {
	mk := NewManagedKey("test-key-1")
	mk.RecordFailure()
	mk.RecordFailure()
	mk.RecordFailure()
	if mk.State() != KeyExhausted {
		t.Fatalf("expected EXHAUSTED after 3 failures, got %v", mk.State())
	}
}

func TestManagedKey_ResetClearsState(t *testing.T) {
	mk := NewManagedKey("test-key-1")
	mk.RecordFailure()
	mk.RecordFailure()
	mk.RecordFailure()
	mk.Reset()
	if mk.State() != KeyHealthy {
		t.Fatalf("expected HEALTHY after reset, got %v", mk.State())
	}
}

func TestKeyPool_NextHealthy_RoundRobin(t *testing.T) {
	pool := NewKeyPool([]*ManagedKey{
		NewManagedKey("k1"),
		NewManagedKey("k2"),
		NewManagedKey("k3"),
	}, nil)

	_, idx1, err := pool.NextHealthy()
	if err != nil {
		t.Fatal(err)
	}
	_, idx2, err := pool.NextHealthy()
	if err != nil {
		t.Fatal(err)
	}
	_, idx3, err := pool.NextHealthy()
	if err != nil {
		t.Fatal(err)
	}

	if idx1 == idx2 || idx2 == idx3 {
		t.Fatalf("expected round-robin, got indices %d, %d, %d", idx1, idx2, idx3)
	}
}

func TestKeyPool_NextHealthy_SkipsCooling(t *testing.T) {
	keys := []*ManagedKey{
		NewManagedKey("k1"),
		NewManagedKey("k2"),
	}
	keys[0].MarkCooling()
	pool := NewKeyPool(keys, nil)

	_, idx, err := pool.NextHealthy()
	if err != nil {
		t.Fatal(err)
	}
	if idx != 1 {
		t.Fatalf("expected index 1, got %d", idx)
	}
}

func TestKeyPool_NextHealthy_AllExhausted(t *testing.T) {
	keys := []*ManagedKey{NewManagedKey("k1")}
	keys[0].RecordFailure()
	keys[0].RecordFailure()
	keys[0].RecordFailure()
	pool := NewKeyPool(keys, nil)

	_, _, err := pool.NextHealthy()
	if err == nil {
		t.Fatal("expected error when all keys exhausted")
	}
}

func TestKeyPool_ProbeRecoversExhaustedKey(t *testing.T) {
	mk := NewManagedKey("k1")
	mk.RecordFailure()
	mk.RecordFailure()
	mk.RecordFailure()
	if mk.State() != KeyExhausted {
		t.Fatal("expected EXHAUSTED")
	}
	mk.Reset()
	if mk.State() != KeyHealthy {
		t.Fatal("expected HEALTHY after probe reset")
	}
}

func TestKeyPool_HasExhaustedKeys(t *testing.T) {
	keys := []*ManagedKey{
		NewManagedKey("k1"),
		NewManagedKey("k2"),
	}
	pool := NewKeyPool(keys, nil)
	if pool.HasExhaustedKeys() {
		t.Fatal("no keys should be exhausted initially")
	}
	keys[0].RecordFailure()
	keys[0].RecordFailure()
	keys[0].RecordFailure()
	if !pool.HasExhaustedKeys() {
		t.Fatal("should detect exhausted key")
	}
}

func TestKeyPool_FirstExhausted(t *testing.T) {
	keys := []*ManagedKey{
		NewManagedKey("k1"),
		NewManagedKey("k2"),
	}
	pool := NewKeyPool(keys, nil)
	keys[1].RecordFailure()
	keys[1].RecordFailure()
	keys[1].RecordFailure()
	mk, idx := pool.FirstExhausted()
	if mk == nil || idx != 1 {
		t.Fatalf("expected exhausted key at index 1, got idx=%d", idx)
	}
}
