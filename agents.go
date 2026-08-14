package agentdex

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/p3bot/agentdex/internal/catalog"
	"github.com/p3bot/agentdex/modelsdev"
)

// Get returns detection detail for one agent, selected exactly by its catalog id.
// Coverage verdicts and not-installed or agnostic-without-providers are data plus
// warnings, never errors; warnings also ride the error return.
func (s AgentService) Get(ctx context.Context, id string, q AgentGetQuery) (AgentDetail, error) {
	c := s.core
	cat, info, err := c.resolveCatalog(ctx)
	if err != nil {
		return AgentDetail{}, err
	}
	var warnings []Warning
	if info.Stale {
		warnings = append(warnings, staleWarning())
	}

	ka, ok := cat.Agents[id]
	if !ok {
		return AgentDetail{Warnings: warnings}, errf(ErrAgentUnknown, "no agent %q", id)
	}

	caller := dedupeIDs(q.Providers)
	// Rejected at every level: the set contradicts catalog data already in hand.
	if !ka.Agnostic && len(caller) > 0 {
		return AgentDetail{Warnings: warnings}, errf(ErrProvidersNotAllowed, "agent %q has catalog providers", id)
	}

	detail := AgentDetail{Agent: c.detect(ka)}
	if !detail.Detection.Found {
		warnings = append(warnings, notInstalledWarning(id))
	}

	if q.Enrich == EnrichNone {
		detail.Enrichment = EnrichmentNotRequested
		detail.Warnings = warnings
		return detail, nil
	}

	// Catalog alone: not-applicable, guidance warning, no models.dev.
	if ka.Agnostic && len(caller) == 0 {
		detail.Enrichment = EnrichmentNotApplicable
		warnings = append(warnings, providersRequiredWarning(id))
		detail.Warnings = warnings
		return detail, nil
	}

	// Copy public slices; never alias the memoised catalog's Provider array.
	var providers []string
	if ka.Agnostic {
		providers = caller
	} else {
		providers = append([]string(nil), detail.CatalogProviders...)
	}
	detail.ResolvedProviders = providers

	mc := c.modelsClient()
	// Gate on real mc calls, not merely obtaining the client.
	modelsConsulted := false

	// Unknown id rejects; drift or outage degrades rather than rejects.
	validation := provOK
	var validationErr error
	if ka.Agnostic {
		validation, validationErr = c.validateProviders(ctx, mc, providers)
		modelsConsulted = true
		if validation == provUnknown {
			detail.Warnings = appendModelsStale(warnings, mc, modelsConsulted)
			return detail, validationErr
		}
	}

	if q.Enrich == EnrichProviders {
		// Ids still reported; degrade means unchecked, so the fault rides alongside.
		switch validation {
		case provUnreachable:
			detail.Enrichment = EnrichmentDegraded
			warnings = append(warnings, providersUnvalidatedWarning(degradeUnreachable, validationErr))
		case provSchema:
			detail.Enrichment = EnrichmentDegraded
			warnings = append(warnings, providersUnvalidatedWarning(degradeSchema, validationErr))
		case provOK:
			detail.Enrichment = EnrichmentApplied
		case provUnknown:
			// Mirrors the early return so untrusted ids cannot claim EnrichmentApplied
			// if that path is ever narrowed.
			detail.Warnings = appendModelsStale(warnings, mc, modelsConsulted)
			return detail, validationErr
		}
		detail.Warnings = appendModelsStale(warnings, mc, modelsConsulted)
		return detail, nil
	}

	cov := c.probeCoverage(ctx, mc, providers)
	modelsConsulted = true
	detail.Coverage = cov.cov
	detail.Enrichment = cov.state
	if cov.state == EnrichmentDegraded {
		// Coverage.Err still carries the models.dev fault for errors.Is.
		switch cov.cov.Status {
		case CoverageUnreachable:
			warnings = append(warnings, modelsUnreachableGetWarning())
		case CoverageSchemaDrift:
			warnings = append(warnings, modelsSchemaDriftGetWarning(cov.cov.Err))
		case CoverageNotProbed, CoverageAllPresent, CoverageSomePresent, CoverageNonePresent:
		}
		detail.Warnings = appendModelsStale(warnings, mc, modelsConsulted)
		return detail, nil
	}
	detail.ProviderEnv = cov.providerEnv
	detail.ModelCount = len(cov.models)
	if q.Enrich == EnrichFull {
		sortModels(cov.models)
		detail.Models = cov.models
	}
	if cov.cov.Status == CoverageSomePresent {
		warnings = append(warnings, someProvidersAbsentWarning(cov.cov.Absent))
	}
	detail.Warnings = appendModelsStale(warnings, mc, modelsConsulted)
	return detail, nil
}

// List browses the catalog with local detection and, from EnrichProviders upward,
// the resolved provider set and models.dev enrichment. No per-agent coverage is
// probed. Providers is validated once at the boundary at every level; an unknown
// id fails the whole listing.
func (s AgentService) List(ctx context.Context, q AgentQuery) (Result[Agent], error) {
	c := s.core
	cat, info, err := c.resolveCatalog(ctx)
	if err != nil {
		return Result[Agent]{}, err
	}
	var warnings []Warning
	if info.Stale {
		warnings = append(warnings, staleWarning())
	}

	providers := dedupeIDs(q.Providers)
	needModels := q.Enrich >= EnrichCount

	var mc *modelsdev.Client
	if needModels || len(providers) > 0 {
		mc = c.modelsClient()
	}
	// True only after a real mc call on this listing — not a prior Index load.
	modelsConsulted := false

	degrade := degradeNone
	var degradeErr error

	// Boundary validation every level: unknown id fails even if the result is empty.
	if len(providers) > 0 {
		kind, verr := c.validateProviders(ctx, mc, providers)
		modelsConsulted = true
		switch kind {
		case provUnknown:
			return Result[Agent]{Warnings: appendModelsStale(warnings, mc, modelsConsulted)}, verr
		case provSchema:
			degrade, degradeErr = degradeSchema, verr
		case provUnreachable:
			degrade, degradeErr = degradeUnreachable, verr
		case provOK:
		}
	}

	// Only non-schema outage is decided here; per-model drift surfaces in enrichment.
	if needModels && degrade == degradeNone {
		if _, cerr := mc.Catalog(ctx); cerr != nil && !errors.Is(cerr, modelsdev.ErrModelsSchema) {
			degrade, degradeErr = degradeUnreachable, cerr
		}
		modelsConsulted = true
	}

	agents, err := c.detectAll(ctx, cat)
	if err != nil {
		return Result[Agent]{Warnings: appendModelsStale(warnings, mc, modelsConsulted)}, err
	}
	if q.Installed {
		agents = keepInstalled(agents)
	}

	// Exhaustive on Enrich so a new level cannot join the model path silently.
	switch q.Enrich {
	case EnrichNone:
		for i := range agents {
			agents[i].Enrichment = EnrichmentNotRequested
		}
	case EnrichProviders:
		for i := range agents {
			c.resolveListProviders(&agents[i], providers, degrade)
		}
		// Fault is listing-wide; say it once rather than per row.
		if degrade != degradeNone {
			warnings = append(warnings, providersUnvalidatedWarning(degrade, degradeErr))
		}
	case EnrichCount, EnrichFull:
		if degrade != degradeNone {
			for i := range agents {
				c.degradeListAgent(&agents[i], providers)
			}
			warnings = append(warnings, listDegradeWarning(degrade, degradeErr))
			break
		}
		wantModels := q.Enrich == EnrichFull
		for i := range agents {
			if serr := c.enrichListAgent(ctx, mc, &agents[i], providers, wantModels); serr != nil && degrade == degradeNone {
				if errors.Is(serr, modelsdev.ErrModelsSchema) {
					degrade, degradeErr = degradeSchema, serr
				} else {
					degrade, degradeErr = degradeUnreachable, serr
				}
			}
		}
		if degrade != degradeNone {
			for i := range agents {
				c.degradeListAgent(&agents[i], providers)
			}
			warnings = append(warnings, listDegradeWarning(degrade, degradeErr))
		}
	}

	if q.Filter != "" {
		agents = filterAgents(agents, q.Filter)
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].ID < agents[j].ID })
	return Result[Agent]{Items: agents, Warnings: appendModelsStale(warnings, mc, modelsConsulted)}, nil
}

type degradeMode int

const (
	degradeNone degradeMode = iota
	degradeUnreachable
	degradeSchema
)

type provValidation int

const (
	provOK provValidation = iota
	provUnknown
	provUnreachable
	provSchema
)

// validateProviders treats an absent id as ErrUnknownProvider; drift or outage is
// a degrade condition, not a rejection.
func (c *core) validateProviders(ctx context.Context, mc *modelsdev.Client, ids []string) (provValidation, error) {
	for _, pid := range ids {
		_, found, err := mc.Provider(ctx, pid)
		switch {
		case errors.Is(err, modelsdev.ErrModelsSchema):
			return provSchema, err
		case err != nil:
			return provUnreachable, err
		case !found:
			return provUnknown, errf(ErrUnknownProvider, "unknown provider id: %q", pid)
		}
	}
	return provOK, nil
}

type covResult struct {
	cov         ProviderCoverage
	providerEnv map[string]bool
	models      []Model
	state       EnrichmentState
}

// probeCoverage short-circuits drift/outage to EnrichmentDegraded. No empty-set
// case: home-provider sets are non-empty by schema, and agnostic empty never reaches here.
func (c *core) probeCoverage(ctx context.Context, mc *modelsdev.Client, providers []string) covResult {
	var present, absent []string
	var providerEnv map[string]bool
	for _, pid := range providers {
		p, found, err := mc.Provider(ctx, pid)
		switch {
		case errors.Is(err, modelsdev.ErrModelsSchema):
			return covResult{cov: ProviderCoverage{Status: CoverageSchemaDrift, Err: err}, state: EnrichmentDegraded}
		case err != nil:
			return covResult{cov: ProviderCoverage{Status: CoverageUnreachable, Err: err}, state: EnrichmentDegraded}
		case !found:
			absent = append(absent, pid)
		default:
			present = append(present, pid)
			for _, env := range p.Env {
				if providerEnv == nil {
					providerEnv = map[string]bool{}
				}
				_, ok := c.envLookup(env)
				providerEnv[env] = ok
			}
		}
	}

	var models []Model
	if len(present) > 0 {
		m, merr := c.modelsForProviders(ctx, mc, present)
		if merr != nil {
			// Present ids already passed the per-provider check; this fault is genuine.
			if errors.Is(merr, modelsdev.ErrModelsSchema) {
				return covResult{cov: ProviderCoverage{Status: CoverageSchemaDrift, Err: merr}, state: EnrichmentDegraded}
			}
			return covResult{cov: ProviderCoverage{Status: CoverageUnreachable, Err: merr}, state: EnrichmentDegraded}
		}
		models = m
	}

	status := CoverageAllPresent
	switch {
	case len(present) == 0:
		status = CoverageNonePresent
	case len(absent) > 0:
		status = CoverageSomePresent
	}
	return covResult{
		cov:         ProviderCoverage{Present: present, Absent: absent, Status: status},
		providerEnv: providerEnv,
		models:      models,
		state:       EnrichmentApplied,
	}
}

// detectAll locates every catalogued agent. Only a cancelled or expired context
// is returned; filesystem misses are Found=false, not errors.
func (c *core) detectAll(ctx context.Context, cat *catalog.Catalog) ([]Agent, error) {
	agents := make([]Agent, 0, len(cat.Agents))
	for _, ka := range cat.Agents {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		agents = append(agents, c.detect(ka))
	}
	return agents, nil
}

// Catalog list offline for home-provider; listing-wide set or not-applicable for agnostic.
func (c *core) resolveListProviders(a *Agent, listProviders []string, degrade degradeMode) {
	if a.Agnostic && len(listProviders) == 0 {
		a.Enrichment = EnrichmentNotApplicable
		return
	}
	if a.Agnostic {
		a.ResolvedProviders = append([]string(nil), listProviders...)
		if degrade == degradeNone {
			a.Enrichment = EnrichmentApplied
		} else {
			a.Enrichment = EnrichmentDegraded
		}
		return
	}
	a.ResolvedProviders = append([]string(nil), a.CatalogProviders...)
	a.Enrichment = EnrichmentApplied
}

// enrichListAgent skips absent providers rather than reporting coverage; any
// models.dev fault is returned for the caller to degrade the whole listing.
func (c *core) enrichListAgent(ctx context.Context, mc *modelsdev.Client, a *Agent, listProviders []string, wantModels bool) error {
	if a.Agnostic && len(listProviders) == 0 {
		a.Enrichment = EnrichmentNotApplicable
		return nil
	}
	var set []string
	if a.Agnostic {
		set = append([]string(nil), listProviders...)
	} else {
		set = append([]string(nil), a.CatalogProviders...)
	}
	a.ResolvedProviders = set

	var providerEnv map[string]bool
	for _, pid := range set {
		p, found, err := mc.Provider(ctx, pid)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		for _, env := range p.Env {
			if providerEnv == nil {
				providerEnv = map[string]bool{}
			}
			_, ok := c.envLookup(env)
			providerEnv[env] = ok
		}
	}
	models, err := c.modelsForProviders(ctx, mc, set)
	if err != nil {
		return err
	}
	a.ProviderEnv = providerEnv
	a.ModelCount = len(models)
	if wantModels {
		sortModels(models)
		a.Models = models
	}
	a.Enrichment = EnrichmentApplied
	return nil
}

// degradeListAgent keeps the resolved provider set but clears count/env/models.
// Agnostic with no set is not-applicable rather than degraded.
func (c *core) degradeListAgent(a *Agent, listProviders []string) {
	if a.Agnostic && len(listProviders) == 0 {
		a.Enrichment = EnrichmentNotApplicable
		return
	}
	if a.Agnostic {
		a.ResolvedProviders = append([]string(nil), listProviders...)
	} else {
		a.ResolvedProviders = append([]string(nil), a.CatalogProviders...)
	}
	a.Enrichment = EnrichmentDegraded
	a.ModelCount = 0
	a.ProviderEnv = nil
	a.Models = nil
}

func keepInstalled(agents []Agent) []Agent {
	out := make([]Agent, 0, len(agents))
	for _, a := range agents {
		if a.Detection.Found {
			out = append(out, a)
		}
	}
	return out
}

func filterAgents(agents []Agent, filter string) []Agent {
	needle := strings.ToLower(filter)
	out := make([]Agent, 0, len(agents))
	for _, a := range agents {
		if matchesFilter(a.ID, a.Name, needle) {
			out = append(out, a)
		}
	}
	return out
}

// Shared wording so catalog-stale messages never drift between surfaces.
func staleWarning() Warning {
	return Warning{Kind: WarnStaleCatalog, Msg: "agentdex catalog is stale: re-resolution failed, using the last resolved version"}
}

// Shared wording so models.dev-stale messages never drift between surfaces.
func modelsStaleWarning() Warning {
	return Warning{Kind: WarnModelsStale, Msg: "models.dev catalog is stale: refetch failed, using the cached copy"}
}

// appendModelsStale only when this operation actually called mc; a prior shared
// client load must not warn a path that never reached models.dev.
func appendModelsStale(ws []Warning, mc *modelsdev.Client, consulted bool) []Warning {
	if !consulted || mc == nil || !mc.Stale() {
		return ws
	}
	return append(ws, modelsStaleWarning())
}

func notInstalledWarning(id string) Warning {
	return Warning{Kind: WarnNotInstalled, Msg: fmt.Sprintf("agent %q is catalogued but not installed", id)}
}

func providersRequiredWarning(id string) Warning {
	return Warning{Kind: WarnProvidersRequired, Msg: fmt.Sprintf("%q is provider-agnostic", id)}
}

func modelsUnreachableGetWarning() Warning {
	return Warning{Kind: WarnModelsUnreachable, Msg: "models.dev is unreachable and not cached: model enrichment and provider-env omitted"}
}

// EnrichCount/Full wording; EnrichProviders uses providersUnvalidatedWarning.
func modelsSchemaDriftGetWarning(cause error) Warning {
	return Warning{Kind: WarnModelsSchemaDrift, Msg: fmt.Sprintf("models.dev schema unrecognised: model enrichment omitted: %v", cause)}
}

func someProvidersAbsentWarning(absent []string) Warning {
	return Warning{Kind: WarnSomeProvidersAbsent, Msg: fmt.Sprintf("some providers are absent from models.dev: %s", strings.Join(absent, ", "))}
}

// EnrichProviders shortfall: unvalidated ids (no model data at this level).
func providersUnvalidatedWarning(mode degradeMode, cause error) Warning {
	if mode == degradeSchema {
		return Warning{Kind: WarnModelsSchemaDrift, Msg: fmt.Sprintf("provider ids unvalidated: %v", cause)}
	}
	return Warning{Kind: WarnModelsUnreachable, Msg: "provider ids unvalidated: models.dev is unreachable and not cached"}
}

// List-only count-focused wording; Get uses the *GetWarning helpers.
func listDegradeWarning(mode degradeMode, cause error) Warning {
	if mode == degradeSchema {
		return Warning{Kind: WarnModelsSchemaDrift, Msg: fmt.Sprintf("model counts omitted: %v", cause)}
	}
	return Warning{Kind: WarnModelsUnreachable, Msg: "model counts unavailable: models.dev is unreachable and not cached"}
}
