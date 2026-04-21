package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DataDir returns the platform-specific application data directory, creating it if needed.
func DataDir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "linux":
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, "Library", "Application Support")
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			return "", fmt.Errorf("APPDATA not set")
		}
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	dir := filepath.Join(base, "findo")
	migrateLegacyDataDir(base, dir)
	return dir, os.MkdirAll(dir, 0o755)
}

// migrateLegacyDataDir renames the pre-rename "universal-search" data directory
// to "findo" on first launch if the new directory does not already exist. A
// best-effort migration — any failure is silent and falls through to a fresh
// start at the new path.
func migrateLegacyDataDir(base, newDir string) {
	legacy := filepath.Join(base, "universal-search")
	if _, err := os.Stat(legacy); err != nil {
		return
	}
	if _, err := os.Stat(newDir); err == nil {
		return
	}
	_ = os.Rename(legacy, newDir)
}

// DBPath returns the absolute path to the SQLite metadata database file.
func DBPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "metadata.db"), nil
}

// IndexPath returns the absolute path to the HNSW vector index file.
func IndexPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vectors.hnsw"), nil
}

// ThumbnailDir returns the directory where generated thumbnails are cached.
func ThumbnailDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	thumbDir := filepath.Join(dir, "thumbnails")
	return thumbDir, os.MkdirAll(thumbDir, 0o755)
}
