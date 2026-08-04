package agentdex

import (
	"fmt"

	"github.com/p3bot/agentdex/modelsdev"
)

// Result is the symmetric return of every List: ordered items and warnings.
// Warnings are valid on the error return; read Items only when err is nil.
type Result[T any] struct {
	Items    []T
	Warnings []Warning
}

// AgentQuery narrows and enriches an Agents.List. Providers is the listing-wide
// set for agnostic rows, validated at the boundary.
type AgentQuery struct {
	Filter    string
	Installed bool
	Providers []string
	Enrich    Enrich
}

// AgentGetQuery selects enrichment level and the agnostic provider set for Agents.Get.
type AgentGetQuery struct {
	Providers []string
	Enrich    Enrich
}

// ProviderQuery narrows a Providers.List by case-insensitive id/name substring.
type ProviderQuery struct {
	Filter string
}

// ModelQuery scopes and narrows a Models.List.
type ModelQuery struct {
	Scope  ModelScope
	Filter string
}

// ModelScope selects the provider set a model listing spans. Providers is also
// the enrichment set for an agnostic Agent.
type ModelScope struct {
	Agent     string
	Providers []string
}

// KnownAgent is one catalog entry as identity and capability (no resolved paths).
// ID is the catalog map key. CatalogProviders is empty when Agnostic is true.
type KnownAgent struct {
	ID               string
	Name             string
	Bin              string
	Description      string
	Homepage         string
	CatalogProviders []string
	Agnostic         bool
}

// ResolvedPaths is a catalog directory pair after expansion, with existence per scope.
// Local is "" when the catalog defines no local scope.
type ResolvedPaths struct {
	Global       string
	GlobalExists bool
	Local        string
	LocalExists  bool
}

// PathEntry is one expanded catalog path with on-disk existence.
// Path is "" when that role is unsupported for the agent/scope.
type PathEntry struct {
	Path   string
	Exists bool
}

// SkillsScope is one scope's classified skill roots after expansion.
// Primary: agents else native else Alternatives[0]. Alternatives is priority order.
type SkillsScope struct {
	Agents       PathEntry
	Native       PathEntry
	Alternatives []PathEntry
	Primary      PathEntry
}

// SkillsPaths is resolved skills by scope. Zero means the agent has no skills concept.
type SkillsPaths struct {
	Global SkillsScope
	Local  SkillsScope
}

// Detection is what locating an agent found on this machine. Found gates only
// BinaryPath and Version; paths resolve the same whether or not the binary is installed.
type Detection struct {
	Found      bool
	BinaryPath string
	Version    string
	Config     ResolvedPaths
	Skills     SkillsPaths
}

// Agent is catalog facts joined with detection and, from EnrichProviders upward,
// the resolved provider set and models.dev data. ResolvedProviders is empty below
// EnrichProviders and when an agnostic agent has no set.
type Agent struct {
	KnownAgent
	Detection         Detection
	ResolvedProviders []string
	ProviderEnv       map[string]bool // API-key env -> present; nil when models.dev not consulted
	Enrichment        EnrichmentState
	ModelCount        int     // meaningful when Enrichment == EnrichmentApplied
	Models            []Model // EnrichFull only; newest release first
}

// AgentDetail is Agents.Get: Agent plus coverage verdict and this fetch's warnings.
type AgentDetail struct {
	Agent
	Coverage ProviderCoverage
	Warnings []Warning
}

// Provider is a models.dev provider with API-key env presence.
type Provider struct {
	modelsdev.Provider
	EnvPresent map[string]bool
}

// Model is a models.dev model with its provider and optional agnostic-catalog key.
// Every surface returns this type so a short id is never detached from its provider.
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

// CatalogInfo is the identity of the loaded agent catalog. A directory source
// has no version and is never stale.
type CatalogInfo struct {
	Source  CatalogSource
	Dir     string // set when Source is CatalogSourceDir
	Module  string // major-line path when Source is CatalogSourceRegistry
	Version string // resolved version when Source is CatalogSourceRegistry
	Stale   bool
}
