package agentdex

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/p3bot/agentdex/internal/catalog"
)

// detect resolves outside facts only. Found gates binary path, never provider
// set or paths. Detection never executes the binary.
func (c *core) detect(ka catalog.KnownAgent) Agent {
	a := Agent{
		KnownAgent: KnownAgent{
			ID:               ka.ID,
			Name:             ka.Name,
			Bin:              ka.Bin,
			Description:      ka.Description,
			Homepage:         ka.Homepage,
			CatalogProviders: append([]string(nil), ka.Provider...),
			Agnostic:         ka.Agnostic,
		},
	}
	a.Detection.BinaryPath, a.Detection.Found = c.locateBinary(ka.ID, ka.Bin)
	a.Detection.Config = c.resolvePathPair(ka.Config)
	if ka.Skills != nil {
		a.Detection.Skills = c.resolveSkillsPaths(*ka.Skills)
	}
	return a
}

// locateBinary: binPaths override is sole candidate; else lookPath then search
// dirs. Every hit must be executable; non-executable lookPath falls through.
// Relative paths root at the captured working directory before the executable check.
func (c *core) locateBinary(id, bin string) (string, bool) {
	if override, ok := c.binPaths[id]; ok && override != "" {
		p := c.absPath(override)
		if isExecutable(p) {
			return p, true
		}
		return "", false
	}
	if p, err := c.lookPath(bin); err == nil {
		p = c.absPath(p)
		if isExecutable(p) {
			return p, true
		}
	}
	for _, dir := range c.searchDirs {
		candidate := c.absPath(filepath.Join(dir, bin))
		if isExecutable(candidate) {
			return candidate, true
		}
	}
	return "", false
}

// Empty local scope stays empty; relative local roots at the captured working dir.
func (c *core) resolvePathPair(pp catalog.PathPair) ResolvedPaths {
	rp := ResolvedPaths{Global: c.expandPath(pp.Global)}
	rp.GlobalExists = pathExists(rp.Global)
	if pp.Local != "" {
		local := c.expandPath(pp.Local)
		if !filepath.IsAbs(local) {
			local = filepath.Join(c.workingDir, local)
		}
		rp.Local = local
		rp.LocalExists = pathExists(rp.Local)
	}
	return rp
}

func (c *core) resolveSkillsPaths(sp catalog.SkillsPaths) SkillsPaths {
	return SkillsPaths{
		Global: c.resolveSkillsScope(sp.Global, false),
		Local:  c.resolveSkillsScope(sp.Local, true),
	}
}

func (c *core) resolveSkillsScope(sc catalog.SkillsScope, projectLocal bool) SkillsScope {
	out := SkillsScope{
		Agents: c.resolveSkillPath(sc.Agents, projectLocal),
		Native: c.resolveSkillPath(sc.Native, projectLocal),
	}
	if len(sc.Alternatives) > 0 {
		out.Alternatives = make([]PathEntry, 0, len(sc.Alternatives))
		for _, raw := range sc.Alternatives {
			out.Alternatives = append(out.Alternatives, c.resolveSkillPath(raw, projectLocal))
		}
	}
	out.Primary = skillsPrimary(out)
	return out
}

func (c *core) resolveSkillPath(raw string, projectLocal bool) PathEntry {
	if raw == "" {
		return PathEntry{}
	}
	p := c.expandPath(raw)
	if projectLocal && !filepath.IsAbs(p) {
		p = filepath.Join(c.workingDir, p)
	}
	return PathEntry{Path: p, Exists: pathExists(p)}
}

func skillsPrimary(sc SkillsScope) PathEntry {
	if sc.Agents.Path != "" {
		return sc.Agents
	}
	if sc.Native.Path != "" {
		return sc.Native
	}
	if len(sc.Alternatives) > 0 {
		return sc.Alternatives[0]
	}
	return PathEntry{}
}

// Env first ($XDG…), then leading tilde; both use the captured lookup/home.
// Unset vars become empty — no XDG home fallback (that is the loader's job).
func (c *core) expandPath(raw string) string {
	if raw == "" {
		return ""
	}
	expanded := os.Expand(raw, func(key string) string {
		v, _ := c.envLookup(key)
		return v
	})
	switch {
	case expanded == "~":
		return c.home
	case strings.HasPrefix(expanded, "~/"):
		return filepath.Join(c.home, expanded[len("~/"):])
	}
	return expanded
}

// Root relative paths at the captured working directory, same as local config.
func (c *core) absPath(p string) string {
	if p == "" {
		return p
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(c.workingDir, p)
}

func pathExists(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

func isExecutable(p string) bool {
	info, err := os.Stat(p)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}
