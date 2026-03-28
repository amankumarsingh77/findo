package embedder

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/genai"
)

// KeyState represents the health state of a single API key.
type KeyState int

const (
	KeyHealthy  KeyState = iota
	KeyCooling
	KeyExhausted

	cooldownDuration    = 60 * time.Second
	escalationWindow    = 5 * time.Minute
	escalationThreshold = 3
)

// ManagedKey tracks the health state of a single API key.
type ManagedKey struct {
	mu         sync.Mutex
	apiKey     string
	Client     *genai.Client
	state      KeyState
	cooldownAt time.Time
	firstFail  time.Time
	failCount  int
}

// NewManagedKey creates a new ManagedKey in the healthy state.
func NewManagedKey(apiKey string) *ManagedKey {
	return &ManagedKey{
		apiKey: apiKey,
		state:  KeyHealthy,
	}
}

// State returns the current health state of the key, automatically
// transitioning from KeyCooling to KeyHealthy once the cooldown expires.
func (mk *ManagedKey) State() KeyState {
	mk.mu.Lock()
	defer mk.mu.Unlock()
	if mk.state == KeyCooling && time.Since(mk.cooldownAt) >= cooldownDuration {
		mk.state = KeyHealthy
		mk.failCount = 0
	}
	return mk.state
}

// MarkCooling transitions the key to the cooling state and records the cooldown start time.
func (mk *ManagedKey) MarkCooling() {
	mk.mu.Lock()
	defer mk.mu.Unlock()
	mk.state = KeyCooling
	mk.cooldownAt = time.Now()
}

// RecordFailure increments the failure counter and escalates the key state.
// After escalationThreshold failures within escalationWindow, the key becomes exhausted.
// Otherwise it enters a cooldown period.
func (mk *ManagedKey) RecordFailure() {
	mk.mu.Lock()
	defer mk.mu.Unlock()
	now := time.Now()
	if mk.failCount == 0 || now.Sub(mk.firstFail) > escalationWindow {
		mk.firstFail = now
		mk.failCount = 0
	}
	mk.failCount++
	if mk.failCount >= escalationThreshold {
		mk.state = KeyExhausted
	} else {
		mk.state = KeyCooling
		mk.cooldownAt = now
	}
}

// Reset clears all failure state and returns the key to healthy.
func (mk *ManagedKey) Reset() {
	mk.mu.Lock()
	defer mk.mu.Unlock()
	mk.state = KeyHealthy
	mk.failCount = 0
	mk.firstFail = time.Time{}
	mk.cooldownAt = time.Time{}
}

// SetClient attaches a genai.Client to this key.
func (mk *ManagedKey) SetClient(c *genai.Client) {
	mk.Client = c
}

// KeyPool manages a set of API keys with round-robin selection.
type KeyPool struct {
	mu     sync.Mutex
	keys   []*ManagedKey
	next   int
	logger *slog.Logger
}

// NewKeyPool creates a KeyPool from the provided keys. If logger is nil, slog.Default() is used.
func NewKeyPool(keys []*ManagedKey, logger *slog.Logger) *KeyPool {
	if logger == nil {
		logger = slog.Default()
	}
	return &KeyPool{
		keys:   keys,
		logger: logger.WithGroup("keypool"),
	}
}

// NextHealthy returns the next healthy key in round-robin order, along with its index.
// Returns an error if all keys are cooling or exhausted.
func (kp *KeyPool) NextHealthy() (*ManagedKey, int, error) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	n := len(kp.keys)
	for i := 0; i < n; i++ {
		idx := (kp.next + i) % n
		if kp.keys[idx].State() == KeyHealthy {
			kp.next = (idx + 1) % n
			return kp.keys[idx], idx, nil
		}
	}
	return nil, -1, fmt.Errorf("keypool: all %d keys are cooling or exhausted", n)
}

// ResetAll resets every key in the pool to healthy.
func (kp *KeyPool) ResetAll() {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	for _, k := range kp.keys {
		k.Reset()
	}
	kp.logger.Info("all keys reset to healthy", "count", len(kp.keys))
}

// Keys returns the underlying slice of managed keys.
func (kp *KeyPool) Keys() []*ManagedKey {
	return kp.keys
}

// Len returns the number of keys in the pool.
func (kp *KeyPool) Len() int {
	return len(kp.keys)
}
