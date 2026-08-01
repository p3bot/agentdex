package modelsdev

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"
)

// DefaultURL is the published models.dev catalog fetched unless overridden.
const DefaultURL = "https://models.dev/catalog.json"

// DefaultTTL is how long a cached catalog.json is served before a refetch is
// attempted. On a failed refetch the stale copy is still served.
const DefaultTTL = 24 * time.Hour

// DefaultHTTPTimeout bounds a single catalog fetch end to end. The shared
// single-flight fetch detaches from any one caller's cancellation, so without
// an overall timeout a stalled endpoint would wedge the Client permanently.
// WithHTTPClient overrides it.
const DefaultHTTPTimeout = 30 * time.Second

// maxResponseBytes caps the catalog.json read so a misbehaving endpoint cannot
// exhaust memory. The real catalog is a few megabytes.
const maxResponseBytes = 64 << 20

// Client fetches, caches, merges, and serves the models.dev catalog. Fetch,
// decode, and merge happen once per Client; the merged catalog is memoised in
// memory. A long-lived Client never re-merges — refresh needs a new Client.
// Methods are safe for concurrent use.
type Client struct {
	httpClient   *http.Client
	url          string
	cache        cache
	ttl          time.Duration
	forceRefresh bool
	now          func() time.Time

	mu       sync.Mutex
	catalog  *Catalog   // memoised once a usable result is obtained
	stale    bool       // true when memoised catalog came from post-failure cache re-decode
	inflight *fetchCall // single in-flight fetch, nil when none
}

// fetchCall is one shared fetch attempt. Concurrent callers wait on done rather
// than racing their own requests.
type fetchCall struct {
	done    chan struct{}
	catalog *Catalog
	err     error
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithURL overrides the catalog URL (mirror or frozen snapshot).
func WithURL(url string) ClientOption {
	return func(c *Client) { c.url = url }
}

// WithCacheDir overrides the stale-cache directory.
func WithCacheDir(dir string) ClientOption {
	return func(c *Client) { c.cache = cache{dir: dir} }
}

// WithTTL overrides the cache TTL.
func WithTTL(ttl time.Duration) ClientOption {
	return func(c *Client) { c.ttl = ttl }
}

// WithForceRefresh makes the next load fetch fresh bytes, ignore the cache TTL,
// and report fetch/decode failure rather than fall back to stale cache. Honest
// mode for explicit refresh: the caller learns whether fresh data was fetched.
// A successful fetch still updates the on-disk cache.
func WithForceRefresh() ClientOption {
	return func(c *Client) { c.forceRefresh = true }
}

// WithHTTPClient overrides the package-owned HTTP client, replacing the default
// timeout backstop. A consumer that supplies a client without a timeout takes on
// responsibility for bounding the fetch through the request context.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// New constructs a Client with built-in defaults: the published catalog URL, the
// cache directory under $XDG_CACHE_HOME, a 24h TTL, and a package-owned HTTP
// client. Options override any of these.
func New(opts ...ClientOption) *Client {
	c := &Client{
		// Dedicated client rather than http.DefaultClient: neither read nor mutate
		// it (still shares DefaultTransport's pool). DefaultHTTPTimeout backstops
		// a leader with no deadline so the shared fetch cannot wedge forever.
		httpClient: &http.Client{Timeout: DefaultHTTPTimeout},
		url:        DefaultURL,
		cache:      cache{dir: defaultCacheDir()},
		ttl:        DefaultTTL,
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Catalog returns the merged catalog. The first call fetches, caches, and merges;
// later calls return the memoised copy. The returned pointer is shared and must
// be treated as read-only.
func (c *Client) Catalog(ctx context.Context) (*Catalog, error) {
	return c.load(ctx)
}

// Stale reports whether the memoised catalog was served from the stale-fallback
// path: a network fetch failed and a previously cached copy was re-decoded.
// Meaningful only after a successful load; before any load, and after a failed
// load with no cache, it returns false. Within-TTL hit and fresh fetch both
// report false.
func (c *Client) Stale() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stale
}

// Provider returns one provider by id, whether it was found, and any error.
// found reports existence only and is independent of the error. A provider that
// exists but carries a malformed model returns found true with ErrModelsSchema,
// so branching on found alone cannot swallow schema drift as absence. The
// per-model check is applied to that provider only. The returned Provider shares
// the memoised catalog and must be treated as read-only.
func (c *Client) Provider(ctx context.Context, id string) (Provider, bool, error) {
	cat, err := c.load(ctx)
	if err != nil {
		return Provider{}, false, err
	}
	p, ok := cat.Providers[id]
	if !ok {
		return Provider{}, false, nil
	}
	if err := validateProvider(p); err != nil {
		return p, true, err
	}
	return p, true, nil
}

// Models returns the merged models of the named providers, sorted by id. The
// per-model check is applied only to those providers; a malformed model raises
// ErrModelsSchema. Unknown provider ids are skipped. Returned models alias the
// memoised catalog and must be treated as read-only.
func (c *Client) Models(ctx context.Context, providerIDs ...string) ([]Model, error) {
	cat, err := c.load(ctx)
	if err != nil {
		return nil, err
	}
	var models []Model
	seen := make(map[string]struct{}, len(providerIDs))
	for _, id := range providerIDs {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		p, ok := cat.Providers[id]
		if !ok {
			continue
		}
		if err := validateProvider(p); err != nil {
			return nil, err
		}
		for _, m := range p.Models {
			models = append(models, m)
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}

// Single-flight the first fetch; only a usable result is memoised so a transient
// blip does not permanently poison a long-lived Client. The shared fetch detaches
// from any single caller's cancellation (each waiter selects on its own context)
// and keeps the leader's deadline so a black-hole endpoint cannot leak it.
func (c *Client) load(ctx context.Context) (*Catalog, error) {
	c.mu.Lock()
	if c.catalog != nil {
		cat := c.catalog
		c.mu.Unlock()
		return cat, nil
	}
	call := c.inflight
	if call == nil {
		call = &fetchCall{done: make(chan struct{})}
		c.inflight = call
		c.mu.Unlock()
		go c.runFetch(ctx, call)
	} else {
		c.mu.Unlock()
	}

	select {
	case <-call.done:
		return c.afterInflight(call)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Detaches from the leader's cancellation so an unrelated cancel does not abort
// work waiters depend on, while preserving the leader's deadline as a time bound.
func (c *Client) runFetch(leaderCtx context.Context, call *fetchCall) {
	ctx := context.WithoutCancel(leaderCtx)
	if deadline, ok := leaderCtx.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	cat, stale, err := c.fetchMerge(ctx)

	c.mu.Lock()
	call.catalog, call.err = cat, err
	if err == nil {
		c.catalog = cat
		c.stale = stale
	}
	c.inflight = nil
	close(call.done)
	c.mu.Unlock()
}

// Shared error so the waiter can retry rather than inherit a cached failure.
func (c *Client) afterInflight(call *fetchCall) (*Catalog, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.catalog != nil {
		return c.catalog, nil
	}
	return call.catalog, call.err
}

// Within-TTL hit, fresh fetch, or stale copy on failure. Bool is true only for
// post-failure cache re-decode. forceRefresh skips both within-TTL hit and stale fallback.
func (c *Client) fetchMerge(ctx context.Context) (*Catalog, bool, error) {
	cachedData, modTime, cached := c.cache.read()
	if !c.forceRefresh && cached && c.now().Sub(modTime) < c.ttl {
		if cat, err := decodeValidate(cachedData); err == nil {
			merge(cat)
			return cat, false, nil
		}
		// Corrupt within-TTL cache is unusable as fresh or stale: fall through.
		cached = false
	}

	data, fetchErr := c.get(ctx)
	if fetchErr == nil {
		cat, decErr := decodeValidate(data)
		if decErr == nil {
			_ = c.cache.write(data) // best-effort: usable result returned regardless
			merge(cat)
			return cat, false, nil
		}
		fetchErr = decErr
	}

	if !c.forceRefresh && cached {
		if cat, err := decodeValidate(cachedData); err == nil {
			merge(cat)
			return cat, true, nil
		}
	}
	return nil, false, fetchErr
}

func (c *Client) get(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("build models.dev request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models.dev catalog: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch models.dev catalog: unexpected status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read models.dev catalog: %w", err)
	}
	return data, nil
}

// Decode failure or empty top-level maps are treated as fetch failures by the
// caller so a cached copy is served when one exists.
func decodeValidate(data []byte) (*Catalog, error) {
	var cat Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("decode models.dev catalog: %w", err)
	}
	if err := validateTopLevel(&cat); err != nil {
		return nil, err
	}
	return &cat, nil
}
