package agentdex

import "fmt"

// Enrich selects how much provider and models.dev data an agent operation
// attaches. Each level is a superset of the one below.
type Enrich int

const (
	// EnrichNone is catalog and detection only. Get never contacts models.dev; List
	// still validates a non-empty Providers filter against it at every level.
	EnrichNone Enrich = iota
	// EnrichProviders adds the resolved provider set only (offline for home-provider;
	// validates caller ids for agnostic).
	EnrichProviders
	// EnrichCount adds ProviderEnv and ModelCount (and coverage on Agents.Get).
	EnrichCount
	// EnrichFull adds the Models list on the same fetch as EnrichCount.
	EnrichFull
)

// String returns the constant name for known levels, or Enrich(n) for others.
func (e Enrich) String() string {
	switch e {
	case EnrichNone:
		return "EnrichNone"
	case EnrichProviders:
		return "EnrichProviders"
	case EnrichCount:
		return "EnrichCount"
	case EnrichFull:
		return "EnrichFull"
	default:
		return fmt.Sprintf("Enrich(%d)", int(e))
	}
}

// EnrichmentState records the outcome of enrichment on a returned Agent.
type EnrichmentState int

const (
	// EnrichmentNotRequested means Enrich was EnrichNone.
	EnrichmentNotRequested EnrichmentState = iota
	// EnrichmentApplied means the requested level was satisfied in full.
	EnrichmentApplied
	// EnrichmentNotApplicable is agnostic with no providers: outside facts only,
	// distinct from a real empty result.
	EnrichmentNotApplicable
	// EnrichmentDegraded means models.dev could not fill the level, so ModelCount is
	// not a true zero. Fault rides a List warning or Get coverage verdict.
	EnrichmentDegraded
)

// String returns the constant name for known states, or EnrichmentState(n) for others.
func (s EnrichmentState) String() string {
	switch s {
	case EnrichmentNotRequested:
		return "EnrichmentNotRequested"
	case EnrichmentApplied:
		return "EnrichmentApplied"
	case EnrichmentNotApplicable:
		return "EnrichmentNotApplicable"
	case EnrichmentDegraded:
		return "EnrichmentDegraded"
	default:
		return fmt.Sprintf("EnrichmentState(%d)", int(s))
	}
}

// CoverageStatus is the verdict of probing one agent's catalog provider set
// against models.dev. Zero is CoverageNotProbed; other values are probe results.
type CoverageStatus int

const (
	// CoverageNotProbed means no models.dev contact, so no verdict.
	CoverageNotProbed CoverageStatus = iota
	CoverageAllPresent
	CoverageSomePresent
	CoverageNonePresent
	CoverageUnreachable
	CoverageSchemaDrift
)

// String returns the constant name for known statuses, or CoverageStatus(n) for others.
func (s CoverageStatus) String() string {
	switch s {
	case CoverageNotProbed:
		return "CoverageNotProbed"
	case CoverageAllPresent:
		return "CoverageAllPresent"
	case CoverageSomePresent:
		return "CoverageSomePresent"
	case CoverageNonePresent:
		return "CoverageNonePresent"
	case CoverageUnreachable:
		return "CoverageUnreachable"
	case CoverageSchemaDrift:
		return "CoverageSchemaDrift"
	default:
		return fmt.Sprintf("CoverageStatus(%d)", int(s))
	}
}

// ProviderCoverage is per-provider models.dev coverage of one agent's set, as data.
type ProviderCoverage struct {
	Present []string
	Absent  []string
	Status  CoverageStatus
	// Err wraps the models.dev fault for Unreachable/SchemaDrift so errors.Is works.
	Err error
}

// WarningKind classifies a non-fatal condition. Same kind may carry different Msg
// wording per operation; branch on Kind, emit Msg verbatim.
type WarningKind int

const (
	WarnStaleCatalog WarningKind = iota
	WarnModelsUnreachable
	WarnModelsSchemaDrift
	WarnSomeProvidersAbsent
	WarnNotInstalled
	// WarnProvidersRequired is guidance: agnostic agent reported without providers.
	WarnProvidersRequired
	// WarnModelsStale: stale cache fallback after failed refetch; only when consulted.
	WarnModelsStale
	// WarnUnknownBinPath: a WithBinPaths key is not a catalog id.
	WarnUnknownBinPath
)

// String returns the constant name for known kinds, or WarningKind(n) for others.
func (k WarningKind) String() string {
	switch k {
	case WarnStaleCatalog:
		return "WarnStaleCatalog"
	case WarnModelsUnreachable:
		return "WarnModelsUnreachable"
	case WarnModelsSchemaDrift:
		return "WarnModelsSchemaDrift"
	case WarnSomeProvidersAbsent:
		return "WarnSomeProvidersAbsent"
	case WarnNotInstalled:
		return "WarnNotInstalled"
	case WarnProvidersRequired:
		return "WarnProvidersRequired"
	case WarnModelsStale:
		return "WarnModelsStale"
	case WarnUnknownBinPath:
		return "WarnUnknownBinPath"
	default:
		return fmt.Sprintf("WarningKind(%d)", int(k))
	}
}

// Warning is Kind for branching and Msg for verbatim emission.
type Warning struct {
	Kind WarningKind
	Msg  string
}
