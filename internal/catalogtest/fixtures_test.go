package catalogtest_test

import (
	"testing"

	"github.com/p3bot/agentdex/internal/catalog"
	"github.com/p3bot/agentdex/internal/catalogtest"
)

// TestFixtureBinsMatchCatalogValid locks the single source of fixture binary
// names: every catalog-valid agent bin must equal FixtureBins, and every
// FixtureBins entry must appear in the fixture catalog.
func TestFixtureBinsMatchCatalogValid(t *testing.T) {
	cat, err := catalog.LoadDir(catalogtest.FixtureDir(t, "catalog-valid"))
	if err != nil {
		t.Fatalf("LoadDir catalog-valid: %v", err)
	}
	for id, want := range catalogtest.FixtureBins {
		ka, ok := cat.Agents[id]
		if !ok {
			t.Errorf("FixtureBins has %q, catalog-valid does not", id)
			continue
		}
		if ka.Bin != want {
			t.Errorf("agent %q bin = %q, want FixtureBins value %q", id, ka.Bin, want)
		}
	}
	for id := range cat.Agents {
		if _, ok := catalogtest.FixtureBins[id]; !ok {
			t.Errorf("catalog-valid has agent %q, FixtureBins does not", id)
		}
	}
}
