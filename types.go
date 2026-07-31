package agentdex

import (
	"fmt"

	"github.com/start-cli/agentdex/modelsdev"
)

// Result is the symmetric return of every List operation: the ordered items and
// any warnings the operation raised. Warnings are valid on the error return too,
// so a caller reads Warnings unconditionally and Items only when the error is nil.
type Result[T any] struct {
	Items    []T
	Warnings []Warning
}

// AgentQuery narrows and enriches an Agents.List. Filter is a case-insensitive
// substring over id and name; "" matches all. Installed narrows to agents whose
// binary is detected on this machine. Providers is the listing-wide enrichment
// set applied to provider-agnostic rows and validated at the boundary (R8). Enrich
// selects how much provider and models.dev data to attach (R4).
type AgentQuery struct {
	Filter    string
	Installed bool
	Providers []string
	Enrich    Enrich
}

// AgentGetQuery selects the enrichment level and the agnostic provider set for an
// Agents.Get.
type AgentGetQuery struct {
	Providers []string
	Enrich    Enrich
}

// ProviderQuery narrows a Providers.List by a case-insensitive substring over id
// and name.
type ProviderQuery struct {
	Filter string
}

// ModelQuery scopes and narrows a Models.List. Filter is a case-insensitive
// substring over model id and name.
type ModelQuery struct {
	Scope  ModelScope
	Filter string
}

// ModelScope selects the provider set a model listing spans. Agent scopes to a
// catalogued agent's providers ("" means not scoped by agent); Providers names
// explicit provider ids, and is also the enrichment set for an agnostic Agent.
type ModelScope struct {
	Agent     string
	Providers []string
}

// KnownAgent is one catalog entry slimmed to identity and capability: the static
// facts an agent is known by, with no resolved path or version. ID is the catalog
// map key, the single source of identity. CatalogProviders is the models.dev
// provider ids the catalog pins to this agent; empty when Agnostic is true.
type KnownAgent struct {
	ID               string
	Name             string
	Bin              string
	Description      string
	Homepage         string
	CatalogProviders []string
	Agnostic         bool
}

// ResolvedPaths is a catalog directory pair after tilde, environment, and
// working-directory expansion, with existence recorded per scope. Local is "" when
// the catalog defines no local scope. Used for config, which is a single pair.
type ResolvedPaths struct {
	Global       string
	GlobalExists bool
	Local        string
	LocalExists  bool
}

// PathEntry is one expanded catalog path with on-disk existence recorded.
// Path is "" when that role is unsupported for the agent/scope.
type PathEntry struct {
	Path   string
	Exists bool
}

// SkillsScope is one scope's (global or local) classified skill roots after
// expansion. Primary is derived: agents if set, else native if set, else
// Alternatives[0] if any, else empty. Alternatives is priority order.
type SkillsScope struct {
	Agents       PathEntry
	Native       PathEntry
	Alternatives []PathEntry
	Primary      PathEntry
}

// SkillsPaths is the resolved skills layout by scope. The zero value means the
// agent has no skills concept (catalog omits skills).
type SkillsPaths struct {
	Global SkillsScope
	Local  SkillsScope
}

// Detection is what locating an agent found on this machine: its binary, version,
// and the resolved config and skills paths. Found gates only BinaryPath and
// Version; paths and providers resolve identically whether or not the binary is
// installed (R4).
type Detection struct {
	Found      bool
	BinaryPath string
	Version    string
	Config     ResolvedPaths
	Skills     SkillsPaths
}

// Agent is the catalog's static facts joined with what detection found and, from
// EnrichProviders upward, the resolved provider set and models.dev data.
// ResolvedProviders is the provider id set this operation used for enrichment
// (catalog list for a non-agnostic agent, or the caller's set for an agnostic
// one). It is empty below EnrichProviders and when an agnostic agent has no set.
type Agent struct {
	KnownAgent
	Detection         Detection
	ResolvedProviders []string
	ProviderEnv       map[string]bool // API-key env var -> present; nil when models.dev was not consulted
	Enrichment        EnrichmentState
	ModelCount        int     // meaningful when Enrichment == EnrichmentApplied
	Models            []Model // populated when Enrich == EnrichFull; newest release first
}

// AgentDetail is the exact-fetch result: an Agent with the per-provider coverage
// verdict and the warnings this fetch raised (stale catalog, models.dev stale
// when enrichment consulted models.dev, not-installed, coverage degrade,
// agnostic guidance).
type AgentDetail struct {
	Agent
	Coverage ProviderCoverage
	Warnings []Warning
}

// Provider is a models.dev provider with the presence of each of its API-key
// environment variables.
type Provider struct {
	modelsdev.Provider
	EnvPresent map[string]bool
}

// Model is a models.dev model with the provider it was resolved within and its
// agnostic-catalog key when it has one, else "". Every library surface that
// returns models uses this type so a short id is never detached from its provider.
type Model struct {
	modelsdev.Model
	Provider    string `json:"provider"`
	CanonicalID string `json:"canonical_id,omitempty"`
}

// Target selects which caches a Refresh forces.
type Target int

const (
	TargetCatalog Target = iota
	TargetModels
	TargetAll
)

// String returns the constant name for known targets, or Target(n) for others.
func (t Target) String() string {
	switch t {
	case TargetCatalog:
		return "TargetCatalog"
	case TargetModels:
		return "TargetModels"
	case TargetAll:
		return "TargetAll"
	default:
		return fmt.Sprintf("Target(%d)", int(t))
	}
}

// Refreshed reports which targets a Refresh actually re-resolved or refetched.
type Refreshed struct {
	Catalog bool
	Models  bool
}

// CatalogSource identifies where the agent catalog was loaded from.
type CatalogSource int

const (
	// CatalogSourceRegistry is the CUE Central Registry (or CUE_REGISTRY).
	CatalogSourceRegistry CatalogSource = iota
	// CatalogSourceDir is a local CUE module directory from WithCatalogDir.
	CatalogSourceDir
)

// String returns the constant name for known sources, or CatalogSource(n) for others.
func (s CatalogSource) String() string {
	switch s {
	case CatalogSourceRegistry:
		return "CatalogSourceRegistry"
	case CatalogSourceDir:
		return "CatalogSourceDir"
	default:
		return fmt.Sprintf("CatalogSource(%d)", int(s))
	}
}

// CatalogInfo is the identity of the loaded agent catalog: where it came from,
// which module and version when registry-backed, and whether the resolution is a
// stale fallback. A directory source has no version and is never stale.
type CatalogInfo struct {
	Source  CatalogSource
	Dir     string // absolute or as configured; set when Source is CatalogSourceDir
	Module  string // major-line module path when Source is CatalogSourceRegistry
	Version string // resolved module version when Source is CatalogSourceRegistry
	Stale   bool
}
