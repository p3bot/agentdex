// Package catalog fetches the agentdex agent catalog from the CUE Central
// Registry, validates it by evaluating the fetched module against its bundled
// schema, caches the resolved module version, and decodes the catalog into an
// internal representation. The root package agentdex maps each KnownAgent into
// its public types (KnownAgent, Detection, Agent) at detect time; this package
// never imports the root package, keeping the dependency one-way.
package catalog

// Catalog is the loaded set of known agents in this package's internal
// representation, keyed by catalog id.
type Catalog struct {
	Agents map[string]KnownAgent
}

// KnownAgent is one decoded catalog entry. ID is populated from the catalog map
// key by the loader; the schema's #KnownAgent has no id field. Agnostic agents
// carry no Provider list; home-provider agents always have at least one.
type KnownAgent struct {
	ID          string
	Name        string
	Bin         string
	Description string
	Config      PathPair
	Skills      *SkillsPaths
	Version     *VersionProbe
	Agnostic    bool
	Provider    []string
	Homepage    string
}

// PathPair is a catalog global/local directory pair before any expansion.
// Config uses this shape.
type PathPair struct {
	Global string
	Local  string
}

// SkillsPaths is catalog skills roots before expansion, split by scope.
// Primary is not stored; the library derives it after resolve.
type SkillsPaths struct {
	Global SkillsScope
	Local  SkillsScope
}

// SkillsScope is one scope's classified skill roots before expansion.
// Empty strings and a nil Alternatives mean that role is unsupported.
// Alternatives is priority order (first is primary when agents and native are unset).
type SkillsScope struct {
	Agents       string
	Native       string
	Alternatives []string
}

// VersionProbe describes how to read an agent's version from its binary.
type VersionProbe struct {
	Args    []string
	Pattern string
}
