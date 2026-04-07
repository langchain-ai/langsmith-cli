package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultDir returns ~/.langsmith/cache.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "langsmith-cache")
	}
	return filepath.Join(home, ".langsmith", "cache")
}

// PathForKey returns a cache file path using a SHA256 hash of the key.
func PathForKey(dir, prefix, key string) string {
	h := sha256.Sum256([]byte(key))
	name := fmt.Sprintf("%s-%x.json", prefix, h[:8])
	return filepath.Join(dir, name)
}

// ReadIfFresh reads a cached file if it exists and is within TTL.
func ReadIfFresh(path string, ttl time.Duration) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > ttl {
		return nil, fmt.Errorf("cache expired")
	}
	return os.ReadFile(path)
}

// Write writes data to a cache file, creating parent directories as needed.
func Write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
