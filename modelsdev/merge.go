package modelsdev

import (
	"fmt"
	"strings"
)

// merge enriches provider models with benchmarks and weights that upstream keeps
// only in the provider-agnostic map. Agnostic-first: decompose each path-style
// id and copy onto the matching provider model. Only real upstream ids are
// touched — no composite is minted and Model.ID is never rewritten. Aggregator
// keys (already path-bearing) have no agnostic id decomposing to them and get
// nothing. Mutates cat in place.
func merge(cat *Catalog) {
	for id, agnostic := range cat.Models {
		providerID, modelKey, ok := strings.Cut(id, "/")
		if !ok || providerID == "" || modelKey == "" {
			continue
		}
		provider, ok := cat.Providers[providerID]
		if !ok {
			continue
		}
		model, ok := provider.Models[modelKey]
		if !ok {
			continue
		}
		model.Benchmarks = agnostic.Benchmarks
		model.Weights = agnostic.Weights
		provider.Models[modelKey] = model
	}
}

// validateTopLevel is the gross-drift guard on every fetch: both top-level maps
// must be non-empty. A violation means a wholesale schema change would otherwise
// silently blank enrichment.
func validateTopLevel(cat *Catalog) error {
	if len(cat.Models) == 0 || len(cat.Providers) == 0 {
		return fmt.Errorf("top-level models or providers map empty: %w", ErrModelsSchema)
	}
	return nil
}

// validateProvider applies the per-model required-field check to one requested
// provider. A model is malformed when its id is empty — the only per-model field
// upstream guarantees. A zero limit is not malformed: media-generation models
// legitimately carry none. Scoped to requested providers so an unrelated
// provider's bad model cannot break enrichment.
func validateProvider(p Provider) error {
	for key, m := range p.Models {
		if m.ID == "" {
			return fmt.Errorf("provider %q model %q malformed: %w", p.ID, key, ErrModelsSchema)
		}
	}
	return nil
}
