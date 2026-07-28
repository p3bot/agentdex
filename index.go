package agentdex

import "context"

// Index is the entry point and facade returned by Open. It exposes the three noun
// services as fields and carries the cache-level operations. It is safe for
// concurrent use: the lazy catalog and models.dev resolution behind the services
// happens once under a guard, and Refresh publishes replacement state under the
// same guard (R12, R13).
type Index struct {
	Agents    AgentService
	Providers ProviderService
	Models    ModelService

	core *core
}

// Refresh forces re-resolution or refetch of the requested targets past their
// caches and publishes the refreshed state on the Index, so the operations a caller
// makes next serve the fresh data (R13). TargetAll runs its targets in order —
// catalog, then models.dev — and stops at the first failure, returning that target's
// error with Refreshed reporting only the targets that completed before it; a target
// the failure leaves unattempted is neither refreshed nor failed. A target that
// fails to refresh leaves its existing state untouched, so a failed refresh never
// costs a caller a working index. A catalog supplied by WithCatalogDir has no version
// to re-resolve, so its target is reported not-refreshed with no error.
func (x *Index) Refresh(ctx context.Context, t Target) (Refreshed, error) {
	var refreshed Refreshed
	if t == TargetCatalog || t == TargetAll {
		did, err := x.core.refreshCatalog(ctx)
		if err != nil {
			return refreshed, err
		}
		refreshed.Catalog = did
	}
	if t == TargetModels || t == TargetAll {
		if err := x.core.refreshModels(ctx); err != nil {
			return refreshed, err
		}
		refreshed.Models = true
	}
	return refreshed, nil
}

// CatalogInfo returns the identity of the loaded agent catalog: source, module
// path and version when registry-backed, directory when dir-backed, and whether
// the resolution is a stale fallback. It resolves the catalog lazily like any
// catalog-touching operation, so a cold-offline first call returns
// ErrCatalogUnavailable rather than an empty identity (R2, R12).
func (x *Index) CatalogInfo(ctx context.Context) (CatalogInfo, error) {
	_, info, err := x.core.resolveCatalog(ctx)
	if err != nil {
		return CatalogInfo{}, err
	}
	return info, nil
}

// CatalogStale reports whether the loaded agent catalog is a stale fallback: a
// re-resolution that failed after the TTL expired and reused the last resolved
// version. It is equivalent to CatalogInfo(ctx).Stale and shares its lazy-load
// and error behaviour. A catalog supplied by WithCatalogDir is never stale.
func (x *Index) CatalogStale(ctx context.Context) (bool, error) {
	info, err := x.CatalogInfo(ctx)
	if err != nil {
		return false, err
	}
	return info.Stale, nil
}

// ModelsStale reports whether the models.dev catalog currently served is a stale
// fallback: a fetch that failed after the TTL expired and reused the on-disk
// cache. It triggers the models.dev load lazily; a cold-offline first call with
// nothing cached returns ErrModelsUnavailable rather than a misleading false.
func (x *Index) ModelsStale(ctx context.Context) (bool, error) {
	mc := x.core.modelsClient()
	if _, err := mc.Catalog(ctx); err != nil {
		return false, mapModelsErr(err)
	}
	return mc.Stale(), nil
}

// AgentService browses and fetches agents joined with detection and enrichment.
type AgentService struct{ core *core }

// ProviderService browses and fetches models.dev providers.
type ProviderService struct{ core *core }

// ModelService browses and fetches models across models.dev providers.
type ModelService struct{ core *core }
