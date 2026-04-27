package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"findo/internal/apperr"
	"findo/internal/config"
)

func TestApp_SetBaseContext_StoresContext(t *testing.T) {
	a := NewApp(nil)
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "marker")
	a.SetBaseContext(ctx)
	if a.baseCtx == nil {
		t.Fatal("SetBaseContext did not store ctx")
	}
	if a.baseCtx.Value(key{}) != "marker" {
		t.Fatalf("baseCtx lost its value")
	}
}

func TestApp_EmitBackendError_NoContextNoOp(t *testing.T) {
	a := &App{logger: slog.Default()}
	a.emitBackendError("ERR_TEST", "test", nil)
}

func TestApp_EmitBackendError_InvokesSink(t *testing.T) {
	a := &App{logger: slog.Default(), ctx: context.Background()}
	var mu sync.Mutex
	got := make([]map[string]any, 0)
	a.backendErrorSink = func(payload map[string]any) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, payload)
	}
	a.emitBackendError(apperr.ErrEmbedFailed.Code, "embed failed", map[string]any{"file": "/foo"})

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(got))
	}
	p := got[0]
	if p["code"] != apperr.ErrEmbedFailed.Code {
		t.Errorf("expected code %q, got %v", apperr.ErrEmbedFailed.Code, p["code"])
	}
	if p["message"] != "embed failed" {
		t.Errorf("expected message 'embed failed', got %v", p["message"])
	}
	if p["file"] != "/foo" {
		t.Errorf("expected file '/foo', got %v", p["file"])
	}
}

func TestApp_BackgroundError_EmitsBackendErrorEvent(t *testing.T) {
	a := &App{
		cfg:    config.DefaultConfig(),
		logger: slog.Default(),
		ctx:    context.Background(),
	}
	var mu sync.Mutex
	received := 0
	a.backendErrorSink = func(payload map[string]any) {
		mu.Lock()
		received++
		mu.Unlock()
	}
	a.reportBackgroundError("task-x", apperr.Wrap(apperr.ErrEmbedFailed.Code, "embed died", errors.New("boom")))

	mu.Lock()
	defer mu.Unlock()
	if received != 1 {
		t.Fatalf("expected 1 background-error event, got %d", received)
	}
}

func TestShutdown_CleanExitOnCancel(t *testing.T) {
	a := &App{
		cfg:    config.DefaultConfig(),
		logger: slog.Default(),
		ctx:    context.Background(),
	}
	a.cfg.App.ShutdownTimeoutMs = 1000
	a.SetBaseContext(context.Background())
	a.startErrgroup()

	a.group.Go(func() error {
		<-a.groupCtx.Done()
		return nil
	})

	timedOut := a.shutdownWithTimeout()
	if timedOut {
		t.Fatal("expected clean shutdown, got timeout")
	}
}

func TestEmitStatusLoop_ExitsOnCtxCancel(t *testing.T) {
	a := &App{cfg: config.DefaultConfig(), logger: slog.Default(), ctx: context.Background()}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		a.emitStatusLoop(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("emitStatusLoop did not exit on ctx cancel")
	}
}

func TestShutdown_TimeoutReturnsTrue(t *testing.T) {
	a := &App{
		cfg:    config.DefaultConfig(),
		logger: slog.Default(),
		ctx:    context.Background(),
	}
	a.cfg.App.ShutdownTimeoutMs = 100
	a.SetBaseContext(context.Background())
	a.startErrgroup()

	done := make(chan struct{})
	a.group.Go(func() error {
		<-done
		return nil
	})
	defer close(done)

	start := time.Now()
	timedOut := a.shutdownWithTimeout()
	elapsed := time.Since(start)
	if !timedOut {
		t.Fatal("expected timeout, got clean shutdown")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("shutdown exceeded reasonable bound: %v", elapsed)
	}
}
