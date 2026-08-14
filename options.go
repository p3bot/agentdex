package agentdex

import (
	"log/slog"
	"maps"
	"net/http"
	"time"
)

// Option configures Open. Nondeterministic inputs that shape reported values
// enter here (except process cache dir and clock). PATH search via WithLookPath.
type Option func(*options)

// Open rejects both catalogDir and catalogModule set.
type options struct {
	catalogModule string
	catalogDir    string
	catalogTTL    time.Duration
	catalogTTLSet bool
	cacheDir      string
	modelsURL     string
	modelsTTL     time.Duration
	modelsTTLSet  bool
	searchDirs    []string
	binPaths      map[string]string
	lookPath      func(string) (string, error)
	envLookup     func(string) (string, bool)
	workingDir    string
	workingDirSet bool
	httpClient    *http.Client
	logger        *slog.Logger
}

// WithCatalogModule overrides the registry catalog module path.
// Mutually exclusive with WithCatalogDir.
func WithCatalogModule(path string) Option {
	return func(o *options) { o.catalogModule = path }
}

// WithCatalogDir evaluates a local CUE module (never stale, no network).
// Mutually exclusive with WithCatalogModule; schema reject is ErrCatalogInvalid.
func WithCatalogDir(dir string) Option {
	return func(o *options) { o.catalogDir = dir }
}

// WithCatalogTTL sets the catalog version-resolution cache TTL. Inert under WithCatalogDir.
func WithCatalogTTL(d time.Duration) Option {
	return func(o *options) {
		o.catalogTTL = d
		o.catalogTTLSet = true
	}
}

// WithCacheDir sets the catalog resolution and models.dev cache directory.
// The clock stays on the process.
func WithCacheDir(dir string) Option {
	return func(o *options) { o.cacheDir = dir }
}

// WithModelsURL overrides the models.dev catalog source URL.
func WithModelsURL(url string) Option {
	return func(o *options) { o.modelsURL = url }
}

// WithModelsTTL sets the models.dev cache TTL.
func WithModelsTTL(d time.Duration) Option {
	return func(o *options) {
		o.modelsTTL = d
		o.modelsTTLSet = true
	}
}

// WithSearchDirs adds binary search locations consulted after PATH.
func WithSearchDirs(dirs ...string) Option {
	return func(o *options) { o.searchDirs = append(o.searchDirs, dirs...) }
}

// WithBinPaths overrides agents' binary paths by id (filesystem path, not PATH-
// resolved; relative roots at working directory).
func WithBinPaths(m map[string]string) Option {
	return func(o *options) {
		if o.binPaths == nil {
			o.binPaths = make(map[string]string, len(m))
		}
		maps.Copy(o.binPaths, m)
	}
}

// WithLookPath supplies PATH search for agent binaries (default exec.LookPath).
// Non-executable or failed hits fall through to WithSearchDirs. Inject a closed
// or fixture-scoped function so host PATH never leaks into detection.
func WithLookPath(fn func(string) (string, error)) Option {
	return func(o *options) { o.lookPath = fn }
}

// WithEnvLookup supplies env for provider-env presence and path expansion ($VAR, ~).
// Default os.LookupEnv. Only presence is taken from a variable, never its value.
func WithEnvLookup(fn func(string) (string, bool)) Option {
	return func(o *options) { o.envLookup = fn }
}

// WithWorkingDir sets the base for relative local config, skills, or binary paths.
// Default os.Getwd.
func WithWorkingDir(dir string) Option {
	return func(o *options) {
		o.workingDir = dir
		o.workingDirSet = true
	}
}

// WithHTTPClient overrides the HTTP client models.dev is fetched with.
func WithHTTPClient(hc *http.Client) Option {
	return func(o *options) { o.httpClient = hc }
}

// WithLogger threads a structured logger through decision points. Default is
// discard so the library is silent unless a caller opts in.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}
