package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher_DetectsNewFile(t *testing.T) {
	dir := t.TempDir()

	events := make(chan FileEvent, 10)
	w, err := New(events, 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = w.Add(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a file
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

	// Create file before watching
	testFile := filepath.Join(dir, "existing.txt")
	os.WriteFile(testFile, []byte("original"), 0o644)

	events := make(chan FileEvent, 10)
	w, err := New(events, 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = w.Add(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Modify the file
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

	// Create file before watching
	testFile := filepath.Join(dir, "todelete.txt")
	os.WriteFile(testFile, []byte("delete me"), 0o644)

	events := make(chan FileEvent, 10)
	w, err := New(events, 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = w.Add(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Delete the file
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
