package agentdex

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/p3bot/agentdex/internal/catalogtest"
	"github.com/p3bot/agentdex/internal/modelsdevtest"
	"github.com/p3bot/agentdex/modelsdev"
)

// In-process OCI registry; resCache is shared so a second Index can reuse a resolution.
// Closer takes the registry offline for stale/cold-offline paths.
func openRegistry(t *testing.T, resCache string, opts ...Option) (*Index, func()) {
	t.Helper()
	_, closeReg := catalogtest.StartRegistry(t)
	base := []Option{WithCacheDir(resCache), WithModelsURL(modelsdevtest.MustNotFetch(t))}
	idx, err := Open(append(base, opts...)...)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return idx, closeReg
}

// One canonical id so the top-level models.dev shape validates.
func modelsCatalogJSON(t *testing.T, present ...string) []byte {
	t.Helper()
	cat := modelsdev.Catalog{
		Models: map[string]modelsdev.Model{
			"anthropic/claude-sonnet": {ID: "anthropic/claude-sonnet", Name: "Claude Sonnet", Limit: modelsdev.Limit{Context: 200000}},
		},
		Providers: map[string]modelsdev.Provider{},
	}
	for _, pid := range present {
		cat.Providers[pid] = modelsdevtest.Provider(pid, false)
	}
	data, err := json.Marshal(cat)
	if err != nil {
		t.Fatalf("marshal models catalog: %v", err)
	}
	return data
}

// Provider set swappable at runtime; set before the first fetch.
func mutableModelsServer(t *testing.T) (url string, set func(present ...string)) {
	t.Helper()
	var mu sync.Mutex
	var data []byte
	set = func(present ...string) {
		b := modelsCatalogJSON(t, present...)
		mu.Lock()
		data = b
		mu.Unlock()
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		b := data
		mu.Unlock()
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, set
}

func TestOpenRejectsDirAndModuleTogether(t *testing.T) {
	dir := catalogtest.WriteModule(t, testCatalog)
	_, err := Open(
		WithCatalogDir(dir),
		WithCatalogModule("github.com/p3bot/agentdex/catalog@v1"),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	if err == nil {
		t.Fatal("Open with both catalog sources: want error, got nil")
	}
	const want = "agentdex: WithCatalogDir and WithCatalogModule are mutually exclusive"
	if err.Error() != want {
		t.Errorf("Open error = %q, want %q", err.Error(), want)
	}
}

func TestCatalogStaleColdOfflineIsErrCatalogUnavailable(t *testing.T) {
	// Registry offline with no prior resolution: first catalog touch must fail.
	idx, closeReg := openRegistry(t, t.TempDir())
	closeReg()

	_, err := idx.CatalogStale(context.Background())
	if !errors.Is(err, ErrCatalogUnavailable) {
		t.Fatalf("CatalogStale cold-offline error = %v, want ErrCatalogUnavailable", err)
	}
	_, err = idx.CatalogInfo(context.Background())
	if !errors.Is(err, ErrCatalogUnavailable) {
		t.Fatalf("CatalogInfo cold-offline error = %v, want ErrCatalogUnavailable", err)
	}
}

func TestCatalogInfoDirectorySource(t *testing.T) {
	ctx := context.Background()
	dir := catalogtest.WriteModule(t, testCatalog)
	idx, err := Open(WithCatalogDir(dir),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	info, err := idx.CatalogInfo(ctx)
	if err != nil {
		t.Fatalf("CatalogInfo: %v", err)
	}
	if info.Source != CatalogSourceDir {
		t.Errorf("Source = %v, want CatalogSourceDir", info.Source)
	}
	if info.Dir != dir {
		t.Errorf("Dir = %q, want %q", info.Dir, dir)
	}
	if info.Module != "" || info.Version != "" || info.Stale {
		t.Errorf("dir source should have empty module/version and Stale=false: %+v", info)
	}
	stale, err := idx.CatalogStale(ctx)
	if err != nil || stale {
		t.Errorf("CatalogStale on dir source = (%v, %v), want (false, nil)", stale, err)
	}
}

func TestCatalogInfoDirectoryInvalidIsErrCatalogInvalid(t *testing.T) {
	dir := catalogtest.FixtureDir(t, "catalog-invalid")
	idx, err := Open(WithCatalogDir(dir),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	info, err := idx.CatalogInfo(context.Background())
	if !errors.Is(err, ErrCatalogInvalid) {
		t.Fatalf("CatalogInfo error = %v, want ErrCatalogInvalid", err)
	}
	if info != (CatalogInfo{}) {
		t.Errorf("failed CatalogInfo returned non-zero identity: %+v", info)
	}
}

func TestCatalogInfoRegistrySource(t *testing.T) {
	ctx := context.Background()
	idx, closeReg := openRegistry(t, t.TempDir())
	t.Cleanup(closeReg)

	info, err := idx.CatalogInfo(ctx)
	if err != nil {
		t.Fatalf("CatalogInfo: %v", err)
	}
	if info.Source != CatalogSourceRegistry {
		t.Errorf("Source = %v, want CatalogSourceRegistry", info.Source)
	}
	if info.Module != "github.com/p3bot/agentdex/catalog@v1" {
		t.Errorf("Module = %q, want default major-line path", info.Module)
	}
	if info.Version == "" {
		t.Error("Version empty on a successful registry load")
	}
	if info.Dir != "" || info.Stale {
		t.Errorf("fresh registry info: Dir=%q Stale=%v", info.Dir, info.Stale)
	}
	stale, err := idx.CatalogStale(ctx)
	if err != nil || stale {
		t.Errorf("CatalogStale = (%v, %v), want (false, nil)", stale, err)
	}
}

func TestCatalogInfoHonoursModuleOverride(t *testing.T) {
	ctx := context.Background()
	const module = "github.com/p3bot/agentdex/catalog@v1"

	idx, closeReg := openRegistry(t, t.TempDir(), WithCatalogModule(module))
	t.Cleanup(closeReg)
	info, err := idx.CatalogInfo(ctx)
	if err != nil {
		t.Fatalf("CatalogInfo: %v", err)
	}
	if info.Module != module {
		t.Errorf("Module = %q, want override %q", info.Module, module)
	}
	if info.Source != CatalogSourceRegistry || info.Version == "" {
		t.Errorf("override load incomplete: %+v", info)
	}

	// Unpublished override fails; default path would succeed on the same registry.
	bad, closeBad := openRegistry(t, t.TempDir(), WithCatalogModule("example.com/not/a/catalog@v1"))
	t.Cleanup(closeBad)
	info, err = bad.CatalogInfo(ctx)
	if !errors.Is(err, ErrCatalogUnavailable) {
		t.Fatalf("unknown module CatalogInfo error = %v, want ErrCatalogUnavailable", err)
	}
	if info != (CatalogInfo{}) {
		t.Errorf("failed CatalogInfo returned non-zero identity: %+v", info)
	}
}

func TestCatalogInfoMemoisedAfterLoad(t *testing.T) {
	ctx := context.Background()
	idx, closeReg := openRegistry(t, t.TempDir())
	t.Cleanup(closeReg)

	first, err := idx.CatalogInfo(ctx)
	if err != nil {
		t.Fatalf("first CatalogInfo: %v", err)
	}
	// Memoised: same identity without re-resolving after the registry is closed.
	closeReg()
	second, err := idx.CatalogInfo(ctx)
	if err != nil {
		t.Fatalf("second CatalogInfo after registry close: %v", err)
	}
	if first != second {
		t.Errorf("memoised CatalogInfo diverged: first=%+v second=%+v", first, second)
	}
}

func TestCatalogStaleFreshThenStaleWithWarningInjection(t *testing.T) {
	ctx := context.Background()
	resCache := t.TempDir()

	warm, closeReg := openRegistry(t, resCache)
	stale, err := warm.CatalogStale(ctx)
	if err != nil {
		t.Fatalf("warm CatalogStale: %v", err)
	}
	if stale {
		t.Fatal("freshly resolved catalog reported stale")
	}
	res, err := warm.Agents.List(ctx, AgentQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("warm List: %v", err)
	}
	if hasWarning(res.Warnings, WarnStaleCatalog) {
		t.Error("fresh catalog listing carried a stale warning")
	}

	// Offline + TTL 0: re-resolution fails, last version reused as stale.
	closeReg()
	idx, err := Open(WithCacheDir(resCache),
		WithCatalogTTL(0),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	if err != nil {
		t.Fatalf("Open stale index: %v", err)
	}

	staleRes, err := idx.Agents.List(ctx, AgentQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("stale List: %v", err)
	}
	msg, ok := warningMsg(staleRes.Warnings, WarnStaleCatalog)
	if !ok {
		t.Fatalf("stale listing missing WarnStaleCatalog, got %v", staleRes.Warnings)
	}
	const want = "agentdex catalog is stale: re-resolution failed, using the last resolved version"
	if msg != want {
		t.Errorf("stale warning = %q, want %q", msg, want)
	}
	if s, err := idx.CatalogStale(ctx); err != nil || !s {
		t.Errorf("CatalogStale = (%v, %v), want (true, nil)", s, err)
	}
	info, err := idx.CatalogInfo(ctx)
	if err != nil {
		t.Fatalf("CatalogInfo under stale: %v", err)
	}
	if !info.Stale || info.Source != CatalogSourceRegistry || info.Version == "" {
		t.Errorf("stale CatalogInfo = %+v, want Stale registry with a version", info)
	}

	// Stale warning rides the error return.
	d, err := idx.Agents.Get(ctx, "no-such-agent", AgentGetQuery{Enrich: EnrichNone})
	if !errors.Is(err, ErrAgentUnknown) {
		t.Fatalf("Get unknown error = %v, want ErrAgentUnknown", err)
	}
	if !hasWarning(d.Warnings, WarnStaleCatalog) {
		t.Errorf("error return dropped the stale warning, got %v", d.Warnings)
	}

	mres, err := idx.Models.List(ctx, ModelQuery{Scope: ModelScope{Agent: "delta-agent"}})
	if !errors.Is(err, ErrProvidersRequired) {
		t.Fatalf("Models.List agnostic error = %v, want ErrProvidersRequired", err)
	}
	if !hasWarning(mres.Warnings, WarnStaleCatalog) {
		t.Errorf("Models.List error dropped the stale warning, got %v", mres.Warnings)
	}
}

func TestRefreshCatalogSuccess(t *testing.T) {
	ctx := context.Background()
	idx, closeReg := openRegistry(t, t.TempDir())
	t.Cleanup(closeReg)

	before, err := idx.CatalogInfo(ctx)
	if err != nil {
		t.Fatalf("CatalogInfo before refresh: %v", err)
	}

	refreshed, err := idx.Refresh(ctx, TargetCatalog)
	if err != nil {
		t.Fatalf("Refresh catalog: %v", err)
	}
	if !refreshed.Catalog {
		t.Error("Refreshed.Catalog = false, want true")
	}
	if refreshed.Models {
		t.Error("Refreshed.Models = true on a catalog-only refresh")
	}
	if s, err := idx.CatalogStale(ctx); err != nil || s {
		t.Errorf("CatalogStale after a successful refresh = (%v, %v), want (false, nil)", s, err)
	}

	after, err := idx.CatalogInfo(ctx)
	if err != nil {
		t.Fatalf("CatalogInfo after refresh: %v", err)
	}
	if after.Source != CatalogSourceRegistry || after.Module != before.Module {
		t.Errorf("refresh changed source identity: before=%+v after=%+v", before, after)
	}
	if after.Version == "" || after.Stale || after.Dir != "" {
		t.Errorf("post-refresh CatalogInfo = %+v, want non-stale registry version", after)
	}
	if after.Version != before.Version {
		t.Errorf("Version after refresh = %q, want %q", after.Version, before.Version)
	}
}

func TestRefreshCatalogStaleFallbackIsErrorAndKeepsState(t *testing.T) {
	ctx := context.Background()
	resCache := t.TempDir()
	idx, closeReg := openRegistry(t, resCache)

	before, err := idx.Agents.List(ctx, AgentQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("warm List: %v", err)
	}
	infoBefore, err := idx.CatalogInfo(ctx)
	if err != nil {
		t.Fatalf("CatalogInfo before failed refresh: %v", err)
	}
	if infoBefore.Stale {
		t.Fatal("warm CatalogInfo reported stale")
	}

	closeReg()
	refreshed, err := idx.Refresh(ctx, TargetCatalog)
	if !errors.Is(err, ErrCatalogUnavailable) {
		t.Fatalf("Refresh stale error = %v, want ErrCatalogUnavailable", err)
	}
	if refreshed.Catalog {
		t.Error("Refreshed.Catalog = true on a stale-fallback refresh")
	}

	// Failed refresh leaves prior state untouched (no stale warning).
	after, err := idx.Agents.List(ctx, AgentQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("post-failure List: %v", err)
	}
	if len(after.Items) != len(before.Items) {
		t.Errorf("post-failure listing has %d agents, want %d", len(after.Items), len(before.Items))
	}
	if hasWarning(after.Warnings, WarnStaleCatalog) {
		t.Error("a failed refresh left the index reporting stale")
	}

	infoAfter, err := idx.CatalogInfo(ctx)
	if err != nil {
		t.Fatalf("CatalogInfo after failed refresh: %v", err)
	}
	if infoAfter != infoBefore {
		t.Errorf("failed refresh changed CatalogInfo: before=%+v after=%+v", infoBefore, infoAfter)
	}
}

func TestRefreshModelsServesFreshDataThroughSameIndex(t *testing.T) {
	ctx := context.Background()
	url, set := mutableModelsServer(t)
	set("anthropic")

	dir := catalogtest.WriteModule(t, testCatalog)
	idx, err := Open(WithCatalogDir(dir), WithCacheDir(t.TempDir()), WithModelsURL(url))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	first, err := idx.Providers.List(ctx, ProviderQuery{})
	if err != nil {
		t.Fatalf("first Providers.List: %v", err)
	}
	if len(first.Items) != 1 || first.Items[0].ID != "anthropic" {
		t.Fatalf("first listing = %v, want [anthropic]", providerIDs(first.Items))
	}

	// Memoised client keeps the old set until Refresh installs a fresh client.
	set("anthropic", "openai")
	stillOld, err := idx.Providers.List(ctx, ProviderQuery{})
	if err != nil {
		t.Fatalf("pre-refresh Providers.List: %v", err)
	}
	if len(stillOld.Items) != 1 {
		t.Fatalf("pre-refresh listing = %v, want the memoised [anthropic]", providerIDs(stillOld.Items))
	}

	refreshed, err := idx.Refresh(ctx, TargetModels)
	if err != nil {
		t.Fatalf("Refresh models: %v", err)
	}
	if !refreshed.Models || refreshed.Catalog {
		t.Errorf("Refreshed = %+v, want {Catalog:false Models:true}", refreshed)
	}

	fresh, err := idx.Providers.List(ctx, ProviderQuery{})
	if err != nil {
		t.Fatalf("post-refresh Providers.List: %v", err)
	}
	if len(fresh.Items) != 2 {
		t.Errorf("post-refresh listing = %v, want [anthropic openai]", providerIDs(fresh.Items))
	}
}

func TestRefreshModelsUnreachableIsError(t *testing.T) {
	ctx := context.Background()
	dir := catalogtest.WriteModule(t, testCatalog)
	idx, err := Open(WithCatalogDir(dir), WithCacheDir(t.TempDir()), WithModelsURL(modelsdevtest.Closed(t)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := idx.Refresh(ctx, TargetModels); !errors.Is(err, ErrModelsUnavailable) {
		t.Fatalf("Refresh unreachable error = %v, want ErrModelsUnavailable", err)
	}
}

func TestRefreshModelsSchemaDriftIsError(t *testing.T) {
	ctx := context.Background()
	// Gross drift (empty maps): data fault, not an outage.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"models":{},"providers":{}}`))
	}))
	t.Cleanup(srv.Close)

	dir := catalogtest.WriteModule(t, testCatalog)
	idx, err := Open(WithCatalogDir(dir), WithCacheDir(t.TempDir()), WithModelsURL(srv.URL))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := idx.Refresh(ctx, TargetModels); !errors.Is(err, modelsdev.ErrModelsSchema) {
		t.Fatalf("Refresh drift error = %v, want modelsdev.ErrModelsSchema", err)
	}
}

func TestRefreshDirectoryCatalogNotRefreshed(t *testing.T) {
	ctx := context.Background()
	url, set := mutableModelsServer(t)
	set("anthropic")

	dir := catalogtest.WriteModule(t, testCatalog)
	idx, err := Open(WithCatalogDir(dir), WithCacheDir(t.TempDir()), WithModelsURL(url))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	refreshed, err := idx.Refresh(ctx, TargetCatalog)
	if err != nil {
		t.Fatalf("Refresh directory catalog: %v", err)
	}
	if refreshed.Catalog {
		t.Error("a directory catalog reported as refreshed; it has no version to re-resolve")
	}

	all, err := idx.Refresh(ctx, TargetAll)
	if err != nil {
		t.Fatalf("Refresh all over directory catalog: %v", err)
	}
	if all.Catalog {
		t.Error("Refreshed.Catalog = true for a directory catalog under TargetAll")
	}
	if !all.Models {
		t.Error("Refreshed.Models = false under TargetAll over a directory catalog")
	}
}

func TestRefreshAllStopsAtFirstCatalogFailure(t *testing.T) {
	ctx := context.Background()
	resCache := t.TempDir()
	// openRegistry uses MustNotFetch; models must not be attempted after catalog fails.
	idx, closeReg := openRegistry(t, resCache)
	if _, err := idx.CatalogStale(ctx); err != nil {
		t.Fatalf("warm CatalogStale: %v", err)
	}
	closeReg()

	refreshed, err := idx.Refresh(ctx, TargetAll)
	if !errors.Is(err, ErrCatalogUnavailable) {
		t.Fatalf("Refresh all error = %v, want ErrCatalogUnavailable", err)
	}
	if refreshed.Catalog || refreshed.Models {
		t.Errorf("Refreshed = %+v, want both false when the catalog target fails first", refreshed)
	}
}

func TestIndexConcurrentUseUnderRace(t *testing.T) {
	ctx := context.Background()
	srv := modelsdevtest.Server(t, []string{"anthropic", "openai", "google"})
	idx, _ := openRegistry(t, t.TempDir(), WithModelsURL(srv.URL))

	var wg sync.WaitGroup
	work := func(fn func() error) {
		wg.Go(func() {
			if err := fn(); err != nil {
				t.Errorf("concurrent op: %v", err)
			}
		})
	}

	for range 8 {
		work(func() error {
			_, err := idx.Agents.List(ctx, AgentQuery{Enrich: EnrichCount})
			return err
		})
		work(func() error {
			_, err := idx.Providers.List(ctx, ProviderQuery{})
			return err
		})
		work(func() error {
			_, err := idx.Models.List(ctx, ModelQuery{})
			return err
		})
	}
	work(func() error {
		_, err := idx.Refresh(ctx, TargetModels)
		return err
	})
	work(func() error {
		_, err := idx.Refresh(ctx, TargetCatalog)
		return err
	})
	wg.Wait()
}
