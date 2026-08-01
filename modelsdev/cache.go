package modelsdev

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// cacheFileName is the plain-JSON stale cache of catalog.json, distinct from the
// CUE version-resolution cache the agent catalog uses.
const cacheFileName = "catalog-modelsdev.json"

// Upstream JSON verbatim; TTL is the file's mtime.
type cache struct {
	dir string
}

// Best-effort: unreadable is absent rather than fatal.
func (c cache) read() (data []byte, modTime time.Time, ok bool) {
	path := filepath.Join(c.dir, cacheFileName)
	info, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, false
	}
	data, err = os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, false
	}
	return data, info.ModTime(), true
}

// write uses temp+rename so concurrent readers never see a torn file. fsync is
// skipped: a lost write costs one re-fetch, not data.
func (c cache) write(data []byte) error {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(c.dir, cacheFileName+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp cache file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once rename succeeds

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp cache file: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temp cache permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp cache file: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Join(c.dir, cacheFileName)); err != nil {
		return fmt.Errorf("rename cache into place: %w", err)
	}
	return nil
}

// defaultCacheDir resolves $XDG_CACHE_HOME/agentdex, falling back to
// ~/.cache/agentdex, then to a relative path if neither is available. Mirrors
// the agent catalog loader; this leaf package cannot import it.
func defaultCacheDir() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "agentdex")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".cache", "agentdex")
	}
	return filepath.Join(".cache", "agentdex")
}
