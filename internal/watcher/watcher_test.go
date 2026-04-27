package watcher

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestWatcher_DetectsNewFile(t *testing.T) {
	dir := t.TempDir()

	events := make(chan FileEvent, 10)
	w, err := New(events, 200*time.Millisecond, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = w.Add(dir)
	if err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0o644)

	select {
	case ev := <-events:
		if ev.Path != testFile {
			t.Fatalf("expected path %s, got %s", testFile, ev.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for file event")
	}
}

func TestWatcher_DetectsModifiedFile(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "existing.txt")
	os.WriteFile(testFile, []byte("original"), 0o644)

	events := make(chan FileEvent, 10)
	w, err := New(events, 200*time.Millisecond, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = w.Add(dir)
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile(testFile, []byte("modified"), 0o644)

	select {
	case ev := <-events:
		if ev.Path != testFile {
			t.Fatalf("expected path %s, got %s", testFile, ev.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for file event")
	}
}

func TestWatcher_DetectsDeletedFile(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "todelete.txt")
	os.WriteFile(testFile, []byte("delete me"), 0o644)

	events := make(chan FileEvent, 10)
	w, err := New(events, 200*time.Millisecond, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = w.Add(dir)
	if err != nil {
		t.Fatal(err)
	}

	os.Remove(testFile)

	select {
	case ev := <-events:
		if ev.Path != testFile {
			t.Fatalf("expected path %s, got %s", testFile, ev.Path)
		}
		if ev.Type != FileDeleted {
			t.Fatalf("expected FileDeleted event, got %d", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for file event")
	}
}

func TestWatcher_Remove(t *testing.T) {
	dir := t.TempDir()
	events := make(chan FileEvent, 10)
	w, err := New(events, 50*time.Millisecond, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Add(dir); err != nil {
		t.Fatal(err)
	}

	if err := w.Remove(dir); err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)
	time.Sleep(200 * time.Millisecond)

	select {
	case ev := <-events:
		t.Fatalf("should not receive event after remove, got %v", ev)
	default:
	}
}
