package agentdex

import (
	"context"
	"log/slog"
	"strings"
)

// List browses models.dev providers by id order, optionally filtered. Loads no
// agent catalog. Stale fallback raises WarnModelsStale; outage is
// ErrModelsUnavailable; schema drift propagates modelsdev.ErrModelsSchema.
func (s ProviderService) List(ctx context.Context, q ProviderQuery) (Result[Provider], error) {
	c := s.core
	mc := c.modelsClient()
	cat, err := mc.Catalog(ctx)
	if err != nil {
		return Result[Provider]{}, mapModelsErr(err)
	}
	var warnings []Warning
	if mc.Stale() {
		warnings = append(warnings, modelsStaleWarning())
	}

	needle := strings.ToLower(q.Filter)
	ids := sortedKeys(cat.Providers)
	items := make([]Provider, 0, len(ids))
	for _, id := range ids {
		p := cat.Providers[id]
		if needle != "" && !matchesFilter(p.ID, p.Name, needle) {
			continue
		}
		items = append(items, Provider{Provider: p, EnvPresent: c.envPresence(p.Env)})
	}
	c.logger.LogAttrs(ctx, slog.LevelDebug, "providers listed", slog.Int("count", len(items)))
	return Result[Provider]{Items: items, Warnings: warnings}, nil
}

// Get returns one models.dev provider by id with API-key env presence.
// Unknown id is ErrNotFound. Loads no agent catalog (no warnings channel).
func (s ProviderService) Get(ctx context.Context, id string) (Provider, error) {
	c := s.core
	p, found, err := c.modelsClient().Provider(ctx, id)
	if err != nil {
		return Provider{}, mapModelsErr(err)
	}
	if !found {
		return Provider{}, errf(ErrNotFound, "no models.dev provider %q", id)
	}
	c.logger.LogAttrs(ctx, slog.LevelDebug, "provider resolved", slog.String("provider", id))
	return Provider{Provider: p, EnvPresent: c.envPresence(p.Env)}, nil
}

// Presence only, never the value. Non-nil even when env is empty (resolved fact,
// not the Agent ProviderEnv nil that means models.dev was not consulted).
func (c *core) envPresence(env []string) map[string]bool {
	present := make(map[string]bool, len(env))
	for _, name := range env {
		_, ok := c.envLookup(name)
		present[name] = ok
	}
	return present
}

func matchesFilter(id, name, needle string) bool {
	return strings.Contains(strings.ToLower(id), needle) || strings.Contains(strings.ToLower(name), needle)
}
