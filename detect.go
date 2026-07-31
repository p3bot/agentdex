package agentdex

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/start-cli/agentdex/internal/catalog"
)

// maxConcurrentDetections bounds how many catalog entries are detected at once.
// Each detection may exec a binary for its version probe, so an unbounded fan-out
// would spawn one child process per entry. The work is I/O-bound (subprocess and
// filesystem waits), so a fixed cap above GOMAXPROCS keeps healthy parallelism
// while bounding process count as the catalog grows.
const maxConcurrentDetections = 16

// detect resolves one catalog entry's outside facts: its binary and version, and
// its config and skills paths, expanded against the boundary inputs captured at
// Open. It performs no enrichment and reads no models.dev — Found gates only the
// binary path and version, never the provider set or paths (R4).
func (c *core) detect(ctx context.Context, ka catalog.KnownAgent) Agent {
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
	if a.Detection.Found && ka.Version != nil {
		a.Detection.Version = probeVersion(ctx, a.Detection.BinaryPath, *ka.Version)
	}
	return a
}

// locateBinary resolves an agent's binary. An explicit per-agent override wins
// outright — it is the sole candidate, still verified to exist and be executable
// so Found reflects reality — otherwise PATH is searched via the boundary
// lookPath (default exec.LookPath), then the extra search dirs. Every candidate
// path, including a lookPath hit, must be executable before Found is true; a
// non-executable lookPath result falls through to search dirs the same way a
// failed PATH search does. Making the located path absolute uses the captured
// working directory, so BinaryPath and the local config and skills paths all
// root a relative value the same way.
func (c *core) locateBinary(id, bin string) (string, bool) {
	if override, ok := c.binPaths[id]; ok && override != "" {
		if isExecutable(override) {
			return c.absPath(override), true
		}
		return "", false
	}
	if p, err := c.lookPath(bin); err == nil && isExecutable(p) {
		return c.absPath(p), true
	}
	for _, dir := range c.searchDirs {
		candidate := filepath.Join(dir, bin)
		if isExecutable(candidate) {
			return c.absPath(candidate), true
		}
	}
	return "", false
}

// resolvePathPair expands a catalog config pair into resolved global and local
// paths with per-scope existence. Local is rooted at the captured working
// directory when it is not already absolute. An empty local scope stays empty
// and not-existing.
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

// resolveSkillsPaths expands catalog skills roles into path entries with
// per-path existence and derives Primary per scope (agents, else native, else
// first alternative). Local roots are joined to the captured working directory
// when not already absolute.
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

// resolveSkillPath expands one catalog skill path. projectLocal roots relative
// paths at the captured working directory; empty raw stays empty.
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

// skillsPrimary picks the install/query target for one scope: shared agents
// convention first, then native, then the first alternative.
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

// expandPath applies environment-variable expansion and then leading-tilde
// expansion to a catalog path. Env expansion runs first so "$XDG_CONFIG_HOME/agent"
// resolves; tilde second for the "~/..." form. Both draw on the boundary-captured
// lookup rather than the live process environment (R10): $VAR resolves each name
// through it and ~ resolves the captured home. An unset variable becomes empty —
// there is no XDG home fallback here, which is the loader's job for its own cache.
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

// absPath makes a located path absolute against the captured working directory,
// so a relative --bin-path or search-dir value roots the same way a relative local
// config path does (R10). An already-absolute path is returned cleaned.
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
