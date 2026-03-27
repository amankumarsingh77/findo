package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// EventType represents the kind of file system event.
type EventType int

const (
	FileCreated EventType = iota
	FileModified
	FileDeleted
)

// FileEvent represents a debounced file system event.
type FileEvent struct {
	Path string
	Type EventType
}

// Watcher watches directories for file changes with debouncing.
type Watcher struct {
	fsw      *fsnotify.Watcher
	events   chan<- FileEvent
	debounce time.Duration
	pending  map[string]*time.Timer
	mu       sync.Mutex
	done     chan struct{}
}

// New creates a new Watcher that sends debounced events to the provided channel.
func New(events chan<- FileEvent, debounce time.Duration) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		fsw:      fsw,
		events:   events,
		debounce: debounce,
		pending:  make(map[string]*time.Timer),
		done:     make(chan struct{}),
	}
	go w.loop()
	return w, nil
}

func (w *Watcher) loop() {
	for {
		select {
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
		case <-w.done:
			return
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	var evType EventType
	switch {
	case event.Has(fsnotify.Create):
		evType = FileCreated
		// If directory created, watch it recursively
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			w.addRecursive(event.Name)
			return
		}
	case event.Has(fsnotify.Write):
		evType = FileModified
	case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename):
		evType = FileDeleted
	default:
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if timer, ok := w.pending[event.Name]; ok {
		timer.Stop()
	}

	fe := FileEvent{Path: event.Name, Type: evType}
	w.pending[event.Name] = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		delete(w.pending, event.Name)
		w.mu.Unlock()
		w.events <- fe
	})
}

// Add watches a directory and all its subdirectories recursively.
func (w *Watcher) Add(path string) error {
	return w.addRecursive(path)
}

func (w *Watcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			return w.fsw.Add(path)
		}
		return nil
	})
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	close(w.done)
	return w.fsw.Close()
}
