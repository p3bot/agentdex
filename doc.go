// Package agentdex indexes three kinds of data and serves them through one
// coherent surface: the AI coding agents in a published catalog, the models.dev
// providers that power them, and the models those providers offer. It owns the
// outside of an agent — identity, location, paths, capability — and never
// reads an agent's internal configuration or executes its binary.
//
// Open constructs an *Index with no network I/O; catalogs resolve lazily once under
// a guard. Options configure catalog source (registry module or local WithCatalogDir),
// caches, detection, and boundary inputs (env lookup, WithLookPath, working dir).
//
//	idx, err := agentdex.Open()
//	if err != nil { return err }
//	res, err := idx.Agents.List(ctx, agentdex.AgentQuery{Enrich: agentdex.EnrichCount})
//
// Index exposes Agents, Providers, and Models, each with List and Get, plus Refresh
// and catalog/models staleness helpers. Detection is a property of an agent, not a
// top-level verb.
//
// Enrich is the demand axis for agent operations (each level a superset):
//
//   - EnrichNone: catalog and detection only. Get never contacts models.dev; List
//     does so only to validate a non-empty Providers filter.
//   - EnrichProviders: resolved provider set (offline for home-provider; validates
//     caller ids for agnostic).
//   - EnrichCount: ProviderEnv, ModelCount, and coverage on Agents.Get.
//   - EnrichFull: full Models list on the same fetch as EnrichCount.
//
// Installation does not gate enrichment. EnrichmentState records applied,
// not-requested, not-applicable (agnostic with no providers), or degraded.
//
// Warnings carry Kind (branch) and Msg (emit verbatim) and are valid on the error
// return. Match errors with errors.Is against this package's sentinels;
// ErrModelsSchema aliases modelsdev.ErrModelsSchema. Adding an agent is a catalog
// edit: one generic detection path walks every entry.
package agentdex
