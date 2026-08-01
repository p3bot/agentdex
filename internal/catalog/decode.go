package catalog

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// cueAgent mirrors the #KnownAgent schema for decoding. Optional schema fields
// are pointers (skills, version) or omitempty scalars; CUE honours the json
// tags when decoding.
type cueAgent struct {
	Name        string `json:"name"`
	Bin         string `json:"bin"`
	Description string `json:"description,omitempty"`
	Config      struct {
		Global string `json:"global"`
		Local  string `json:"local,omitempty"`
	} `json:"config"`
	Skills *struct {
		Global *cueSkillsScope `json:"global,omitempty"`
		Local  *cueSkillsScope `json:"local,omitempty"`
	} `json:"skills,omitempty"`
	Version *struct {
		Args    []string `json:"args"`
		Pattern string   `json:"pattern,omitempty"`
	} `json:"version,omitempty"`
	Agnostic bool     `json:"agnostic,omitempty"`
	Provider []string `json:"provider,omitempty"`
	Homepage string   `json:"homepage,omitempty"`
}

// cueSkillsScope mirrors #SkillsScope for decoding.
type cueSkillsScope struct {
	Agents       string   `json:"agents,omitempty"`
	Native       string   `json:"native,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
}

// loadCatalogModule loads the CUE catalog module rooted at sourceDir, validates
// the agents map against the schema that travels with it, and decodes it into
// the internal representation with each agent's ID set from its map key.
//
// Validation is by evaluation: the module bundles schema.cue, whose
// `agents: [...]: #KnownAgent` constraint is unified with the data when the
// package is built, so any contract violation surfaces here before decode. The
// loader carries no schema of its own. SkipImports keeps the registry out of
// this step; stdlib packages such as struct remain available.
func loadCatalogModule(sourceDir string) (*Catalog, error) {
	// Package is left unset so load resolves the module root's single package by
	// its unique context, rather than assuming a name; this keeps a fork selected
	// via the module-path override loadable whatever it names its package.
	cfg := &load.Config{
		Dir:         sourceDir,
		SkipImports: true,
	}
	insts := load.Instances([]string{"."}, cfg)
	if len(insts) != 1 {
		return nil, fmt.Errorf("%w: expected one instance, got %d", ErrInvalidCatalog, len(insts))
	}
	inst := insts[0]
	if inst.Err != nil {
		return nil, fmt.Errorf("%w: load: %w", ErrInvalidCatalog, inst.Err)
	}

	ctx := cuecontext.New()
	val := ctx.BuildInstance(inst)
	if err := val.Err(); err != nil {
		return nil, fmt.Errorf("%w: build: %w", ErrInvalidCatalog, err)
	}

	agentsVal := val.LookupPath(cue.ParsePath("agents"))
	if err := agentsVal.Err(); err != nil {
		return nil, fmt.Errorf("%w: no agents field: %w", ErrInvalidCatalog, err)
	}
	// Concrete validation surfaces both constraint violations (a bottom from,
	// e.g., name failing !="" or MinFields) and missing required fields.
	if err := agentsVal.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCatalog, err)
	}

	var decoded map[string]cueAgent
	if err := agentsVal.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("%w: decode: %w", ErrInvalidCatalog, err)
	}

	agents := make(map[string]KnownAgent, len(decoded))
	for id, a := range decoded {
		ka := KnownAgent{
			ID:          id,
			Name:        a.Name,
			Bin:         a.Bin,
			Description: a.Description,
			Config:      PathPair{Global: a.Config.Global, Local: a.Config.Local},
			Agnostic:    a.Agnostic,
			Provider:    a.Provider,
			Homepage:    a.Homepage,
		}
		if a.Skills != nil {
			ka.Skills = mapSkills(a.Skills.Global, a.Skills.Local)
		}
		if a.Version != nil {
			ka.Version = &VersionProbe{Args: a.Version.Args, Pattern: a.Version.Pattern}
		}
		agents[id] = ka
	}
	return &Catalog{Agents: agents}, nil
}

// mapSkills builds SkillsPaths from a decoded skills object. Emptiness is
// already rejected by schema MinFields during evaluation.
func mapSkills(global, local *cueSkillsScope) *SkillsPaths {
	sk := &SkillsPaths{}
	if global != nil {
		sk.Global = mapSkillsScope(global)
	}
	if local != nil {
		sk.Local = mapSkillsScope(local)
	}
	return sk
}

func mapSkillsScope(s *cueSkillsScope) SkillsScope {
	return SkillsScope{
		Agents:       s.Agents,
		Native:       s.Native,
		Alternatives: append([]string(nil), s.Alternatives...),
	}
}
