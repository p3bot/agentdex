package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// resolutionCache is version-resolution caching over CUE's content cache (not a
// catalog snapshot), one file per module path so resolutions never collide.
type resolutionCache struct {
	dir string
}

type resolution struct {
	ModulePath string    `json:"module_path"`
	Version    string    `json:"version"`
	ResolvedAt time.Time `json:"resolved_at"`
}

func newResolutionCache(dir string) *resolutionCache {
	return &resolutionCache{dir: dir}
}

func (r resolution) fresh(now time.Time, ttl time.Duration) bool {
	return now.Sub(r.ResolvedAt) < ttl
}

// path hashes the module path so registry coordinates are filesystem-safe and
// each path maps to its own file.
func (c *resolutionCache) path(modulePath string) string {
	sum := sha256.Sum256([]byte(modulePath))
	return filepath.Join(c.dir, "catalog-resolution-"+hex.EncodeToString(sum[:8])+".json")
}

func (c *resolutionCache) read(modulePath string) (resolution, bool, error) {
	data, err := os.ReadFile(c.path(modulePath))
	if errors.Is(err, fs.ErrNotExist) {
		return resolution{}, false, nil
	}
	if err != nil {
		return resolution{}, false, fmt.Errorf("read resolution cache: %w", err)
	}
	var res resolution
	if err := json.Unmarshal(data, &res); err != nil {
		// Corrupt entry is absent; re-resolution overwrites it.
		return resolution{}, false, nil
	}
	// Reject a hash collision serving another module's resolution.
	if res.ModulePath != modulePath {
		return resolution{}, false, nil
	}
	return res, true, nil
}

// write uses temp+rename so concurrent readers never see a torn file. fsync is
// skipped: a lost write costs one re-resolution, not data.
func (c *resolutionCache) write(res resolution) error {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	data, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("marshal resolution: %w", err)
	}

	tmp, err := os.CreateTemp(c.dir, "catalog-resolution-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp resolution file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once rename succeeds

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp resolution file: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temp resolution permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp resolution file: %w", err)
	}
	if err := os.Rename(tmpName, c.path(res.ModulePath)); err != nil {
		return fmt.Errorf("rename resolution cache into place: %w", err)
	}
	return nil
}
