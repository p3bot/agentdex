package catalog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultModulePath is the published catalog module loaded unless overridden.
const DefaultModulePath = "github.com/start-cli/agentdex/catalog@v1"

// DefaultTTL is the version-resolution cache lifetime.
const DefaultTTL = 24 * time.Hour

// Loader fetches, validates, and decodes the agent catalog. Registry, clock, and
// cache directory are injected so load and cache logic are testable from inputs.
type Loader struct {
	registry   Registry
	modulePath string
	cache      *resolutionCache
	ttl        time.Duration
	now        func() time.Time
}

// Result is the outcome of a successful load.
type Result struct {
	Catalog *Catalog
	// Version is the canonical module version the catalog was loaded from.
	Version string
	// Stale is true when re-resolution failed after the TTL expired and the last
	// resolved version was reused. The catalog is still usable; a caller may warn.
	Stale bool
}

// Option configures a Loader.
type Option func(*Loader)

// WithModulePath overrides the source catalog module path. The resolution cache
// is keyed by module path, so switching the override needs no special invalidation.
func WithModulePath(modulePath string) Option {
	return func(l *Loader) { l.modulePath = modulePath }
}

// WithCacheDir overrides the version-resolution cache directory.
func WithCacheDir(dir string) Option {
	return func(l *Loader) { l.cache = newResolutionCache(dir) }
}

// WithTTL overrides the version-resolution cache TTL.
func WithTTL(ttl time.Duration) Option {
	return func(l *Loader) { l.ttl = ttl }
}

// WithClock overrides the clock; used to make TTL behaviour deterministic in tests.
func WithClock(now func() time.Time) Option {
	return func(l *Loader) { l.now = now }
}

// New constructs a Loader over the given registry with built-in defaults: the
// default module path, the default cache directory under $XDG_CACHE_HOME, the
// default TTL, and the wall clock. Options override any of these.
func New(registry Registry, opts ...Option) *Loader {
	l := &Loader{
		registry:   registry,
		modulePath: DefaultModulePath,
		cache:      newResolutionCache(defaultCacheDir()),
		ttl:        DefaultTTL,
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Load resolves the catalog module version (honouring the cache and TTL),
// fetches the module, validates it against its bundled schema, and decodes it.
//
// Version resolution requires the network; fetching a canonical module@version
// is served from CUE's content cache offline. After a first successful run the
// catalog loads offline within the TTL; a failed re-resolution after the TTL
// keeps the last resolved version (reported as stale); a first run with no
// network and no cached resolution fails with ErrUnavailable.
func (l *Loader) Load(ctx context.Context) (*Result, error) {
	version, stale, err := l.resolveVersion(ctx)
	if err != nil {
		return nil, err
	}

	// Malformed coordinate is a config fault, not the offline condition.
	canonical, err := canonicalModulePath(l.modulePath, version)
	if err != nil {
		return nil, err
	}

	sourceDir, err := l.registry.Fetch(ctx, canonical)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	cat, err := loadCatalogModule(sourceDir)
	if err != nil {
		return nil, err
	}

	return &Result{Catalog: cat, Version: version, Stale: stale}, nil
}

// Within-TTL cache hits skip the network. Otherwise re-resolve: success refreshes
// the cache; failure keeps the last resolved version (stale) when one exists, or
// fails with ErrUnavailable on a first run with no cache. Cache I/O is best-effort:
// read errors are cache misses, write errors after a successful resolve are discarded.
func (l *Loader) resolveVersion(ctx context.Context) (version string, stale bool, err error) {
	// ok=false on read error: degrade to cache miss rather than abort.
	cached, ok, _ := l.cache.read(l.modulePath)
	if ok && cached.fresh(l.now(), l.ttl) {
		return cached.Version, false, nil
	}

	resolved, resolveErr := l.registry.ResolveLatestVersion(ctx, l.modulePath)
	if resolveErr != nil {
		if ok {
			return cached.Version, true, nil
		}
		return "", false, fmt.Errorf("%w: %w", ErrUnavailable, resolveErr)
	}

	// Best-effort: a failed write costs one re-resolution next run.
	_ = l.cache.write(resolution{
		ModulePath: l.modulePath,
		Version:    resolved,
		ResolvedAt: l.now(),
	})
	return resolved, false, nil
}

// defaultCacheDir resolves $XDG_CACHE_HOME/agentdex, falling back to
// ~/.cache/agentdex, then to a relative path if neither is available.
func defaultCacheDir() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "agentdex")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".cache", "agentdex")
	}
	return filepath.Join(".cache", "agentdex")
}
