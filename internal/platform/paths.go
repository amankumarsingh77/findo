package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

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
	dir := filepath.Join(base, "universal-search")
	return dir, os.MkdirAll(dir, 0o755)
}

func DBPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "metadata.db"), nil
}

func IndexPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vectors.hnsw"), nil
}

func ThumbnailDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	thumbDir := filepath.Join(dir, "thumbnails")
	return thumbDir, os.MkdirAll(thumbDir, 0o755)
}
