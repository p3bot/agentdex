package catalog

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// cueAgent mirrors #KnownAgent for decoding. Optional fields are pointers or
// omitempty; CUE honours the json tags.
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

type cueSkillsScope struct {
	Agents       string   `json:"agents,omitempty"`
	Native       string   `json:"native,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
}

// loadCatalogModule loads the CUE catalog module at sourceDir, validates by
// evaluation against the bundled schema (no schema of its own), and decodes
// with each agent's ID set from its map key. SkipImports keeps the registry out;
// stdlib packages such as struct remain available.
func loadCatalogModule(sourceDir string) (*Catalog, error) {
	// Package unset: load resolves the module root's package by unique context
	// so a fork selected via module-path override stays loadable.
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
	// Concrete validation surfaces constraint violations and missing required fields.
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
