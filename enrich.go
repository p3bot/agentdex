package agentdex

import "fmt"

// Enrich selects how much provider and models.dev data an agent operation
// attaches. It is the single demand axis: each level is a superset of the one
// below, and what the operation resolves, reports, and validates against
// models.dev is keyed off it (R4).
type Enrich int

const (
	// EnrichNone is catalog and detection facts only: no provider resolution and no
	// enrichment of agent rows. Get never contacts models.dev. List is offline from
	// models.dev when Providers is empty; a non-empty Providers filter is still
	// validated against models.dev at every Enrich level, including this one.
	EnrichNone Enrich = iota
	// EnrichProviders adds the resolved provider set and nothing else. For a
	// home-provider agent that is offline catalog data; for an agnostic agent the
	// caller ids are validated against models.dev before they are reported.
	EnrichProviders
	// EnrichCount adds ProviderEnv and ModelCount (and coverage on Agents.Get). It
	// is models.dev-backed for every agent a provider set was resolved for.
	EnrichCount
	// EnrichFull adds the full Models list. It pays the same models.dev fetch as
	// EnrichCount; only the models list separates them.
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

// EnrichmentState records the outcome of enrichment on a returned Agent, replacing
// the nil/empty/null encodings a caller would otherwise decode by hand.
type EnrichmentState int

const (
	// EnrichmentNotRequested means Enrich was EnrichNone.
	EnrichmentNotRequested EnrichmentState = iota
	// EnrichmentApplied means the requested level was satisfied in full.
	EnrichmentApplied
	// EnrichmentNotApplicable means an agnostic agent was resolved with no providers
	// supplied: outside facts only, distinct from a real empty result.
	EnrichmentNotApplicable
	// EnrichmentDegraded means models.dev could not fill the level — unreachable and
	// uncached, or serving data agentdex cannot parse — so ModelCount is not a true
	// zero. The fault is said alongside, as a warning kind on List and as the
	// coverage verdict on Get.
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

// CoverageStatus is the verdict of probing a single agent's catalog provider set
// against models.dev. The zero value is CoverageNotProbed, so every other status
// is a positive verdict a probe actually established.
type CoverageStatus int

const (
	// CoverageNotProbed means the level reached no models.dev, so no verdict.
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

// ProviderCoverage is the per-provider models.dev coverage of one agent's catalog
// provider set, reported as data by Agents.Get. The caller maps verdicts to policy.
type ProviderCoverage struct {
	Present []string
	Absent  []string
	Status  CoverageStatus
	// Err is the models.dev fault behind CoverageUnreachable and CoverageSchemaDrift;
	// nil otherwise. It wraps the modelsdev error so errors.Is resolves the drift.
	Err error
}

// WarningKind classifies a non-fatal condition an operation raised. Kind and Msg
// are one-to-many: the same kind carries different wording from different
// operations, so a caller branches on Kind while emitting Msg verbatim (R6).
type WarningKind int

const (
	WarnStaleCatalog WarningKind = iota
	WarnModelsUnreachable
	WarnModelsSchemaDrift
	WarnSomeProvidersAbsent
	WarnNotInstalled
	// WarnProvidersRequired is guidance, not an error: an agnostic agent reported
	// without a provider set.
	WarnProvidersRequired
	// WarnModelsStale means models.dev was served from a stale cache fallback after
	// a failed refetch. Emitted only by operations that actually consulted models.dev.
	WarnModelsStale
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
	default:
		return fmt.Sprintf("WarningKind(%d)", int(k))
	}
}

// Warning is a structured, self-describing note: the kind a caller branches on and
// the human-readable message it emits verbatim (adding a remedy clause only where
// the remedy names the caller's own affordance).
type Warning struct {
	Kind WarningKind
	Msg  string
}
