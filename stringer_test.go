package agentdex

import (
	"fmt"
	"testing"
)

// TestStringers pins each enum's String output to its constant identifier and the
// out-of-range fallback to the Stringer-style form, so a mistyped return in a
// switch case — which go vet does not catch — fails here. That a case exists for
// every constant is the exhaustive linter's job, enforced across every enum switch
// by .golangci.yml; this test covers what the linter cannot see, namely a case
// returning the wrong identifier.
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
		}, func(v int) string { return WarningKind(v).String() })
	})

	t.Run("Target", func(t *testing.T) {
		assertNames(t, "Target", []string{
			"TargetCatalog",
			"TargetModels",
			"TargetAll",
		}, func(v int) string { return Target(v).String() })
	})
}

// assertNames checks that each constant from zero upward stringifies to the
// identifier listed at its index, then that the first value past the list falls
// through to the Stringer-style fallback.
//
// That fallback probe is also the completeness check. A constant added to the enum
// stringifies to its own identifier rather than the fallback, so this fails until
// the identifier is listed in names — and listing it is where a case returning the
// wrong text is caught. The linter forces the case to exist; this forces its text
// to be stated.
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
