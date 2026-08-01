package modelsdev

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func smallCatalog() Catalog {
	return Catalog{
		Models: map[string]Model{
			"anthropic/claude-x": {ID: "anthropic/claude-x", Benchmarks: []Benchmark{{Name: "SWE-Bench"}}},
		},
		Providers: map[string]Provider{
			"anthropic": {ID: "anthropic", Env: []string{"ANTHROPIC_API_KEY"}, Models: map[string]Model{
				"claude-x": {ID: "claude-x", Limit: Limit{Context: 200000, Output: 64000}},
			}},
		},
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return data
}

func serveBytes(t *testing.T, body []byte) (url string, requests *atomic.Int64) {
	t.Helper()
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &count
}

func TestCatalogMergesRealData(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "catalog.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	url, _ := serveBytes(t, body)
	c := New(WithURL(url), WithCacheDir(t.TempDir()))

	cat, err := c.Catalog(context.Background())
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}

	kimi, ok := cat.Providers["moonshotai"].Models["kimi-k2-thinking"]
	if !ok {
		t.Fatal("expected moonshotai/kimi-k2-thinking in fixture")
	}
	if len(kimi.Benchmarks) == 0 {
		t.Error("first-party model did not receive agnostic benchmarks")
	}
	if kimi.ID != "kimi-k2-thinking" {
		t.Errorf("provider Model.ID rewritten: got %q, want short id", kimi.ID)
	}

	// Aggregator under a path-bearing key has no decomposing agnostic id.
	agg, ok := cat.Providers["requesty"].Models["xai/grok-4"]
	if !ok {
		t.Fatal("expected aggregator requesty/xai/grok-4 in fixture")
	}
	if len(agg.Benchmarks) != 0 {
		t.Errorf("aggregator model received benchmarks: %+v", agg)
	}
	if agg.ID == "requesty/xai/grok-4" {
		t.Error("aggregator Model.ID is a minted composite")
	}

	gemini := cat.Providers["google"].Models["gemini-2.5-pro"]
	if gemini.Cost == nil || len(gemini.Cost.Tiers) == 0 {
		t.Fatal("expected google/gemini-2.5-pro to carry tiered pricing")
	}
	if tier := gemini.Cost.Tiers[0].Tier; tier.Type != "context" || tier.Size != 200000 {
		t.Errorf("tier dimension not decoded: got %+v", tier)
	}
	if gemini.Cost.ContextOver200K == nil {
		t.Error("expected google/gemini-2.5-pro to carry context_over_200k pricing")
	} else if got := gemini.Cost.ContextOver200K.Input; got != 2.5 {
		t.Errorf("context_over_200k input not decoded: got %v, want 2.5", got)
	}

	// Join is partial: some first-party models attach; far from all do.
	var withBench, total int
	for _, p := range cat.Providers {
		for _, m := range p.Models {
			total++
			if len(m.Benchmarks) > 0 {
				withBench++
			}
		}
	}
	if withBench == 0 {
		t.Error("expected some provider models to attach benchmarks")
	}
	if withBench >= total {
		t.Errorf("expected a partial join, got %d/%d", withBench, total)
	}
}

func TestSingleFlightAndMemoisation(t *testing.T) {
	url, requests := serveBytes(t, mustJSON(t, smallCatalog()))
	c := New(WithURL(url), WithCacheDir(t.TempDir()))

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(i int) {
			defer wg.Done()
			switch i % 3 {
			case 0:
				_, _ = c.Catalog(context.Background())
			case 1:
				_, _, _ = c.Provider(context.Background(), "anthropic")
			default:
				_, _ = c.Models(context.Background(), "anthropic")
			}
		}(i)
	}
	wg.Wait()

	if got := requests.Load(); got != 1 {
		t.Errorf("expected exactly one upstream fetch, got %d", got)
	}

	if _, err := c.Catalog(context.Background()); err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("memoised Client refetched: %d requests", got)
	}
}

func TestLeaderCancellationDoesNotPoisonWaiters(t *testing.T) {
	// Hold responses until release so the first fetch is in flight while the
	// test arranges a waiter and cancels the leader. Once-guarded so test body
	// and cleanup can both call without double-close.
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	var count atomic.Int64
	body := mustJSON(t, smallCatalog())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		<-release
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(unblock) // unblock any handler still parked on early exit

	c := New(WithURL(srv.URL), WithCacheDir(t.TempDir()))

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	go func() { _, _ = c.Catalog(leaderCtx) }()
	waitFor(t, func() bool { return count.Load() == 1 }, "leader fetch to reach the server")

	type result struct {
		cat *Catalog
		err error
	}
	waiter := make(chan result, 1)
	go func() {
		cat, err := c.Catalog(context.Background())
		waiter <- result{cat, err}
	}()

	cancelLeader()
	unblock()

	got := <-waiter
	if got.err != nil {
		t.Fatalf("live-context waiter inherited the leader's cancellation: %v", got.err)
	}
	if got.cat == nil || got.cat.Providers["anthropic"].ID != "anthropic" {
		t.Errorf("waiter did not receive the merged catalog: %+v", got.cat)
	}
	if n := count.Load(); n != 1 {
		t.Errorf("expected one shared fetch despite leader cancellation, got %d", n)
	}
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCacheWithinTTLReadsFile(t *testing.T) {
	dir := t.TempDir()
	url, requests := serveBytes(t, mustJSON(t, smallCatalog()))

	if _, err := New(WithURL(url), WithCacheDir(dir)).Catalog(context.Background()); err != nil {
		t.Fatalf("first Catalog: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, cacheFileName)); err != nil {
		t.Fatalf("fresh fetch did not write cache file: %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("expected one fetch, got %d", got)
	}

	// Fresh Client over same cache dir within TTL must not hit the network.
	if _, err := New(WithURL(url), WithCacheDir(dir), WithTTL(time.Hour)).Catalog(context.Background()); err != nil {
		t.Fatalf("second Catalog: %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("within-TTL call refetched: %d requests", got)
	}
}

func TestCacheExpiryRefetches(t *testing.T) {
	dir := t.TempDir()
	url, requests := serveBytes(t, mustJSON(t, smallCatalog()))
	if _, err := New(WithURL(url), WithCacheDir(dir)).Catalog(context.Background()); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	// Clock past TTL forces refetch of an otherwise-fresh file.
	c := New(WithURL(url), WithCacheDir(dir), WithTTL(time.Minute))
	c.now = func() time.Time { return time.Now().Add(time.Hour) }
	if _, err := c.Catalog(context.Background()); err != nil {
		t.Fatalf("Catalog after expiry: %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Errorf("expected a refetch after TTL expiry, got %d requests", got)
	}
}

func TestStaleServedOnFetchFailure(t *testing.T) {
	dir := t.TempDir()
	good, _ := serveBytes(t, mustJSON(t, smallCatalog()))
	if _, err := New(WithURL(good), WithCacheDir(dir)).Catalog(context.Background()); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	// TTL zero forces refetch; failing endpoint falls back to stale file.
	var attempts atomic.Int64
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)

	c := New(WithURL(failing.URL), WithCacheDir(dir), WithTTL(0))
	if c.Stale() {
		t.Error("Stale before load must be false")
	}
	cat, err := c.Catalog(context.Background())
	if err != nil {
		t.Fatalf("expected stale copy served, got error: %v", err)
	}
	if _, ok := cat.Providers["anthropic"]; !ok {
		t.Error("stale catalog missing expected provider")
	}
	if !c.Stale() {
		t.Error("Stale after post-failure cache re-decode must be true")
	}

	if _, err := c.Catalog(context.Background()); err != nil {
		t.Fatalf("second Catalog: %v", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("stale-served result re-fetched: %d upstream attempts", got)
	}
	if !c.Stale() {
		t.Error("memoised stale serve must keep Stale true")
	}
}

func TestStaleFalseOnFreshAndWithinTTL(t *testing.T) {
	body := mustJSON(t, smallCatalog())
	url, _ := serveBytes(t, body)
	dir := t.TempDir()

	fresh := New(WithURL(url), WithCacheDir(dir))
	if _, err := fresh.Catalog(context.Background()); err != nil {
		t.Fatalf("fresh Catalog: %v", err)
	}
	if fresh.Stale() {
		t.Error("fresh network fetch must report not-stale")
	}

	hit := New(WithURL(url), WithCacheDir(dir), WithTTL(time.Hour))
	if _, err := hit.Catalog(context.Background()); err != nil {
		t.Fatalf("within-TTL Catalog: %v", err)
	}
	if hit.Stale() {
		t.Error("within-TTL cache hit must report not-stale")
	}
}

func TestForceRefreshFailsRatherThanServeStale(t *testing.T) {
	// WithForceRefresh reports fetch failure even when a cache exists, so an
	// explicit refresh learns the network was unreachable rather than silent stale.
	dir := t.TempDir()
	good, _ := serveBytes(t, mustJSON(t, smallCatalog()))
	if _, err := New(WithURL(good), WithCacheDir(dir)).Catalog(context.Background()); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)

	c := New(WithURL(failing.URL), WithCacheDir(dir), WithForceRefresh())
	if _, err := c.Catalog(context.Background()); err == nil {
		t.Fatal("force refresh must report the fetch failure, not serve the stale cache")
	}
}

func TestForceRefreshUpdatesCacheOnSuccess(t *testing.T) {
	// Successful force refresh writes the cache so a later ordinary client can
	// serve offline.
	dir := t.TempDir()
	url, _ := serveBytes(t, mustJSON(t, smallCatalog()))
	if _, err := New(WithURL(url), WithCacheDir(dir), WithForceRefresh()).Catalog(context.Background()); err != nil {
		t.Fatalf("force refresh: %v", err)
	}

	offline := New(WithURL("http://127.0.0.1:0"), WithCacheDir(dir), WithTTL(time.Hour))
	cat, err := offline.Catalog(context.Background())
	if err != nil {
		t.Fatalf("expected cached data served offline: %v", err)
	}
	if _, ok := cat.Providers["anthropic"]; !ok {
		t.Error("refreshed cache missing expected provider")
	}
}

func TestCorruptCacheNotServedAsStale(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, cacheFileName), []byte("{not catalog json"), 0o644); err != nil {
		t.Fatalf("seed corrupt cache: %v", err)
	}

	// Within-TTL corrupt cache is unusable as fresh or stale; with a failing
	// endpoint the fetch error must surface rather than the corrupt file.
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)
	if _, err := New(WithURL(failing.URL), WithCacheDir(dir), WithTTL(time.Hour)).Catalog(context.Background()); err == nil {
		t.Fatal("expected error: a corrupt cache must not be served as stale")
	}

	good, _ := serveBytes(t, mustJSON(t, smallCatalog()))
	cat, err := New(WithURL(good), WithCacheDir(dir), WithTTL(time.Hour)).Catalog(context.Background())
	if err != nil {
		t.Fatalf("fetch over corrupt cache: %v", err)
	}
	if _, ok := cat.Providers["anthropic"]; !ok {
		t.Error("expected a fresh catalog after the corrupt cache was replaced")
	}
}

func TestFirstFetchFailureIsRetryable(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	var count atomic.Int64
	body := mustJSON(t, smallCatalog())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	c := New(WithURL(srv.URL), WithCacheDir(t.TempDir()))

	if _, err := c.Catalog(context.Background()); err == nil {
		t.Fatal("expected first fetch to fail")
	}

	fail.Store(false)
	cat, err := c.Catalog(context.Background())
	if err != nil {
		t.Fatalf("retry after failure: %v", err)
	}
	if _, ok := cat.Providers["anthropic"]; !ok {
		t.Error("retried catalog missing expected provider")
	}
	if got := count.Load(); got != 2 {
		t.Errorf("expected the failure not to be memoised (2 fetches), got %d", got)
	}
}

func TestTopLevelSchemaError(t *testing.T) {
	empty := mustJSON(t, Catalog{Models: map[string]Model{}, Providers: map[string]Provider{}})

	t.Run("no cache surfaces error", func(t *testing.T) {
		url, _ := serveBytes(t, empty)
		c := New(WithURL(url), WithCacheDir(t.TempDir()))
		_, err := c.Catalog(context.Background())
		if !errors.Is(err, ErrModelsSchema) {
			t.Errorf("got %v, want ErrModelsSchema", err)
		}
	})

	t.Run("serves stale when cache present", func(t *testing.T) {
		dir := t.TempDir()
		good, _ := serveBytes(t, mustJSON(t, smallCatalog()))
		if _, err := New(WithURL(good), WithCacheDir(dir)).Catalog(context.Background()); err != nil {
			t.Fatalf("seed cache: %v", err)
		}
		url, _ := serveBytes(t, empty)
		cat, err := New(WithURL(url), WithCacheDir(dir), WithTTL(0)).Catalog(context.Background())
		if err != nil {
			t.Fatalf("expected stale served, got %v", err)
		}
		if _, ok := cat.Providers["anthropic"]; !ok {
			t.Error("stale catalog missing expected provider")
		}
	})
}

func TestPerRequestedProviderSchemaError(t *testing.T) {
	// Malformed model (empty id) in "broken"; "anthropic" is clean.
	cat := smallCatalog()
	cat.Providers["broken"] = Provider{ID: "broken", Models: map[string]Model{
		"bad": {ID: ""},
	}}
	url, _ := serveBytes(t, mustJSON(t, cat))
	c := New(WithURL(url), WithCacheDir(t.TempDir()))

	if _, err := c.Catalog(context.Background()); err != nil {
		t.Fatalf("Catalog must not validate per-model: %v", err)
	}
	if _, ok, err := c.Provider(context.Background(), "anthropic"); err != nil || !ok {
		t.Fatalf("clean provider: ok=%v err=%v", ok, err)
	}
	if _, err := c.Models(context.Background(), "anthropic"); err != nil {
		t.Fatalf("clean provider models: %v", err)
	}

	// found=true with ErrModelsSchema: existence is independent of schema error.
	if p, ok, err := c.Provider(context.Background(), "broken"); !errors.Is(err, ErrModelsSchema) {
		t.Errorf("Provider(broken): got %v, want ErrModelsSchema", err)
	} else if !ok || p.ID != "broken" {
		t.Errorf("Provider(broken): got found=%v id=%q, want found=true id=broken", ok, p.ID)
	}
	if _, err := c.Models(context.Background(), "broken"); !errors.Is(err, ErrModelsSchema) {
		t.Errorf("Models(broken): got %v, want ErrModelsSchema", err)
	}
}

func TestModelsDedupsRepeatedProviders(t *testing.T) {
	url, _ := serveBytes(t, mustJSON(t, smallCatalog()))
	c := New(WithURL(url), WithCacheDir(t.TempDir()))

	once, err := c.Models(context.Background(), "anthropic")
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(once) == 0 {
		t.Fatal("expected anthropic to contribute at least one model")
	}
	twice, err := c.Models(context.Background(), "anthropic", "anthropic")
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(twice) != len(once) {
		t.Errorf("repeated provider id duplicated models: got %d, want %d", len(twice), len(once))
	}
}

func TestHTTPClientTimeoutDefaultAndOverride(t *testing.T) {
	if got := New().httpClient.Timeout; got != DefaultHTTPTimeout {
		t.Errorf("default client timeout: got %v, want %v", got, DefaultHTTPTimeout)
	}
	custom := &http.Client{Timeout: 5 * time.Second}
	if got := New(WithHTTPClient(custom)).httpClient; got != custom {
		t.Errorf("WithHTTPClient did not override the client: got %p, want %p", got, custom)
	}
}

func TestProviderNotFound(t *testing.T) {
	url, _ := serveBytes(t, mustJSON(t, smallCatalog()))
	c := New(WithURL(url), WithCacheDir(t.TempDir()))
	p, ok, err := c.Provider(context.Background(), "nonexistent")
	if err != nil || ok {
		t.Errorf("unknown provider: got ok=%v err=%v, want false,nil", ok, err)
	}
	if p.ID != "" {
		t.Errorf("unknown provider returned non-zero value: %+v", p)
	}
}
