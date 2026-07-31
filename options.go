package agentdex

import (
	"log/slog"
	"maps"
	"net/http"
	"time"
)

// Option configures Open. Every nondeterministic input that shapes a reported
// value enters here, save for the process defaults that R10 still names and
// defends: the default cache directory and the clock. PATH search is injectable
// via WithLookPath (defaulting to exec.LookPath).
type Option func(*options)

// options is the resolved settings Open reads. The two catalog sources are
// mutually exclusive: Open rejects both catalogDir and catalogModule set.
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

// WithCatalogModule overrides the catalog module path resolved from the registry.
// It is mutually exclusive with WithCatalogDir; Open rejects both.
func WithCatalogModule(path string) Option {
	return func(o *options) { o.catalogModule = path }
}

// WithCatalogDir loads the catalog by evaluating a local CUE module directory,
// bypassing the registry entirely. It is never stale and needs no network. It is
// mutually exclusive with WithCatalogModule; Open rejects both. An entry the
// schema rejects fails with ErrCatalogInvalid.
func WithCatalogDir(dir string) Option {
	return func(o *options) { o.catalogDir = dir }
}

// WithCatalogTTL sets the catalog version-resolution cache TTL. Inert under
// WithCatalogDir.
func WithCatalogTTL(d time.Duration) Option {
	return func(o *options) {
		o.catalogTTL = d
		o.catalogTTLSet = true
	}
}

// WithCacheDir sets the cache directory for the catalog resolution cache and
// models.dev. The models.dev half honours it; the clock stays on the process
// (R10).
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

// WithBinPaths overrides specific agents' binary paths by id. An override is a
// filesystem path, made absolute against the working directory when relative; it
// is not PATH-resolved and is the binary used for the version exec.
func WithBinPaths(m map[string]string) Option {
	return func(o *options) {
		if o.binPaths == nil {
			o.binPaths = make(map[string]string, len(m))
		}
		maps.Copy(o.binPaths, m)
	}
}

// WithLookPath supplies the PATH search used to locate agent binaries by name. It
// defaults to exec.LookPath. A successful hit must still name an executable file;
// a non-executable or failed result falls through to WithSearchDirs the same way
// a miss does. Callers (and tests) inject a closed or fixture-scoped function so
// host PATH never leaks into detection (R10).
func WithLookPath(fn func(string) (string, error)) Option {
	return func(o *options) { o.lookPath = fn }
}

// WithEnvLookup supplies the environment source for the library's own data-shaping
// reads: provider-env presence and catalog path expansion ($VAR and ~). It
// defaults to os.LookupEnv. Only presence is ever taken from a variable, never its
// value.
func WithEnvLookup(fn func(string) (string, bool)) Option {
	return func(o *options) { o.envLookup = fn }
}

// WithWorkingDir sets the base a relative local config, skills, or binary path
// resolves against. It defaults to os.Getwd, so two callers in different
// directories are told different things about the same agent only when they say so.
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

// WithLogger threads a structured logger through the library's decision points. It
// defaults to a logger over a discard handler, so the library is silent unless a
// caller opts in and never writes to a stream it was not given (R19).
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}
