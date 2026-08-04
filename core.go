package agentdex

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/p3bot/agentdex/internal/catalog"
	"github.com/p3bot/agentdex/modelsdev"
)

// Open constructs an *Index over the configured catalog source and models.dev
// client. No network I/O and no context: both catalogs resolve lazily once under
// the first needing operation's context. Safe for concurrent use.
// WithCatalogDir and WithCatalogModule are mutually exclusive.
func Open(opts ...Option) (*Index, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	if o.catalogDir != "" && o.catalogModule != "" {
		return nil, errors.New("agentdex: WithCatalogDir and WithCatalogModule are mutually exclusive")
	}
	c := newCore(o)
	return &Index{
		Agents:    AgentService{core: c},
		Providers: ProviderService{core: c},
		Models:    ModelService{core: c},
		core:      c,
	}, nil
}

// Boundary inputs (immutable) and lazily resolved catalog/models.dev behind mutexes.
type core struct {
	envLookup  func(string) (string, bool)
	lookPath   func(string) (string, error)
	home       string
	workingDir string
	searchDirs []string
	binPaths   map[string]string
	logger     *slog.Logger

	catalogDir    string
	catalogModule string
	catalogTTL    time.Duration
	catalogTTLSet bool
	cacheDir      string

	modelsURL    string
	modelsTTL    time.Duration
	modelsTTLSet bool
	httpClient   *http.Client

	catMu   sync.Mutex
	cat     *catalog.Catalog
	catInfo CatalogInfo

	mdMu sync.Mutex
	md   *modelsdev.Client
}

// Capture boundary inputs once so per-operation logic is pure of them.
func newCore(o *options) *core {
	lookup := o.envLookup
	if lookup == nil {
		lookup = os.LookupEnv
	}
	lp := o.lookPath
	if lp == nil {
		lp = exec.LookPath
	}
	wd := o.workingDir
	if !o.workingDirSet {
		if got, err := os.Getwd(); err == nil {
			wd = got
		}
	}
	logger := o.logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &core{
		envLookup:     lookup,
		lookPath:      lp,
		home:          resolveHome(lookup),
		workingDir:    wd,
		searchDirs:    o.searchDirs,
		binPaths:      o.binPaths,
		logger:        logger,
		catalogDir:    o.catalogDir,
		catalogModule: o.catalogModule,
		catalogTTL:    o.catalogTTL,
		catalogTTLSet: o.catalogTTLSet,
		cacheDir:      o.cacheDir,
		modelsURL:     o.modelsURL,
		modelsTTL:     o.modelsTTL,
		modelsTTLSet:  o.modelsTTLSet,
		httpClient:    o.httpClient,
	}
}

// Injected HOME, then os.UserHomeDir; no platform user-dir helper.
func resolveHome(lookup func(string) (string, bool)) string {
	if h, ok := lookup("HOME"); ok && h != "" {
		return h
	}
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return ""
}

// Load once under the guard. Failures are not memoised so a transient outage recovers without reopening.
func (c *core) resolveCatalog(ctx context.Context) (*catalog.Catalog, CatalogInfo, error) {
	c.catMu.Lock()
	defer c.catMu.Unlock()
	if c.cat != nil {
		return c.cat, c.catInfo, nil
	}
	cat, info, err := c.loadCatalog(ctx, false)
	if err != nil {
		return nil, CatalogInfo{}, err
	}
	c.cat = cat
	c.catInfo = info
	return cat, info, nil
}

func (c *core) catalogModulePath() string {
	if c.catalogModule != "" {
		return c.catalogModule
	}
	return catalog.DefaultModulePath
}

// Dir source is never stale; registry may report a stale fallback.
func (c *core) loadCatalog(ctx context.Context, force bool) (*catalog.Catalog, CatalogInfo, error) {
	if c.catalogDir != "" {
		cat, err := catalog.LoadDir(c.catalogDir)
		if err != nil {
			return nil, CatalogInfo{}, mapCatalogErr(err)
		}
		info := CatalogInfo{Source: CatalogSourceDir, Dir: c.catalogDir}
		c.logger.LogAttrs(ctx, slog.LevelDebug, "catalog resolved",
			slog.String("source", "dir"), slog.String("dir", c.catalogDir))
		return cat, info, nil
	}

	reg, err := catalog.NewRegistry()
	if err != nil {
		return nil, CatalogInfo{}, err
	}
	module := c.catalogModulePath()
	var lopts []catalog.Option
	if c.catalogModule != "" {
		lopts = append(lopts, catalog.WithModulePath(c.catalogModule))
	}
	switch {
	case force:
		// Zero TTL forces re-resolution past any cached resolution.
		lopts = append(lopts, catalog.WithTTL(0))
	case c.catalogTTLSet:
		lopts = append(lopts, catalog.WithTTL(c.catalogTTL))
	}
	if c.cacheDir != "" {
		lopts = append(lopts, catalog.WithCacheDir(c.cacheDir))
	}
	res, err := catalog.New(reg, lopts...).Load(ctx)
	if err != nil {
		return nil, CatalogInfo{}, mapCatalogErr(err)
	}
	info := CatalogInfo{
		Source:  CatalogSourceRegistry,
		Module:  module,
		Version: res.Version,
		Stale:   res.Stale,
	}
	c.logger.LogAttrs(ctx, slog.LevelDebug, "catalog resolved",
		slog.String("source", "registry"),
		slog.String("module", module),
		slog.String("version", res.Version),
		slog.Bool("stale", res.Stale))
	return res.Catalog, info, nil
}

// mapCatalogErr keeps the loader's CUE diagnostic under the public sentinels.
func mapCatalogErr(err error) error {
	switch {
	case errors.Is(err, catalog.ErrUnavailable):
		return fmt.Errorf("%w: %w", ErrCatalogUnavailable, err)
	case errors.Is(err, catalog.ErrInvalidCatalog):
		return fmt.Errorf("%w: %w", ErrCatalogInvalid, err)
	default:
		return err
	}
}

// Schema drift propagates unchanged; other fetch faults become ErrModelsUnavailable.
// Agent ops degrade rather than calling this.
func mapModelsErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, modelsdev.ErrModelsSchema):
		return err
	default:
		return fmt.Errorf("%w: %w", ErrModelsUnavailable, err)
	}
}

// modelsClient is shared under the guard so concurrent work pays one fetch;
// two clients would each fetch independently.
func (c *core) modelsClient() *modelsdev.Client {
	c.mdMu.Lock()
	defer c.mdMu.Unlock()
	if c.md == nil {
		c.md = c.newModelsClient()
	}
	return c.md
}

// extra is applied last so force-refresh and similar can override base settings.
func (c *core) newModelsClient(extra ...modelsdev.ClientOption) *modelsdev.Client {
	var opts []modelsdev.ClientOption
	if c.modelsTTLSet {
		opts = append(opts, modelsdev.WithTTL(c.modelsTTL))
	}
	if c.modelsURL != "" {
		opts = append(opts, modelsdev.WithURL(c.modelsURL))
	}
	if c.cacheDir != "" {
		opts = append(opts, modelsdev.WithCacheDir(c.cacheDir))
	}
	if c.httpClient != nil {
		opts = append(opts, modelsdev.WithHTTPClient(c.httpClient))
	}
	opts = append(opts, extra...)
	return modelsdev.New(opts...)
}

// Dir source always current (not-refreshed, no error). Stale fallback is
// ErrCatalogUnavailable and leaves installed state untouched.
func (c *core) refreshCatalog(ctx context.Context) (bool, error) {
	if c.catalogDir != "" {
		c.logger.LogAttrs(ctx, slog.LevelDebug, "catalog refresh skipped",
			slog.String("reason", "directory source is always current"), slog.String("dir", c.catalogDir))
		return false, nil
	}
	cat, info, err := c.loadCatalog(ctx, true)
	if err != nil {
		return false, err
	}
	if info.Stale {
		return false, errf(ErrCatalogUnavailable,
			"agentdex catalog refresh failed: could not re-resolve the latest version, the cached version is unchanged")
	}
	c.catMu.Lock()
	c.cat, c.catInfo = cat, info
	c.catMu.Unlock()
	c.logger.LogAttrs(ctx, slog.LevelDebug, "catalog refreshed",
		slog.String("version", info.Version))
	return true, nil
}

// refreshModels swaps in a force-refresh client; a throwaway fetch would leave
// the Index on pre-refresh answers. Failure leaves the existing client in place.
func (c *core) refreshModels(ctx context.Context) error {
	fresh := c.newModelsClient(modelsdev.WithForceRefresh())
	if _, err := fresh.Catalog(ctx); err != nil {
		return mapModelsErr(err)
	}
	c.mdMu.Lock()
	c.md = fresh
	c.mdMu.Unlock()
	c.logger.LogAttrs(ctx, slog.LevelDebug, "models.dev refreshed")
	return nil
}
