package agentdex

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/p3bot/agentdex/internal/catalog"
	"github.com/p3bot/agentdex/internal/catalogtest"
	"github.com/p3bot/agentdex/internal/modelsdevtest"
	"github.com/p3bot/agentdex/modelsdev"
)

func TestFetchAlignsAcrossPackages(t *testing.T) {
	cases := []struct {
		a Fetch
		c catalog.Fetch
		m modelsdev.Fetch
	}{
		{FetchCached, catalog.FetchCached, modelsdev.FetchCached},
		{FetchCacheOnly, catalog.FetchCacheOnly, modelsdev.FetchCacheOnly},
		{FetchTTL, catalog.FetchTTL, modelsdev.FetchTTL},
		{FetchLatest, catalog.FetchLatest, modelsdev.FetchLatest},
	}
	for _, tc := range cases {
		if int(tc.a) != int(tc.c) || int(tc.a) != int(tc.m) {
			t.Errorf("%s iota diverged: agentdex=%d catalog=%d modelsdev=%d",
				tc.a, int(tc.a), int(tc.c), int(tc.m))
		}
		if tc.a.String() != tc.c.String() || tc.a.String() != tc.m.String() {
			t.Errorf("String diverged: agentdex=%q catalog=%q modelsdev=%q",
				tc.a.String(), tc.c.String(), tc.m.String())
		}
		if cs, ok := tc.a.asCatalog(); !ok || cs != tc.c {
			t.Errorf("%s.asCatalog() = (%v, %v), want (%v, true)", tc.a, cs, ok, tc.c)
		}
		if ms, ok := tc.a.asModels(); !ok || ms != tc.m {
			t.Errorf("%s.asModels() = (%v, %v), want (%v, true)", tc.a, ms, ok, tc.m)
		}
	}
	if _, ok := Fetch(99).asCatalog(); ok {
		t.Error("unknown Fetch mapped to a catalog fetch")
	}
	if _, ok := Fetch(99).asModels(); ok {
		t.Error("unknown Fetch mapped to a models.dev fetch")
	}
}

func TestOpenRejectsUnknownFetch(t *testing.T) {
	_, err := Open(WithCatalogFetch(Fetch(99)), WithModelsURL(modelsdevtest.MustNotFetch(t)))
	if err == nil {
		t.Fatal("Open with unknown catalog fetch: want error")
	}
	_, err = Open(WithModelsFetch(Fetch(99)), WithModelsURL(modelsdevtest.MustNotFetch(t)))
	if err == nil {
		t.Fatal("Open with unknown models.dev fetch: want error")
	}
}

func TestCatalogCachedWarmOfflineDoesNotResolve(t *testing.T) {
	ctx := context.Background()
	resCache := t.TempDir()
	warm, closeReg := openRegistry(t, resCache)
	if _, err := warm.Agents.List(ctx, AgentQuery{Enrich: EnrichNone}); err != nil {
		t.Fatalf("warm List: %v", err)
	}
	closeReg()

	for _, src := range []Fetch{FetchCached, FetchCacheOnly} {
		t.Run(src.String(), func(t *testing.T) {
			idx, err := Open(
				WithCacheDir(resCache),
				WithCatalogFetch(src),
				WithCatalogTTL(0),
				WithModelsURL(modelsdevtest.MustNotFetch(t)),
			)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			res, err := idx.Agents.List(ctx, AgentQuery{Enrich: EnrichNone})
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if hasWarning(res.Warnings, WarnStaleCatalog) {
				t.Errorf("%s listing carried WarnStaleCatalog", src)
			}
			info, err := idx.CatalogInfo(ctx)
			if err != nil {
				t.Fatalf("CatalogInfo: %v", err)
			}
			if info.Stale {
				t.Errorf("%s CatalogInfo.Stale = true", src)
			}
			if info.Version == "" {
				t.Error("CatalogInfo.Version empty")
			}
			got, err := idx.Agents.Get(ctx, res.Items[0].ID, AgentGetQuery{Enrich: EnrichNone})
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if hasWarning(got.Warnings, WarnStaleCatalog) {
				t.Errorf("%s Get carried WarnStaleCatalog", src)
			}
		})
	}
}

func TestCatalogCacheOnlyColdIsUnavailable(t *testing.T) {
	idx, closeReg := openRegistry(t, t.TempDir(), WithCatalogFetch(FetchCacheOnly))
	closeReg()

	_, err := idx.Agents.List(context.Background(), AgentQuery{Enrich: EnrichNone})
	if !errors.Is(err, ErrCatalogUnavailable) {
		t.Fatalf("CacheOnly cold List error = %v, want ErrCatalogUnavailable", err)
	}
}

func TestCatalogCachedColdFillsThenCacheOnlyOffline(t *testing.T) {
	ctx := context.Background()
	resCache := t.TempDir()
	fill, closeReg := openRegistry(t, resCache, WithCatalogFetch(FetchCached))
	if _, err := fill.Agents.List(ctx, AgentQuery{Enrich: EnrichNone}); err != nil {
		t.Fatalf("cold Cached List: %v", err)
	}
	closeReg()

	only, err := Open(
		WithCacheDir(resCache),
		WithCatalogFetch(FetchCacheOnly),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := only.Agents.List(ctx, AgentQuery{Enrich: EnrichNone}); err != nil {
		t.Fatalf("CacheOnly after fill: %v", err)
	}
}

func TestCatalogLatestWarmOfflineIsUnavailable(t *testing.T) {
	ctx := context.Background()
	resCache := t.TempDir()
	warm, closeReg := openRegistry(t, resCache)
	if _, err := warm.CatalogInfo(ctx); err != nil {
		t.Fatalf("warm CatalogInfo: %v", err)
	}
	closeReg()

	idx, err := Open(
		WithCacheDir(resCache),
		WithCatalogFetch(FetchLatest),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err = idx.Agents.List(ctx, AgentQuery{Enrich: EnrichNone})
	if !errors.Is(err, ErrCatalogUnavailable) {
		t.Fatalf("Latest offline List error = %v, want ErrCatalogUnavailable", err)
	}
	if msg := err.Error(); strings.Contains(msg, "no cached fallback available") {
		t.Errorf("Latest error claimed no cache: %s", msg)
	} else if !strings.Contains(msg, "unused") {
		t.Errorf("Latest error = %q, want unused cached version named", msg)
	}
}

func TestRefreshModelsIsLatestDespiteCachedOpen(t *testing.T) {
	ctx := context.Background()
	cache := t.TempDir()
	seedModelsCache(t, cache)
	dir := catalogtest.WriteModule(t, testCatalog)
	idx, err := Open(
		WithCatalogDir(dir),
		WithCacheDir(cache),
		WithModelsURL(failingModelsURL(t)),
		WithModelsFetch(FetchCached),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := idx.Providers.List(ctx, ProviderQuery{}); err != nil {
		t.Fatalf("warm Providers.List: %v", err)
	}

	refreshed, err := idx.Refresh(ctx, TargetModels)
	if !errors.Is(err, ErrModelsUnavailable) {
		t.Fatalf("Refresh error = %v, want ErrModelsUnavailable", err)
	}
	if refreshed.Models {
		t.Error("Refreshed.Models = true on a failed Latest refresh")
	}

	if _, err := idx.Providers.List(ctx, ProviderQuery{}); err != nil {
		t.Fatalf("post-failure Providers.List: %v", err)
	}
}

func TestRefreshCatalogIsLatestDespiteCachedOpen(t *testing.T) {
	ctx := context.Background()
	resCache := t.TempDir()
	idx, closeReg := openRegistry(t, resCache, WithCatalogFetch(FetchCached))
	before, err := idx.CatalogInfo(ctx)
	if err != nil {
		t.Fatalf("CatalogInfo: %v", err)
	}
	closeReg()

	refreshed, err := idx.Refresh(ctx, TargetCatalog)
	if !errors.Is(err, ErrCatalogUnavailable) {
		t.Fatalf("Refresh error = %v, want ErrCatalogUnavailable", err)
	}
	if msg := err.Error(); strings.Contains(msg, "no cached fallback available") {
		t.Errorf("Refresh error claimed no cache: %s", msg)
	} else if !strings.Contains(msg, "unused") {
		t.Errorf("Refresh error = %q, want unused cached version named", msg)
	}
	if refreshed.Catalog {
		t.Error("Refreshed.Catalog = true on a failed Latest refresh")
	}
	after, err := idx.CatalogInfo(ctx)
	if err != nil {
		t.Fatalf("CatalogInfo after failed refresh: %v", err)
	}
	if after != before {
		t.Errorf("failed refresh changed CatalogInfo: before=%+v after=%+v", before, after)
	}
}

func TestCatalogDirIgnoresFetchMode(t *testing.T) {
	dir := catalogtest.WriteModule(t, testCatalog)
	idx, err := Open(
		WithCatalogDir(dir),
		WithCatalogFetch(FetchCacheOnly),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	info, err := idx.CatalogInfo(context.Background())
	if err != nil {
		t.Fatalf("CatalogInfo: %v", err)
	}
	if info.Origin != CatalogOriginDir {
		t.Errorf("Origin = %v, want CatalogOriginDir", info.Origin)
	}
}

func TestModelsCachedWarmOffline(t *testing.T) {
	ctx := context.Background()
	cache := t.TempDir()
	seedModelsCache(t, cache)
	dir := catalogtest.WriteModule(t, testCatalog)
	idx, err := Open(
		WithCatalogDir(dir),
		WithCacheDir(cache),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
		WithModelsFetch(FetchCached),
		WithModelsTTL(0),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	res, err := idx.Providers.List(ctx, ProviderQuery{})
	if err != nil {
		t.Fatalf("Providers.List: %v", err)
	}
	if len(res.Items) == 0 {
		t.Fatal("expected cached providers")
	}
	stale, err := idx.ModelsStale(ctx)
	if err != nil || stale {
		t.Errorf("ModelsStale = (%v, %v), want (false, nil)", stale, err)
	}
}

func TestModelsCacheOnlyColdIsUnavailable(t *testing.T) {
	dir := catalogtest.WriteModule(t, testCatalog)
	idx, err := Open(
		WithCatalogDir(dir),
		WithCacheDir(t.TempDir()),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
		WithModelsFetch(FetchCacheOnly),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err = idx.Providers.List(context.Background(), ProviderQuery{})
	if !errors.Is(err, ErrModelsUnavailable) {
		t.Fatalf("CacheOnly cold Providers.List error = %v, want ErrModelsUnavailable", err)
	}
}

func TestModelsLatestDoesNotServeStale(t *testing.T) {
	cache := t.TempDir()
	seedModelsCache(t, cache)
	dir := catalogtest.WriteModule(t, testCatalog)
	idx, err := Open(
		WithCatalogDir(dir),
		WithCacheDir(cache),
		WithModelsURL(failingModelsURL(t)),
		WithModelsFetch(FetchLatest),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err = idx.Providers.List(context.Background(), ProviderQuery{})
	if !errors.Is(err, ErrModelsUnavailable) {
		t.Fatalf("Latest Providers.List error = %v, want ErrModelsUnavailable", err)
	}
}
