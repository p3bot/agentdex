package agentdex

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/start-cli/agentdex/internal/catalogtest"
	"github.com/start-cli/agentdex/internal/modelsdevtest"
	"github.com/start-cli/agentdex/modelsdev"
)

// seedModelsCache populates the models.dev on-disk cache under dir from a live
// catalog so a later client with TTL zero can fall back when its fetch fails.
func seedModelsCache(t *testing.T, dir string, present ...string) {
	t.Helper()
	if len(present) == 0 {
		present = []string{"anthropic", "google", "openai"}
	}
	srv := modelsdevtest.Server(t, present)
	c := modelsdev.New(modelsdev.WithURL(srv.URL), modelsdev.WithCacheDir(dir))
	if _, err := c.Catalog(context.Background()); err != nil {
		t.Fatalf("seed models.dev cache: %v", err)
	}
}

// failingModelsURL returns a models.dev URL that always responds 500.
func failingModelsURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// openStaleModels opens an Index whose models.dev client will serve the seeded
// cache as a stale fallback (TTL zero + failing URL over a warm cache dir).
func openStaleModels(t *testing.T, body string, opts ...Option) *Index {
	t.Helper()
	cache := t.TempDir()
	seedModelsCache(t, cache)
	base := []Option{
		WithCacheDir(cache),
		WithModelsURL(failingModelsURL(t)),
		WithModelsTTL(0),
	}
	return openAgents(t, body, append(base, opts...)...)
}

func TestModelsStaleColdOfflineIsErrModelsUnavailable(t *testing.T) {
	idx, err := Open(
		WithCatalogDir(catalogtest.WriteModule(t, testCatalog)),
		WithCacheDir(t.TempDir()),
		WithModelsURL(modelsdevtest.Closed(t)),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err = idx.ModelsStale(context.Background())
	if !errors.Is(err, ErrModelsUnavailable) {
		t.Fatalf("ModelsStale cold-offline error = %v, want ErrModelsUnavailable", err)
	}
}

func TestModelsStaleFreshThenStaleWithWarningInjection(t *testing.T) {
	ctx := context.Background()
	cache := t.TempDir()
	good := modelsdevtest.Server(t, []string{"anthropic", "google", "openai"})

	// Warm path: a live models.dev is not stale and raises no models-stale warning.
	warm := openAgents(t, testCatalog,
		WithCacheDir(cache),
		WithModelsURL(good.URL),
	)
	stale, err := warm.ModelsStale(ctx)
	if err != nil {
		t.Fatalf("warm ModelsStale: %v", err)
	}
	if stale {
		t.Fatal("fresh models.dev reported stale")
	}
	pres, err := warm.Providers.List(ctx, ProviderQuery{})
	if err != nil {
		t.Fatalf("warm Providers.List: %v", err)
	}
	if hasWarning(pres.Warnings, WarnModelsStale) {
		t.Errorf("fresh providers listing carried WarnModelsStale: %v", pres.Warnings)
	}

	// Stale path: same cache, TTL zero, failing endpoint — serve falls back.
	idx := openAgents(t, testCatalog,
		WithCacheDir(cache),
		WithModelsURL(failingModelsURL(t)),
		WithModelsTTL(0),
	)
	if s, err := idx.ModelsStale(ctx); err != nil || !s {
		t.Fatalf("ModelsStale = (%v, %v), want (true, nil)", s, err)
	}

	const wantMsg = "models.dev catalog is stale: refetch failed, using the cached copy"

	// Providers.List always consults models.dev.
	staleProviders, err := idx.Providers.List(ctx, ProviderQuery{})
	if err != nil {
		t.Fatalf("stale Providers.List: %v", err)
	}
	if msg, ok := warningMsg(staleProviders.Warnings, WarnModelsStale); !ok {
		t.Fatalf("stale Providers.List missing WarnModelsStale, got %v", staleProviders.Warnings)
	} else if msg != wantMsg {
		t.Errorf("Providers.List stale warning = %q, want %q", msg, wantMsg)
	}
	if len(staleProviders.Items) == 0 {
		t.Error("stale Providers.List returned no items; fallback data must still serve")
	}

	// Models.List always consults models.dev on a successful empty-scope listing.
	staleModels, err := idx.Models.List(ctx, ModelQuery{})
	if err != nil {
		t.Fatalf("stale Models.List: %v", err)
	}
	if !hasWarning(staleModels.Warnings, WarnModelsStale) {
		t.Errorf("stale Models.List missing WarnModelsStale, got %v", staleModels.Warnings)
	}

	// Agents.List at EnrichCount consults models.dev for the listing-wide probe.
	staleAgents, err := idx.Agents.List(ctx, AgentQuery{Enrich: EnrichCount})
	if err != nil {
		t.Fatalf("stale Agents.List: %v", err)
	}
	if !hasWarning(staleAgents.Warnings, WarnModelsStale) {
		t.Errorf("stale Agents.List missing WarnModelsStale, got %v", staleAgents.Warnings)
	}
	// Exactly one models-stale warning at the operation level, not per agent.
	n := 0
	for _, w := range staleAgents.Warnings {
		if w.Kind == WarnModelsStale {
			n++
		}
	}
	if n != 1 {
		t.Errorf("Agents.List WarnModelsStale count = %d, want 1", n)
	}

	// Agents.Get at EnrichFull for a home-provider agent consults models.dev.
	d, err := idx.Agents.Get(ctx, "alpha-cli", AgentGetQuery{Enrich: EnrichFull})
	if err != nil {
		t.Fatalf("stale Agents.Get: %v", err)
	}
	if !hasWarning(d.Warnings, WarnModelsStale) {
		t.Errorf("stale Agents.Get missing WarnModelsStale, got %v", d.Warnings)
	}

	// Agnostic Get at EnrichProviders with a provider set validates against models.dev.
	dAgn, err := idx.Agents.Get(ctx, "delta-agent", AgentGetQuery{
		Enrich:    EnrichProviders,
		Providers: []string{"anthropic"},
	})
	if err != nil {
		t.Fatalf("stale agnostic Get: %v", err)
	}
	if !hasWarning(dAgn.Warnings, WarnModelsStale) {
		t.Errorf("stale agnostic Get missing WarnModelsStale, got %v", dAgn.Warnings)
	}
}

func TestModelsStaleWarningOmittedWhenNotConsulted(t *testing.T) {
	ctx := context.Background()
	// Pre-load models.dev as stale on the shared client, then exercise paths that
	// must not emit WarnModelsStale even though the client bit is true.
	idx := openStaleModels(t, testCatalog)
	if s, err := idx.ModelsStale(ctx); err != nil || !s {
		t.Fatalf("precondition ModelsStale = (%v, %v), want (true, nil)", s, err)
	}

	// EnrichNone with no provider filter never reaches models.dev.
	res, err := idx.Agents.List(ctx, AgentQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("EnrichNone List: %v", err)
	}
	if hasWarning(res.Warnings, WarnModelsStale) {
		t.Errorf("EnrichNone List must not warn about models.dev staleness: %v", res.Warnings)
	}

	// Home-provider agent at EnrichProviders is offline catalog data only.
	d, err := idx.Agents.Get(ctx, "alpha-cli", AgentGetQuery{Enrich: EnrichProviders})
	if err != nil {
		t.Fatalf("home EnrichProviders Get: %v", err)
	}
	if hasWarning(d.Warnings, WarnModelsStale) {
		t.Errorf("home EnrichProviders Get must not warn about models.dev staleness: %v", d.Warnings)
	}

	// Agnostic with no provider set is not-applicable and never touches models.dev.
	dAgn, err := idx.Agents.Get(ctx, "delta-agent", AgentGetQuery{Enrich: EnrichFull})
	if err != nil {
		t.Fatalf("agnostic no-providers Get: %v", err)
	}
	if hasWarning(dAgn.Warnings, WarnModelsStale) {
		t.Errorf("agnostic no-providers Get must not warn about models.dev staleness: %v", dAgn.Warnings)
	}
	if dAgn.Enrichment != EnrichmentNotApplicable {
		t.Errorf("Enrichment = %v, want EnrichmentNotApplicable", dAgn.Enrichment)
	}
}

func TestModelsStaleWarningOnProviderFilterAtAnyEnrich(t *testing.T) {
	// A non-empty provider filter validates against models.dev even at EnrichNone.
	ctx := context.Background()
	idx := openStaleModels(t, testCatalog)

	res, err := idx.Agents.List(ctx, AgentQuery{
		Enrich:    EnrichNone,
		Providers: []string{"anthropic"},
	})
	if err != nil {
		t.Fatalf("List with provider filter: %v", err)
	}
	if !hasWarning(res.Warnings, WarnModelsStale) {
		t.Errorf("provider-filter List at EnrichNone missing WarnModelsStale: %v", res.Warnings)
	}
}

func TestModelsStaleWarningOnErrorPathAfterConsultation(t *testing.T) {
	// An unknown provider id is rejected only after models.dev was consulted, so
	// the stale warning rides the error return (R6).
	ctx := context.Background()
	idx := openStaleModels(t, testCatalog)

	res, err := idx.Agents.List(ctx, AgentQuery{
		Enrich:    EnrichNone,
		Providers: []string{"no-such-provider"},
	})
	if !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("error = %v, want ErrUnknownProvider", err)
	}
	if !hasWarning(res.Warnings, WarnModelsStale) {
		t.Errorf("error path dropped WarnModelsStale: %v", res.Warnings)
	}
}

func TestModelsListOmitsModelsStaleWhenScopeNeverConsults(t *testing.T) {
	// Pre-load a stale models.dev serve, then fail Models.List in resolveModelScope
	// before any models.dev call: the shared client's bit must not leak onto a path
	// that never consulted it.
	ctx := context.Background()
	idx := openStaleModels(t, testCatalog)
	if s, err := idx.ModelsStale(ctx); err != nil || !s {
		t.Fatalf("precondition ModelsStale = (%v, %v), want (true, nil)", s, err)
	}

	unknown, err := idx.Models.List(ctx, ModelQuery{Scope: ModelScope{Agent: "no-such-agent"}})
	if !errors.Is(err, ErrAgentUnknown) {
		t.Fatalf("unknown agent error = %v, want ErrAgentUnknown", err)
	}
	if hasWarning(unknown.Warnings, WarnModelsStale) {
		t.Errorf("unknown-agent Models.List must not warn about models.dev staleness: %v", unknown.Warnings)
	}

	required, err := idx.Models.List(ctx, ModelQuery{Scope: ModelScope{Agent: "delta-agent"}})
	if !errors.Is(err, ErrProvidersRequired) {
		t.Fatalf("agnostic no-providers error = %v, want ErrProvidersRequired", err)
	}
	if hasWarning(required.Warnings, WarnModelsStale) {
		t.Errorf("providers-required Models.List must not warn about models.dev staleness: %v", required.Warnings)
	}
}

func TestRefreshModelsClearsModelsStale(t *testing.T) {
	// A successful models refresh installs a force-fetched client, so ModelsStale
	// and WarnModelsStale must clear the way CatalogStale does after a catalog refresh.
	ctx := context.Background()
	cache := t.TempDir()
	seedModelsCache(t, cache)

	var fail atomic.Bool
	fail.Store(true)
	body := modelsCatalogJSON(t, "anthropic", "google", "openai")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	idx := openAgents(t, testCatalog,
		WithCacheDir(cache),
		WithModelsURL(srv.URL),
		WithModelsTTL(0),
	)
	if s, err := idx.ModelsStale(ctx); err != nil || !s {
		t.Fatalf("ModelsStale before refresh = (%v, %v), want (true, nil)", s, err)
	}
	before, err := idx.Providers.List(ctx, ProviderQuery{})
	if err != nil {
		t.Fatalf("stale Providers.List: %v", err)
	}
	if !hasWarning(before.Warnings, WarnModelsStale) {
		t.Fatalf("pre-refresh listing missing WarnModelsStale: %v", before.Warnings)
	}

	fail.Store(false)
	refreshed, err := idx.Refresh(ctx, TargetModels)
	if err != nil {
		t.Fatalf("Refresh models: %v", err)
	}
	if !refreshed.Models {
		t.Error("Refreshed.Models = false, want true")
	}
	if s, err := idx.ModelsStale(ctx); err != nil || s {
		t.Errorf("ModelsStale after successful refresh = (%v, %v), want (false, nil)", s, err)
	}
	after, err := idx.Providers.List(ctx, ProviderQuery{})
	if err != nil {
		t.Fatalf("post-refresh Providers.List: %v", err)
	}
	if hasWarning(after.Warnings, WarnModelsStale) {
		t.Errorf("post-refresh listing still carried WarnModelsStale: %v", after.Warnings)
	}
}
