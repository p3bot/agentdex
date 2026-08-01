package agentdex

import "context"

// Index is the entry point and facade returned by Open. Safe for concurrent use:
// lazy catalog/models.dev resolution and Refresh publish under guards.
type Index struct {
	Agents    AgentService
	Providers ProviderService
	Models    ModelService

	core *core
}

// Refresh forces re-resolution or refetch past caches and publishes the result.
// TargetAll runs catalog then models.dev and stops at the first failure;
// Refreshed names only targets that completed. Failed targets leave prior state
// untouched. WithCatalogDir has nothing to re-resolve (not-refreshed, no error).
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

// CatalogInfo returns the loaded agent catalog's identity. Lazy like other
// catalog ops: cold-offline first call is ErrCatalogUnavailable, not empty.
func (x *Index) CatalogInfo(ctx context.Context) (CatalogInfo, error) {
	_, info, err := x.core.resolveCatalog(ctx)
	if err != nil {
		return CatalogInfo{}, err
	}
	return info, nil
}

// CatalogStale is CatalogInfo(ctx).Stale. WithCatalogDir is never stale.
func (x *Index) CatalogStale(ctx context.Context) (bool, error) {
	info, err := x.CatalogInfo(ctx)
	if err != nil {
		return false, err
	}
	return info.Stale, nil
}

// ModelsStale reports a models.dev stale-cache fallback. Lazy load; cold-offline
// with nothing cached is ErrModelsUnavailable, not a misleading false.
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
