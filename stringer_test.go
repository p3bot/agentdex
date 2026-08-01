package agentdex

import (
	"fmt"
	"testing"
)

// Pins String() to each constant name and the out-of-range fallback. Exhaustive
// cases are the linter's job; this catches a case that returns the wrong identifier.
func TestStringers(t *testing.T) {
	t.Run("Enrich", func(t *testing.T) {
		assertNames(t, "Enrich", []string{
			"EnrichNone",
			"EnrichProviders",
			"EnrichCount",
			"EnrichFull",
		}, func(v int) string { return Enrich(v).String() })
	})

	t.Run("EnrichmentState", func(t *testing.T) {
		assertNames(t, "EnrichmentState", []string{
			"EnrichmentNotRequested",
			"EnrichmentApplied",
			"EnrichmentNotApplicable",
			"EnrichmentDegraded",
		}, func(v int) string { return EnrichmentState(v).String() })
	})

	t.Run("CoverageStatus", func(t *testing.T) {
		assertNames(t, "CoverageStatus", []string{
			"CoverageNotProbed",
			"CoverageAllPresent",
			"CoverageSomePresent",
			"CoverageNonePresent",
			"CoverageUnreachable",
			"CoverageSchemaDrift",
		}, func(v int) string { return CoverageStatus(v).String() })
	})

	t.Run("WarningKind", func(t *testing.T) {
		assertNames(t, "WarningKind", []string{
			"WarnStaleCatalog",
			"WarnModelsUnreachable",
			"WarnModelsSchemaDrift",
			"WarnSomeProvidersAbsent",
			"WarnNotInstalled",
			"WarnProvidersRequired",
			"WarnModelsStale",
		}, func(v int) string { return WarningKind(v).String() })
	})

	t.Run("Target", func(t *testing.T) {
		assertNames(t, "Target", []string{
			"TargetCatalog",
			"TargetModels",
			"TargetAll",
		}, func(v int) string { return Target(v).String() })
	})

	t.Run("CatalogSource", func(t *testing.T) {
		assertNames(t, "CatalogSource", []string{
			"CatalogSourceRegistry",
			"CatalogSourceDir",
		}, func(v int) string { return CatalogSource(v).String() })
	})
}

// Fallback probe is also completeness: a new constant fails until listed in names.
func assertNames(t *testing.T, typeName string, names []string, str func(int) string) {
	t.Helper()
	for value, want := range names {
		if got := str(value); got != want {
			t.Errorf("%s(%d).String() = %q, want %q", typeName, value, got, want)
		}
	}
	past := len(names)
	want := fmt.Sprintf("%s(%d)", typeName, past)
	if got := str(past); got != want {
		t.Errorf("%s(%d).String() = %q, want the out-of-range fallback %q; a constant was added, so list its identifier in the %s names",
			typeName, past, got, want, typeName)
	}
}
