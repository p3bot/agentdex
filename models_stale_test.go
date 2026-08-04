package agentdex

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/p3bot/agentdex/internal/catalogtest"
	"github.com/p3bot/agentdex/internal/modelsdevtest"
	"github.com/p3bot/agentdex/modelsdev"
)

// Warm on-disk models.dev cache so TTL-zero + failed fetch can fall back.
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

func failingModelsURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// TTL zero + failing URL over a warm cache: stale models.dev fallback.
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

	idx := openAgents(t, testCatalog,
		WithCacheDir(cache),
		WithModelsURL(failingModelsURL(t)),
		WithModelsTTL(0),
	)
	if s, err := idx.ModelsStale(ctx); err != nil || !s {
		t.Fatalf("ModelsStale = (%v, %v), want (true, nil)", s, err)
	}

	const wantMsg = "models.dev catalog is stale: refetch failed, using the cached copy"

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

	staleModels, err := idx.Models.List(ctx, ModelQuery{})
	if err != nil {
		t.Fatalf("stale Models.List: %v", err)
	}
	if !hasWarning(staleModels.Warnings, WarnModelsStale) {
		t.Errorf("stale Models.List missing WarnModelsStale, got %v", staleModels.Warnings)
	}

	staleAgents, err := idx.Agents.List(ctx, AgentQuery{Enrich: EnrichCount})
	if err != nil {
		t.Fatalf("stale Agents.List: %v", err)
	}
	if !hasWarning(staleAgents.Warnings, WarnModelsStale) {
		t.Errorf("stale Agents.List missing WarnModelsStale, got %v", staleAgents.Warnings)
	}
	// Operation-level warning once, not per agent.
	n := 0
	for _, w := range staleAgents.Warnings {
		if w.Kind == WarnModelsStale {
			n++
		}
	}
	if n != 1 {
		t.Errorf("Agents.List WarnModelsStale count = %d, want 1", n)
	}

	d, err := idx.Agents.Get(ctx, "alpha-cli", AgentGetQuery{Enrich: EnrichFull})
	if err != nil {
		t.Fatalf("stale Agents.Get: %v", err)
	}
	if !hasWarning(d.Warnings, WarnModelsStale) {
		t.Errorf("stale Agents.Get missing WarnModelsStale, got %v", d.Warnings)
	}

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
	// Stale client bit set; paths below must not emit WarnModelsStale.
	idx := openStaleModels(t, testCatalog)
	if s, err := idx.ModelsStale(ctx); err != nil || !s {
		t.Fatalf("precondition ModelsStale = (%v, %v), want (true, nil)", s, err)
	}

	res, err := idx.Agents.List(ctx, AgentQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("EnrichNone List: %v", err)
	}
	if hasWarning(res.Warnings, WarnModelsStale) {
		t.Errorf("EnrichNone List must not warn about models.dev staleness: %v", res.Warnings)
	}

	d, err := idx.Agents.Get(ctx, "alpha-cli", AgentGetQuery{Enrich: EnrichProviders})
	if err != nil {
		t.Fatalf("home EnrichProviders Get: %v", err)
	}
	if hasWarning(d.Warnings, WarnModelsStale) {
		t.Errorf("home EnrichProviders Get must not warn about models.dev staleness: %v", d.Warnings)
	}

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
	// Unknown id is rejected after consultation; stale warning rides the error.
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
	// Fail in resolveModelScope before models.dev; client stale bit must not leak.
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
