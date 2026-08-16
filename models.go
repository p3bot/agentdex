package agentdex

import (
	"context"
	"log/slog"
	"strings"

	"github.com/p3bot/agentdex/modelsdev"
)

// List browses models across the scoped providers, newest release first. Empty
// scope spans every models.dev provider. Each Model carries its provider and
// optional agnostic-map canonical id. Stale warnings ride the error path too.
func (s ModelService) List(ctx context.Context, q ModelQuery) (Result[Model], error) {
	c := s.core
	mc := c.modelsClient()

	providers, warnings, modelsConsulted, err := c.resolveModelScope(ctx, mc, q.Scope)
	if err != nil {
		return Result[Model]{Warnings: appendModelsStale(warnings, mc, modelsConsulted)}, err
	}

	items, err := c.modelsForProviders(ctx, mc, providers)
	modelsConsulted = true
	if err != nil {
		return Result[Model]{Warnings: appendModelsStale(warnings, mc, modelsConsulted)}, mapModelsErr(err)
	}
	if needle := strings.ToLower(q.Filter); needle != "" {
		filtered := make([]Model, 0, len(items))
		for _, m := range items {
			if matchesFilter(m.ID, m.Name, needle) {
				filtered = append(filtered, m)
			}
		}
		items = filtered
	}
	sortModels(items)
	return Result[Model]{Items: items, Warnings: appendModelsStale(warnings, mc, modelsConsulted)}, nil
}

// Get returns one model by composite provider-id/model-id. Splits on the first
// slash only (model key may contain slashes). No agent catalog, no warnings channel.
func (s ModelService) Get(ctx context.Context, composite string) (Model, error) {
	c := s.core
	pid, key, ok := strings.Cut(composite, "/")
	if !ok {
		return Model{}, errf(ErrMalformedModelID, "model id %q must be provider-id/model-id", composite)
	}

	mc := c.modelsClient()
	p, found, err := mc.Provider(ctx, pid)
	if err != nil {
		return Model{}, mapModelsErr(err)
	}
	if !found {
		return Model{}, errf(ErrNotFound, "no model %q: unknown provider %q", composite, pid)
	}
	m, ok := p.Models[key]
	if !ok {
		return Model{}, errf(ErrNotFound, "no model %q in provider %q", composite, pid)
	}

	agnostic, err := mc.Catalog(ctx)
	if err != nil {
		return Model{}, mapModelsErr(err)
	}
	canonical := canonicalID(agnostic, composite)
	c.logger.LogAttrs(ctx, slog.LevelDebug, "model resolved",
		slog.String("composite", composite), slog.String("provider", pid), slog.String("canonical", canonical))
	return Model{Model: m, Provider: pid, CanonicalID: canonical}, nil
}

// resolveModelScope enforces agnostic/home rules. Unknown id is ErrUnknownProvider;
// outage is not a rejection (listing fetch reports it). Bool is models.dev consulted.
func (c *core) resolveModelScope(ctx context.Context, mc *modelsdev.Client, scope ModelScope) ([]string, []Warning, bool, error) {
	caller := dedupeIDs(scope.Providers)

	if scope.Agent != "" {
		cat, info, err := c.resolveCatalog(ctx)
		if err != nil {
			return nil, nil, false, err
		}
		var warnings []Warning
		if info.Stale {
			warnings = append(warnings, staleWarning())
		}
		ka, ok := cat.Agents[scope.Agent]
		if !ok {
			return nil, warnings, false, errf(ErrAgentUnknown, "no agent %q", scope.Agent)
		}
		if ka.Agnostic {
			if len(caller) == 0 {
				return nil, warnings, false, errf(ErrProvidersRequired, "providers required for agnostic agent: %q is provider-agnostic", scope.Agent)
			}
			if verr := c.validateModelProviders(ctx, mc, caller); verr != nil {
				return nil, warnings, true, verr
			}
			c.logger.LogAttrs(ctx, slog.LevelDebug, "model scope resolved",
				slog.String("agent", scope.Agent), slog.Any("providers", caller))
			return caller, warnings, true, nil
		}
		set, rerr := restrictHomeProviders(scope.Agent, ka.Provider, caller)
		if rerr != nil {
			return nil, warnings, false, rerr
		}
		c.logger.LogAttrs(ctx, slog.LevelDebug, "model scope resolved",
			slog.String("agent", scope.Agent), slog.Any("providers", set))
		return set, warnings, false, nil
	}

	if len(caller) > 0 {
		if verr := c.validateModelProviders(ctx, mc, caller); verr != nil {
			return nil, nil, true, verr
		}
		c.logger.LogAttrs(ctx, slog.LevelDebug, "model scope resolved", slog.Any("providers", caller))
		return caller, nil, true, nil
	}

	cat, err := mc.Catalog(ctx)
	if err != nil {
		return nil, nil, true, mapModelsErr(err)
	}
	ids := sortedKeys(cat.Providers)
	c.logger.LogAttrs(ctx, slog.LevelDebug, "model scope resolved", slog.Int("providers", len(ids)))
	return ids, nil, true, nil
}

// Unknown id and schema drift reject; outage is non-rejection (listing fetch reports it).
func (c *core) validateModelProviders(ctx context.Context, mc *modelsdev.Client, ids []string) error {
	switch kind, err := c.validateProviders(ctx, mc, ids); kind {
	case provUnknown, provSchema:
		return err
	case provOK, provUnreachable:
	}
	return nil
}

// Unsorted; callers apply newest-first order. Unknown ids are skipped.
func (c *core) modelsForProviders(ctx context.Context, mc *modelsdev.Client, providers []string) ([]Model, error) {
	cat, err := mc.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	var out []Model
	for _, pid := range providers {
		p, found, err := mc.Provider(ctx, pid)
		if err != nil {
			return nil, err
		}
		if !found {
			c.logger.LogAttrs(ctx, slog.LevelDebug, "model provider absent from models.dev", slog.String("provider", pid))
			continue
		}
		for _, key := range sortedKeys(p.Models) {
			m := p.Models[key]
			composite := pid + "/" + key
			out = append(out, Model{
				Model:       m,
				Provider:    pid,
				CanonicalID: canonicalID(cat, composite),
			})
		}
	}
	return out, nil
}

func canonicalID(agnostic *modelsdev.Catalog, composite string) string {
	if _, ok := agnostic.Models[composite]; ok {
		return composite
	}
	return ""
}
