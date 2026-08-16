package agentdex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/p3bot/agentdex/internal/catalogtest"
	"github.com/p3bot/agentdex/internal/modelsdevtest"
	"github.com/p3bot/agentdex/modelsdev"
)

// From catalogtest.FixtureBins — single table shared with the CLI harness.
var (
	fixtureBinAlpha = catalogtest.FixtureBins["alpha-cli"]
	fixtureBinBeta  = catalogtest.FixtureBins["beta-tool"]
	fixtureBinGamma = catalogtest.FixtureBins["gamma-agent"]
	fixtureBinDelta = catalogtest.FixtureBins["delta-agent"]
)

// Home-provider (alpha), multi-provider home (gamma), and agnostic (delta).
var testCatalog = fmt.Sprintf(`
agents: "alpha-cli": {
	name: "Alpha CLI"
	bin:  %q
	config: {global: "~/.alpha", local: ".alpha"}
	skills: {
		global: {native: "~/.alpha/skills"}
		local:  {native: ".alpha/skills"}
	}
	provider: ["anthropic"]
	homepage: "https://example.com/alpha"
}
agents: "gamma-agent": {
	name: "Gamma Agent"
	bin:  %q
	config: {global: "~/.gamma"}
	provider: ["google", "openai"]
}
agents: "delta-agent": {
	name: "Delta Agent"
	bin:  %q
	config: {global: "~/.delta"}
	agnostic: true
}
`, fixtureBinAlpha, fixtureBinGamma, fixtureBinDelta)

// So library tests cannot pick up host tools via PATH.
func closedLookPath(string) (string, error) {
	return "", exec.ErrNotFound
}

func openAgents(t *testing.T, body string, opts ...Option) *Index {
	t.Helper()
	dir := catalogtest.WriteModule(t, body)
	base := []Option{
		WithCatalogDir(dir),
		WithCacheDir(t.TempDir()),
		WithLookPath(closedLookPath),
	}
	idx, err := Open(append(base, opts...)...)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return idx
}

// Executable fakes for detection. Empty on purpose so a stray exec is obvious.
func binDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, nil, 0o755); err != nil {
			t.Fatalf("write fake bin: %v", err)
		}
		if err := os.Chmod(p, 0o755); err != nil {
			t.Fatalf("chmod fake bin: %v", err)
		}
	}
	return dir
}

func envFn(home string, present ...string) func(string) (string, bool) {
	set := map[string]struct{}{}
	for _, k := range present {
		set[k] = struct{}{}
	}
	return func(k string) (string, bool) {
		if k == "HOME" {
			return home, true
		}
		_, ok := set[k]
		return "", ok
	}
}

func hasWarning(ws []Warning, kind WarningKind) bool {
	for _, w := range ws {
		if w.Kind == kind {
			return true
		}
	}
	return false
}

func warningMsg(ws []Warning, kind WarningKind) (string, bool) {
	for _, w := range ws {
		if w.Kind == kind {
			return w.Msg, true
		}
	}
	return "", false
}

func TestWithLookPathBoundary(t *testing.T) {
	hostDir := t.TempDir()
	hostBin := filepath.Join(hostDir, fixtureBinAlpha)
	if err := os.WriteFile(hostBin, nil, 0o755); err != nil {
		t.Fatalf("write host-style bin: %v", err)
	}
	t.Setenv("PATH", hostDir)

	// Default Open uses process PATH; openAgents would close that boundary.
	dir := catalogtest.WriteModule(t, testCatalog)
	def, err := Open(
		WithCatalogDir(dir),
		WithCacheDir(t.TempDir()),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	if err != nil {
		t.Fatalf("default Open: %v", err)
	}
	d, err := def.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("default Get: %v", err)
	}
	if !d.Detection.Found {
		t.Fatal("default lookPath should find the fixture binary on PATH")
	}
	if d.Detection.BinaryPath != hostBin {
		t.Errorf("default BinaryPath = %q, want %q", d.Detection.BinaryPath, hostBin)
	}

	closed := openAgents(t, testCatalog,
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	d, err = closed.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("closed Get: %v", err)
	}
	if d.Detection.Found {
		t.Fatalf("closed lookPath found %q via host PATH", d.Detection.BinaryPath)
	}

	injected := openAgents(t, testCatalog,
		WithLookPath(func(name string) (string, error) {
			if name == fixtureBinAlpha {
				return hostBin, nil
			}
			return "", exec.ErrNotFound
		}),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	d, err = injected.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("injected Get: %v", err)
	}
	if !d.Detection.Found {
		t.Fatal("injected lookPath should find alpha-cli")
	}
	if d.Detection.BinaryPath != hostBin {
		t.Errorf("injected BinaryPath = %q, want %q", d.Detection.BinaryPath, hostBin)
	}

	// Non-executable lookPath hit must not Found; search dirs may still satisfy.
	missing := filepath.Join(t.TempDir(), "absent")
	search := binDir(t, fixtureBinAlpha)
	nonExec := openAgents(t, testCatalog,
		WithLookPath(func(name string) (string, error) {
			if name == fixtureBinAlpha {
				return missing, nil
			}
			return "", exec.ErrNotFound
		}),
		WithSearchDirs(search),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	d, err = nonExec.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("non-exec lookPath Get: %v", err)
	}
	if !d.Detection.Found {
		t.Fatal("search dirs should find alpha-cli after a non-executable lookPath hit")
	}
	wantSearch := filepath.Join(search, fixtureBinAlpha)
	if d.Detection.BinaryPath != wantSearch {
		t.Errorf("BinaryPath = %q, want search-dir path %q", d.Detection.BinaryPath, wantSearch)
	}

	onlyBad := openAgents(t, testCatalog,
		WithLookPath(func(name string) (string, error) {
			if name == fixtureBinAlpha {
				return missing, nil
			}
			return "", exec.ErrNotFound
		}),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	d, err = onlyBad.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("only-bad lookPath Get: %v", err)
	}
	if d.Detection.Found {
		t.Fatalf("non-executable lookPath should not Found, got %q", d.Detection.BinaryPath)
	}
}

func TestWithBinPathsOverride(t *testing.T) {
	// Override is the sole candidate: closed lookPath and no search dirs still find it.
	overrideDir := binDir(t, fixtureBinAlpha)
	override := filepath.Join(overrideDir, fixtureBinAlpha)
	wantAbs, err := filepath.Abs(override)
	if err != nil {
		t.Fatalf("abs override: %v", err)
	}

	idx := openAgents(t, testCatalog,
		WithBinPaths(map[string]string{"alpha-cli": override}),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	d, err := idx.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !d.Detection.Found {
		t.Fatal("WithBinPaths should find alpha-cli with lookPath closed and no search dirs")
	}
	if d.Detection.BinaryPath != wantAbs {
		t.Errorf("BinaryPath = %q, want override %q", d.Detection.BinaryPath, wantAbs)
	}

	// Non-executable override must not Found and must not fall through to lookPath/searchDirs.
	missing := filepath.Join(t.TempDir(), "absent")
	search := binDir(t, fixtureBinAlpha)
	bad := openAgents(t, testCatalog,
		WithBinPaths(map[string]string{"alpha-cli": missing}),
		WithSearchDirs(search),
		WithLookPath(func(name string) (string, error) {
			if name == fixtureBinAlpha {
				return filepath.Join(search, fixtureBinAlpha), nil
			}
			return "", exec.ErrNotFound
		}),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	d, err = bad.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("non-exec override Get: %v", err)
	}
	if d.Detection.Found {
		t.Fatalf("non-executable WithBinPaths must not Found or fall through, got %q", d.Detection.BinaryPath)
	}

	// Relative override roots at WithWorkingDir.
	wd := t.TempDir()
	relName := "rel-bin"
	relPath := filepath.Join(wd, relName)
	if err := os.WriteFile(relPath, nil, 0o755); err != nil {
		t.Fatalf("write relative bin: %v", err)
	}
	if err := os.Chmod(relPath, 0o755); err != nil {
		t.Fatalf("chmod relative bin: %v", err)
	}
	rel := openAgents(t, testCatalog,
		WithBinPaths(map[string]string{"alpha-cli": relName}),
		WithWorkingDir(wd),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	d, err = rel.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("relative override Get: %v", err)
	}
	if !d.Detection.Found {
		t.Fatal("relative WithBinPaths should resolve under WorkingDir")
	}
	if d.Detection.BinaryPath != relPath {
		t.Errorf("BinaryPath = %q, want %q", d.Detection.BinaryPath, relPath)
	}
}

func TestUnknownBinPathOverrideWarns(t *testing.T) {
	ctx := context.Background()
	q := AgentQuery{Enrich: EnrichNone}
	gq := AgentGetQuery{Enrich: EnrichNone}

	t.Run("unknown id", func(t *testing.T) {
		idx := openAgents(t, testCatalog,
			WithBinPaths(map[string]string{"no-such-agent": "/usr/bin/true"}),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(modelsdevtest.MustNotFetch(t)),
		)
		want := `unknown binary-path override id "no-such-agent"`

		res, err := idx.Agents.List(ctx, q)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if msg, ok := warningMsg(res.Warnings, WarnUnknownBinPath); !ok || msg != want {
			t.Errorf("List warnings = %v, want %q", res.Warnings, want)
		}
		if got := agentIDs(res.Items); !equal(got, []string{"alpha-cli", "delta-agent", "gamma-agent"}) {
			t.Errorf("List items = %v, want the full catalog", got)
		}

		d, err := idx.Agents.Get(ctx, "alpha-cli", gq)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if msg, ok := warningMsg(d.Warnings, WarnUnknownBinPath); !ok || msg != want {
			t.Errorf("Get warnings = %v, want %q", d.Warnings, want)
		}
		if d.ID != "alpha-cli" {
			t.Errorf("Get id = %q, want alpha-cli", d.ID)
		}
	})

	t.Run("multiple unknown ids one warning", func(t *testing.T) {
		idx := openAgents(t, testCatalog,
			WithBinPaths(map[string]string{
				"zeta-missing":  "/usr/bin/true",
				"no-such-agent": "/usr/bin/true",
			}),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(modelsdevtest.MustNotFetch(t)),
		)
		want := `unknown binary-path override ids "no-such-agent", "zeta-missing"`
		res, err := idx.Agents.List(ctx, q)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		var n int
		for _, w := range res.Warnings {
			if w.Kind == WarnUnknownBinPath {
				n++
				if w.Msg != want {
					t.Errorf("Msg = %q, want %q", w.Msg, want)
				}
			}
		}
		if n != 1 {
			t.Errorf("WarnUnknownBinPath count = %d, want 1; warnings = %v", n, res.Warnings)
		}
	})

	t.Run("catalogued id omitted from result", func(t *testing.T) {
		idx := openAgents(t, testCatalog,
			WithBinPaths(map[string]string{"alpha-cli": "/nonexistent"}),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(modelsdevtest.MustNotFetch(t)),
		)

		res, err := idx.Agents.List(ctx, AgentQuery{Enrich: EnrichNone, Installed: true})
		if err != nil {
			t.Fatalf("List installed: %v", err)
		}
		if hasWarning(res.Warnings, WarnUnknownBinPath) {
			t.Errorf("installed List warned on catalogued override: %v", res.Warnings)
		}
		if len(res.Items) != 0 {
			t.Errorf("installed List items = %v, want empty", agentIDs(res.Items))
		}

		d, err := idx.Agents.Get(ctx, "delta-agent", gq)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if hasWarning(d.Warnings, WarnUnknownBinPath) {
			t.Errorf("Get of another agent warned on catalogued override: %v", d.Warnings)
		}
	})
}

func TestRelativeSearchDirRootsAtWorkingDir(t *testing.T) {
	// Relative search dirs root at WorkingDir before the executable check (same
	// order as WithBinPaths), so a dir that only exists under WorkingDir still finds.
	wd := t.TempDir()
	relDir := "local-bins"
	absDir := filepath.Join(wd, relDir)
	if err := os.Mkdir(absDir, 0o755); err != nil {
		t.Fatalf("mkdir search dir: %v", err)
	}
	binPath := filepath.Join(absDir, fixtureBinAlpha)
	if err := os.WriteFile(binPath, nil, 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}
	if err := os.Chmod(binPath, 0o755); err != nil {
		t.Fatalf("chmod bin: %v", err)
	}

	idx := openAgents(t, testCatalog,
		WithSearchDirs(relDir),
		WithWorkingDir(wd),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	d, err := idx.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !d.Detection.Found {
		t.Fatal("relative search dir should resolve under WorkingDir")
	}
	if d.Detection.BinaryPath != binPath {
		t.Errorf("BinaryPath = %q, want %q", d.Detection.BinaryPath, binPath)
	}
}

func TestExpandPathEnvAndBareTilde(t *testing.T) {
	// $VAR expansion uses the injected lookup; bare ~ is home; unset vars become empty.
	body := fmt.Sprintf(`
agents: "alpha-cli": {
	name: "Alpha CLI"
	bin:  %q
	config: {
		global: "$XDG_CONFIG_HOME/alpha"
		local:  ".alpha"
	}
	skills: {
		global: {native: "$XDG_DATA_HOME/skills"}
		local:  {native: ".alpha/skills"}
	}
	provider: ["anthropic"]
}
agents: "beta-tool": {
	name: "Beta Tool"
	bin:  %q
	config: {global: "~"}
	provider: ["openai"]
}
agents: "gamma-agent": {
	name: "Gamma Agent"
	bin:  %q
	config: {global: "$UNSET_AGENTDEX_VAR/gone"}
	provider: ["google"]
}
`, fixtureBinAlpha, fixtureBinBeta, fixtureBinGamma)

	xdgConfig := t.TempDir()
	xdgData := t.TempDir()
	home := t.TempDir()
	wd := t.TempDir()
	lookup := func(k string) (string, bool) {
		switch k {
		case "HOME":
			return home, true
		case "XDG_CONFIG_HOME":
			return xdgConfig, true
		case "XDG_DATA_HOME":
			return xdgData, true
		default:
			return "", false
		}
	}
	idx := openAgents(t, body,
		WithEnvLookup(lookup),
		WithWorkingDir(wd),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)

	alpha, err := idx.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("alpha Get: %v", err)
	}
	wantGlobal := filepath.Join(xdgConfig, "alpha")
	if alpha.Detection.Config.Global != wantGlobal {
		t.Errorf("alpha Config.Global = %q, want $XDG_CONFIG_HOME expanded to %q", alpha.Detection.Config.Global, wantGlobal)
	}
	wantSkills := filepath.Join(xdgData, "skills")
	if alpha.Detection.Skills.Global.Native.Path != wantSkills {
		t.Errorf("alpha skills.global.native = %q, want %q", alpha.Detection.Skills.Global.Native.Path, wantSkills)
	}

	beta, err := idx.Agents.Get(context.Background(), "beta-tool", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("beta Get: %v", err)
	}
	if beta.Detection.Config.Global != home {
		t.Errorf("beta bare ~ Config.Global = %q, want home %q", beta.Detection.Config.Global, home)
	}

	gamma, err := idx.Agents.Get(context.Background(), "gamma-agent", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("gamma Get: %v", err)
	}
	// Unset vars become empty: "$UNSET_AGENTDEX_VAR/gone" → "/gone". No XDG home fallback here.
	if gamma.Detection.Config.Global != "/gone" {
		t.Errorf("gamma Config.Global = %q, want /gone after empty env expand", gamma.Detection.Config.Global)
	}
}

func TestDetectionDoesNotExecuteBinary(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "ran")
	bin := filepath.Join(dir, fixtureBinAlpha)
	script := fmt.Sprintf("#!/bin/sh\necho ran >> %q\n", counter)
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatalf("chmod bin: %v", err)
	}
	idx := openAgents(t, testCatalog,
		WithSearchDirs(dir),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	d, err := idx.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !d.Detection.Found {
		t.Fatal("alpha-cli should be found")
	}
	if _, err := os.Stat(counter); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Get executed the agent binary")
	}
	if _, err := idx.Agents.List(context.Background(), AgentQuery{Enrich: EnrichNone, Installed: true}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if _, err := os.Stat(counter); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("List executed the agent binary")
	}
}

func TestGetDetectionFactsOfflineAtEnrichNone(t *testing.T) {
	home := t.TempDir()
	wd := t.TempDir()
	idx := openAgents(t, testCatalog,
		WithSearchDirs(binDir(t, fixtureBinAlpha)),
		WithEnvLookup(envFn(home)),
		WithWorkingDir(wd),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)

	d, err := idx.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !d.Detection.Found {
		t.Fatal("alpha-cli should be found")
	}
	if d.Detection.Config.Global != filepath.Join(home, ".alpha") {
		t.Errorf("Config.Global = %q, want %s/.alpha", d.Detection.Config.Global, home)
	}
	if d.Detection.Config.Local != filepath.Join(wd, ".alpha") {
		t.Errorf("Config.Local = %q, want %s/.alpha", d.Detection.Config.Local, wd)
	}
	if d.Enrichment != EnrichmentNotRequested {
		t.Errorf("Enrichment = %v, want EnrichmentNotRequested", d.Enrichment)
	}
	if len(d.ResolvedProviders) != 0 {
		t.Errorf("ResolvedProviders = %v, want none at EnrichNone", d.ResolvedProviders)
	}
	if d.Coverage.Status != CoverageNotProbed {
		t.Errorf("Coverage.Status = %v, want CoverageNotProbed", d.Coverage.Status)
	}
}

func TestGetHomeProviderEnrichProvidersOffline(t *testing.T) {
	idx := openAgents(t, testCatalog,
		WithSearchDirs(binDir(t, fixtureBinAlpha)),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	d, err := idx.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichProviders})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(d.ResolvedProviders) != 1 || d.ResolvedProviders[0] != "anthropic" {
		t.Errorf("ResolvedProviders = %v, want [anthropic]", d.ResolvedProviders)
	}
	if len(d.CatalogProviders) != 1 || d.CatalogProviders[0] != "anthropic" {
		t.Errorf("CatalogProviders = %v, want [anthropic]", d.CatalogProviders)
	}
	if d.Enrichment != EnrichmentApplied {
		t.Errorf("Enrichment = %v, want EnrichmentApplied", d.Enrichment)
	}
	if d.Coverage.Status != CoverageNotProbed {
		t.Errorf("Coverage.Status = %v, want CoverageNotProbed", d.Coverage.Status)
	}
}

// Caller mutation of returned slices must not affect later Get/List on the same Index.
func TestProviderSlicesDoNotAliasCatalog(t *testing.T) {
	idx := openAgents(t, testCatalog,
		WithSearchDirs(binDir(t, fixtureBinAlpha)),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	ctx := context.Background()

	d, err := idx.Agents.Get(ctx, "alpha-cli", AgentGetQuery{Enrich: EnrichProviders})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	d.CatalogProviders[0] = "evil"
	d.ResolvedProviders[0] = "evil"

	again, err := idx.Agents.Get(ctx, "alpha-cli", AgentGetQuery{Enrich: EnrichProviders})
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if again.CatalogProviders[0] != "anthropic" || again.ResolvedProviders[0] != "anthropic" {
		t.Errorf("after mutation Get CatalogProviders=%v ResolvedProviders=%v, want anthropic",
			again.CatalogProviders, again.ResolvedProviders)
	}

	res, err := idx.Agents.List(ctx, AgentQuery{Enrich: EnrichProviders})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, a := range res.Items {
		if a.ID != "alpha-cli" {
			continue
		}
		if a.CatalogProviders[0] != "anthropic" || a.ResolvedProviders[0] != "anthropic" {
			t.Errorf("after mutation List CatalogProviders=%v ResolvedProviders=%v, want anthropic",
				a.CatalogProviders, a.ResolvedProviders)
		}
		a.CatalogProviders[0] = "evil-list"
		break
	}
	third, err := idx.Agents.Get(ctx, "alpha-cli", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("third Get: %v", err)
	}
	if third.CatalogProviders[0] != "anthropic" {
		t.Errorf("after List mutation CatalogProviders=%v, want [anthropic]", third.CatalogProviders)
	}
}

func TestGetHomeProviderRejectsExplicitProvidersEveryLevel(t *testing.T) {
	idx := openAgents(t, testCatalog,
		WithSearchDirs(binDir(t, fixtureBinAlpha)),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	for _, lvl := range []Enrich{EnrichNone, EnrichProviders, EnrichCount, EnrichFull} {
		d, err := idx.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: lvl, Providers: []string{"anthropic"}})
		if !errors.Is(err, ErrProvidersNotAllowed) {
			t.Errorf("level %v: err = %v, want ErrProvidersNotAllowed", lvl, err)
		}
		if err != nil && err.Error() != `agent "alpha-cli" has catalog providers` {
			t.Errorf("level %v: message = %q", lvl, err.Error())
		}
		_ = d
	}
}

func TestGetAgnosticNoProvidersNotApplicable(t *testing.T) {
	idx := openAgents(t, testCatalog,
		WithSearchDirs(binDir(t, fixtureBinDelta)),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	for _, lvl := range []Enrich{EnrichCount, EnrichFull} {
		d, err := idx.Agents.Get(context.Background(), "delta-agent", AgentGetQuery{Enrich: lvl})
		if err != nil {
			t.Fatalf("level %v: Get: %v", lvl, err)
		}
		if d.Enrichment != EnrichmentNotApplicable {
			t.Errorf("level %v: Enrichment = %v, want EnrichmentNotApplicable", lvl, d.Enrichment)
		}
		if msg, ok := warningMsg(d.Warnings, WarnProvidersRequired); !ok || msg != `"delta-agent" is provider-agnostic` {
			t.Errorf("level %v: providers-required warning = %q (present=%v)", lvl, msg, ok)
		}
		if d.Coverage.Status != CoverageNotProbed {
			t.Errorf("level %v: Coverage.Status = %v, want CoverageNotProbed", lvl, d.Coverage.Status)
		}
	}
}

func TestGetAgnosticValidatesCallerProviders(t *testing.T) {
	srv := modelsdevtest.Server(t, []string{"google"})
	idx := openAgents(t, testCatalog,
		WithSearchDirs(binDir(t, fixtureBinDelta)),
		WithEnvLookup(envFn(t.TempDir(), "GOOGLE_API_KEY")),
		WithModelsURL(srv.URL),
	)
	d, err := idx.Agents.Get(context.Background(), "delta-agent", AgentGetQuery{Enrich: EnrichFull, Providers: []string{"google"}})
	if err != nil {
		t.Fatalf("Get google: %v", err)
	}
	if len(d.ResolvedProviders) != 1 || d.ResolvedProviders[0] != "google" {
		t.Errorf("ResolvedProviders = %v, want [google]", d.ResolvedProviders)
	}
	if len(d.CatalogProviders) != 0 {
		t.Errorf("CatalogProviders = %v, want empty on agnostic agent", d.CatalogProviders)
	}
	if d.Enrichment != EnrichmentApplied || d.ModelCount != 1 {
		t.Errorf("Enrichment=%v ModelCount=%d, want EnrichmentApplied 1", d.Enrichment, d.ModelCount)
	}
	_, err = idx.Agents.Get(context.Background(), "delta-agent", AgentGetQuery{Enrich: EnrichProviders, Providers: []string{"bogus"}})
	if !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("unknown provider err = %v, want ErrUnknownProvider", err)
	}
}

// EnrichProviders has no coverage channel: a degraded verdict must carry a warning
// or the caller sees EnrichmentDegraded with no way to learn why.
func TestEnrichProvidersDegradeStatesFault(t *testing.T) {
	q := AgentGetQuery{Enrich: EnrichProviders, Providers: []string{"google"}}
	lq := AgentQuery{Enrich: EnrichProviders, Providers: []string{"google"}}

	t.Run("schema drift", func(t *testing.T) {
		srv := modelsdevtest.Server(t, []string{"google"}, "google")
		idx := openAgents(t, testCatalog,
			WithSearchDirs(binDir(t, fixtureBinDelta)),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(srv.URL),
		)
		d, err := idx.Agents.Get(context.Background(), "delta-agent", q)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if d.Enrichment != EnrichmentDegraded {
			t.Errorf("Enrichment = %v, want EnrichmentDegraded", d.Enrichment)
		}
		if len(d.ResolvedProviders) != 1 || d.ResolvedProviders[0] != "google" {
			t.Errorf("ResolvedProviders = %v, want [google] reported despite the drift", d.ResolvedProviders)
		}
		msg, ok := warningMsg(d.Warnings, WarnModelsSchemaDrift)
		if !ok || msg != `provider ids unvalidated: provider "google" model "google-model" malformed: models.dev schema unrecognised` {
			t.Errorf("get drift warning = %q (present=%v)", msg, ok)
		}

		res, lerr := idx.Agents.List(context.Background(), lq)
		if lerr != nil {
			t.Fatalf("List: %v", lerr)
		}
		if d := byID(res.Items)["delta-agent"]; d.Enrichment != EnrichmentDegraded {
			t.Errorf("delta Enrichment = %v, want EnrichmentDegraded", d.Enrichment)
		}
		if msg, ok := warningMsg(res.Warnings, WarnModelsSchemaDrift); !ok || msg != `provider ids unvalidated: provider "google" model "google-model" malformed: models.dev schema unrecognised` {
			t.Errorf("list drift warning = %q (present=%v)", msg, ok)
		}
	})

	t.Run("unreachable", func(t *testing.T) {
		idx := openAgents(t, testCatalog,
			WithSearchDirs(binDir(t, fixtureBinDelta)),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(modelsdevtest.Closed(t)),
		)
		d, err := idx.Agents.Get(context.Background(), "delta-agent", q)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if d.Enrichment != EnrichmentDegraded {
			t.Errorf("Enrichment = %v, want EnrichmentDegraded", d.Enrichment)
		}
		msg, ok := warningMsg(d.Warnings, WarnModelsUnreachable)
		if !ok || msg != "provider ids unvalidated: models.dev is unreachable and not cached" {
			t.Errorf("get unreachable warning = %q (present=%v)", msg, ok)
		}

		res, lerr := idx.Agents.List(context.Background(), lq)
		if lerr != nil {
			t.Fatalf("List: %v", lerr)
		}
		if d := byID(res.Items)["delta-agent"]; d.Enrichment != EnrichmentDegraded {
			t.Errorf("delta Enrichment = %v, want EnrichmentDegraded", d.Enrichment)
		}
		if msg, ok := warningMsg(res.Warnings, WarnModelsUnreachable); !ok || msg != "provider ids unvalidated: models.dev is unreachable and not cached" {
			t.Errorf("list unreachable warning = %q (present=%v)", msg, ok)
		}
	})

	t.Run("clean validation stays silent", func(t *testing.T) {
		srv := modelsdevtest.Server(t, []string{"google"})
		idx := openAgents(t, testCatalog,
			WithSearchDirs(binDir(t, fixtureBinDelta)),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(srv.URL),
		)
		d, err := idx.Agents.Get(context.Background(), "delta-agent", q)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if d.Enrichment != EnrichmentApplied {
			t.Errorf("Enrichment = %v, want EnrichmentApplied", d.Enrichment)
		}
		if hasWarning(d.Warnings, WarnModelsSchemaDrift) || hasWarning(d.Warnings, WarnModelsUnreachable) {
			t.Error("a clean validation must raise no models.dev warning")
		}
		res, lerr := idx.Agents.List(context.Background(), lq)
		if lerr != nil {
			t.Fatalf("List: %v", lerr)
		}
		if hasWarning(res.Warnings, WarnModelsSchemaDrift) || hasWarning(res.Warnings, WarnModelsUnreachable) {
			t.Error("a clean listing validation must raise no models.dev warning")
		}
	})
}

func TestGetCoverageVerdicts(t *testing.T) {
	t.Run("all present", func(t *testing.T) {
		srv := modelsdevtest.Server(t, []string{"google", "openai"})
		idx := openAgents(t, testCatalog,
			WithSearchDirs(binDir(t, fixtureBinGamma)),
			WithEnvLookup(envFn(t.TempDir(), "GOOGLE_API_KEY")),
			WithModelsURL(srv.URL),
		)
		d, err := idx.Agents.Get(context.Background(), "gamma-agent", AgentGetQuery{Enrich: EnrichFull})
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if d.Coverage.Status != CoverageAllPresent {
			t.Errorf("Status = %v, want CoverageAllPresent", d.Coverage.Status)
		}
		if d.Enrichment != EnrichmentApplied || d.ModelCount != 2 {
			t.Errorf("Enrichment=%v ModelCount=%d, want EnrichmentApplied 2", d.Enrichment, d.ModelCount)
		}
		if d.ProviderEnv["GOOGLE_API_KEY"] != true || d.ProviderEnv["OPENAI_API_KEY"] != false {
			t.Errorf("ProviderEnv = %v, want GOOGLE present, OPENAI absent", d.ProviderEnv)
		}
		// Newest first: openai (2025-01-01) before google (2024-01-01).
		if len(d.Models) != 2 || d.Models[0].ID != "openai-model" {
			t.Errorf("Models order = %v, want openai-model first", modelIDs(d.Models))
		}
		if d.Models[0].Provider != "openai" || d.Models[1].Provider != "google" {
			t.Errorf("Models providers = %s/%s, want openai then google", d.Models[0].Provider, d.Models[1].Provider)
		}
		// Agnostic map only has anthropic/claude-sonnet; agent path keeps empty CanonicalID.
		if d.Models[0].CanonicalID != "" || d.Models[1].CanonicalID != "" {
			t.Errorf("gamma CanonicalIDs = %q/%q, want empty (not in agnostic map)", d.Models[0].CanonicalID, d.Models[1].CanonicalID)
		}
	})

	t.Run("some present", func(t *testing.T) {
		srv := modelsdevtest.Server(t, []string{"google"})
		idx := openAgents(t, testCatalog,
			WithSearchDirs(binDir(t, fixtureBinGamma)),
			WithEnvLookup(envFn(t.TempDir(), "GOOGLE_API_KEY")),
			WithModelsURL(srv.URL),
		)
		d, err := idx.Agents.Get(context.Background(), "gamma-agent", AgentGetQuery{Enrich: EnrichFull})
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if d.Coverage.Status != CoverageSomePresent {
			t.Errorf("Status = %v, want CoverageSomePresent", d.Coverage.Status)
		}
		if len(d.Coverage.Absent) != 1 || d.Coverage.Absent[0] != "openai" {
			t.Errorf("Absent = %v, want [openai]", d.Coverage.Absent)
		}
		if msg, ok := warningMsg(d.Warnings, WarnSomeProvidersAbsent); !ok || msg != "some providers are absent from models.dev: openai" {
			t.Errorf("some-absent warning = %q (present=%v)", msg, ok)
		}
		if len(d.Models) != 1 || d.Models[0].ID != "google-model" || d.Models[0].Provider != "google" {
			t.Errorf("Models = %v (provider %q), want google/google-model", modelIDs(d.Models), d.Models[0].Provider)
		}
	})

	t.Run("none present", func(t *testing.T) {
		srv := modelsdevtest.Server(t, []string{"google"})
		idx := openAgents(t, testCatalog,
			WithSearchDirs(binDir(t, fixtureBinAlpha)),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(srv.URL),
		)
		d, err := idx.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichFull})
		if err != nil {
			t.Fatalf("Get should not fail on a coverage verdict: %v", err)
		}
		if d.Coverage.Status != CoverageNonePresent {
			t.Errorf("Status = %v, want CoverageNonePresent", d.Coverage.Status)
		}
		if len(d.Coverage.Absent) != 1 || d.Coverage.Absent[0] != "anthropic" {
			t.Errorf("Absent = %v, want [anthropic]", d.Coverage.Absent)
		}
		if d.Enrichment != EnrichmentApplied {
			t.Errorf("Enrichment = %v, want EnrichmentApplied (a true zero)", d.Enrichment)
		}
	})

	t.Run("schema drift", func(t *testing.T) {
		srv := modelsdevtest.Server(t, []string{"google"}, "anthropic")
		idx := openAgents(t, testCatalog,
			WithSearchDirs(binDir(t, fixtureBinAlpha)),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(srv.URL),
		)
		d, err := idx.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichFull})
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if d.Coverage.Status != CoverageSchemaDrift {
			t.Errorf("Status = %v, want CoverageSchemaDrift", d.Coverage.Status)
		}
		if !errors.Is(d.Coverage.Err, modelsdev.ErrModelsSchema) {
			t.Errorf("Coverage.Err = %v, want to wrap ErrModelsSchema", d.Coverage.Err)
		}
		if !errors.Is(d.Coverage.Err, ErrModelsSchema) {
			t.Errorf("Coverage.Err = %v, want to wrap root ErrModelsSchema alias", d.Coverage.Err)
		}
		if d.Enrichment != EnrichmentDegraded {
			t.Errorf("Enrichment = %v, want EnrichmentDegraded", d.Enrichment)
		}
		if hasWarning(d.Warnings, WarnModelsUnreachable) {
			t.Error("schema drift on Get must not raise the unreachable warning")
		}
		msg, ok := warningMsg(d.Warnings, WarnModelsSchemaDrift)
		if !ok || !strings.Contains(msg, "model enrichment omitted:") {
			t.Errorf("schema-drift warning = %q (present=%v), want Get enrichment wording", msg, ok)
		}
		if strings.Contains(msg, "model counts omitted") {
			t.Errorf("Get must not use list count wording: %q", msg)
		}
	})

	t.Run("schema drift at EnrichProviders", func(t *testing.T) {
		// Drift must warn unvalidated ids, not omitted model data.
		srv := modelsdevtest.Server(t, nil, "google")
		idx := openAgents(t, testCatalog,
			WithSearchDirs(binDir(t, fixtureBinDelta)),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(srv.URL),
		)
		d, err := idx.Agents.Get(context.Background(), "delta-agent", AgentGetQuery{
			Enrich:    EnrichProviders,
			Providers: []string{"google"},
		})
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if d.Enrichment != EnrichmentDegraded {
			t.Errorf("Enrichment = %v, want EnrichmentDegraded", d.Enrichment)
		}
		msg, ok := warningMsg(d.Warnings, WarnModelsSchemaDrift)
		if !ok || !strings.Contains(msg, "provider ids unvalidated:") {
			t.Errorf("EnrichProviders schema-drift warning = %q (present=%v)", msg, ok)
		}
		if strings.Contains(msg, "model counts omitted") || strings.Contains(msg, "model enrichment omitted") {
			t.Errorf("EnrichProviders must not use model-omission wording: %q", msg)
		}
	})

	t.Run("unreachable", func(t *testing.T) {
		idx := openAgents(t, testCatalog,
			WithSearchDirs(binDir(t, fixtureBinAlpha)),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(modelsdevtest.Closed(t)),
		)
		d, err := idx.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichFull})
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if d.Coverage.Status != CoverageUnreachable {
			t.Errorf("Status = %v, want CoverageUnreachable", d.Coverage.Status)
		}
		if d.Enrichment != EnrichmentDegraded {
			t.Errorf("Enrichment = %v, want EnrichmentDegraded", d.Enrichment)
		}
		if msg, ok := warningMsg(d.Warnings, WarnModelsUnreachable); !ok || msg != "models.dev is unreachable and not cached: model enrichment and provider-env omitted" {
			t.Errorf("unreachable warning = %q (present=%v)", msg, ok)
		}
	})
}

func TestGetNotInstalledEnrichesLikeInstalled(t *testing.T) {
	srv := modelsdevtest.Server(t, []string{"anthropic"})
	idx := openAgents(t, testCatalog,
		WithSearchDirs(binDir(t)),
		WithEnvLookup(envFn(t.TempDir(), "ANTHROPIC_API_KEY")),
		WithModelsURL(srv.URL),
	)
	d, err := idx.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichFull})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if d.Detection.Found {
		t.Fatal("alpha-cli should not be installed")
	}
	if !hasWarning(d.Warnings, WarnNotInstalled) {
		t.Errorf("expected the not-installed warning: %v", d.Warnings)
	}
	if msg, _ := warningMsg(d.Warnings, WarnNotInstalled); msg != `agent "alpha-cli" is catalogued but not installed` {
		t.Errorf("not-installed message = %q", msg)
	}
	// Enrichment does not depend on installation.
	if d.Coverage.Status != CoverageAllPresent || d.Enrichment != EnrichmentApplied {
		t.Errorf("Status=%v Enrichment=%v, want CoverageAllPresent EnrichmentApplied", d.Coverage.Status, d.Enrichment)
	}
	if d.ModelCount != 1 || len(d.Models) != 1 {
		t.Errorf("ModelCount=%d Models=%v, want 1 model filled", d.ModelCount, modelIDs(d.Models))
	}
	if d.Models[0].Provider != "anthropic" || d.Models[0].CanonicalID != "anthropic/claude-sonnet" {
		t.Errorf("Models[0] = %s/%s canonical %q, want anthropic/claude-sonnet with canonical id",
			d.Models[0].Provider, d.Models[0].ID, d.Models[0].CanonicalID)
	}
	if _, ok := d.ProviderEnv["ANTHROPIC_API_KEY"]; !ok {
		t.Errorf("ProviderEnv = %v, want ANTHROPIC_API_KEY reported", d.ProviderEnv)
	}
}

func TestGetEnrichCountOmitsModelsList(t *testing.T) {
	srv := modelsdevtest.Server(t, []string{"anthropic"})
	idx := openAgents(t, testCatalog,
		WithSearchDirs(binDir(t, fixtureBinAlpha)),
		WithEnvLookup(envFn(t.TempDir(), "ANTHROPIC_API_KEY")),
		WithModelsURL(srv.URL),
	)
	d, err := idx.Agents.Get(context.Background(), "alpha-cli", AgentGetQuery{Enrich: EnrichCount})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if d.ModelCount != 1 {
		t.Errorf("ModelCount = %d, want 1", d.ModelCount)
	}
	if d.Models != nil {
		t.Errorf("Models = %v, want nil at EnrichCount", modelIDs(d.Models))
	}
	if d.ProviderEnv == nil {
		t.Error("ProviderEnv should be filled at EnrichCount")
	}
}

func TestGetUnknownAgentCarriesMessage(t *testing.T) {
	idx := openAgents(t, testCatalog, WithModelsURL(modelsdevtest.MustNotFetch(t)))
	_, err := idx.Agents.Get(context.Background(), "no-such", AgentGetQuery{Enrich: EnrichNone})
	if !errors.Is(err, ErrAgentUnknown) {
		t.Fatalf("err = %v, want ErrAgentUnknown", err)
	}
	if err.Error() != `no agent "no-such"` {
		t.Errorf("message = %q, want library text", err.Error())
	}
}

func TestListOrdersByIDAndNarrowsByInstalled(t *testing.T) {
	idx := openAgents(t, testCatalog,
		WithSearchDirs(binDir(t, fixtureBinAlpha, fixtureBinGamma)),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	res, err := idx.Agents.List(context.Background(), AgentQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := agentIDs(res.Items); !equal(got, []string{"alpha-cli", "delta-agent", "gamma-agent"}) {
		t.Errorf("order = %v, want by id", got)
	}

	res, err = idx.Agents.List(context.Background(), AgentQuery{Enrich: EnrichNone, Installed: true})
	if err != nil {
		t.Fatalf("List installed: %v", err)
	}
	if got := agentIDs(res.Items); !equal(got, []string{"alpha-cli", "gamma-agent"}) {
		t.Errorf("installed = %v, want the detected agents", got)
	}
}

func TestListFilterNarrowsByIDAndName(t *testing.T) {
	idx := openAgents(t, testCatalog,
		WithSearchDirs(binDir(t, fixtureBinAlpha, fixtureBinGamma, fixtureBinDelta)),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	res, err := idx.Agents.List(context.Background(), AgentQuery{Enrich: EnrichNone, Filter: "alpha"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := agentIDs(res.Items); !equal(got, []string{"alpha-cli"}) {
		t.Errorf("filtered = %v, want [alpha-cli]", got)
	}
}

func TestListEnrichFullPerAgent(t *testing.T) {
	srv := modelsdevtest.Server(t, []string{"anthropic", "google", "openai"})
	idx := openAgents(t, testCatalog,
		WithSearchDirs(binDir(t, fixtureBinAlpha, fixtureBinGamma, fixtureBinDelta)),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(srv.URL),
	)
	res, err := idx.Agents.List(context.Background(), AgentQuery{Enrich: EnrichFull})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	by := byID(res.Items)
	if a := by["alpha-cli"]; a.Enrichment != EnrichmentApplied || a.ModelCount != 1 {
		t.Errorf("alpha: Enrichment=%v ModelCount=%d", a.Enrichment, a.ModelCount)
	}
	if a := by["alpha-cli"]; len(a.Models) != 1 || a.Models[0].CanonicalID != "anthropic/claude-sonnet" {
		t.Errorf("alpha CanonicalID = %v, want anthropic/claude-sonnet on agent List path", a.Models)
	}
	if g := by["gamma-agent"]; g.Enrichment != EnrichmentApplied || g.ModelCount != 2 {
		t.Errorf("gamma: Enrichment=%v ModelCount=%d", g.Enrichment, g.ModelCount)
	}
	if d := by["delta-agent"]; d.Enrichment != EnrichmentNotApplicable || len(d.Models) != 0 {
		t.Errorf("delta: Enrichment=%v Models=%v, want not-applicable and empty", d.Enrichment, modelIDs(d.Models))
	}
	if hasWarning(res.Warnings, WarnProvidersRequired) {
		t.Error("a listing must not raise the agnostic guidance warning")
	}
	if g := by["gamma-agent"]; g.Models[0].ID != "openai-model" || g.Models[0].Provider != "openai" {
		t.Errorf("gamma Models[0] = %s/%s, want openai/openai-model", g.Models[0].Provider, g.Models[0].ID)
	}
	if g := by["gamma-agent"]; g.Models[0].CanonicalID != "" {
		t.Errorf("gamma Models[0].CanonicalID = %q, want empty on agent List path", g.Models[0].CanonicalID)
	}
}

func TestListDegradeWarnings(t *testing.T) {
	t.Run("unreachable", func(t *testing.T) {
		idx := openAgents(t, testCatalog,
			WithSearchDirs(binDir(t, fixtureBinAlpha)),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(modelsdevtest.Closed(t)),
		)
		res, err := idx.Agents.List(context.Background(), AgentQuery{Enrich: EnrichFull, Installed: true})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if msg, ok := warningMsg(res.Warnings, WarnModelsUnreachable); !ok || msg != "model counts unavailable: models.dev is unreachable and not cached" {
			t.Errorf("list unreachable warning = %q (present=%v)", msg, ok)
		}
		if a := byID(res.Items)["alpha-cli"]; a.Enrichment != EnrichmentDegraded {
			t.Errorf("alpha Enrichment = %v, want EnrichmentDegraded", a.Enrichment)
		}
	})

	t.Run("schema drift", func(t *testing.T) {
		srv := modelsdevtest.Server(t, nil, "anthropic")
		idx := openAgents(t, testCatalog,
			WithSearchDirs(binDir(t, fixtureBinAlpha)),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(srv.URL),
		)
		res, err := idx.Agents.List(context.Background(), AgentQuery{Enrich: EnrichFull, Installed: true})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		msg, ok := warningMsg(res.Warnings, WarnModelsSchemaDrift)
		if !ok || msg != `model counts omitted: provider "anthropic" model "claude-sonnet" malformed: models.dev schema unrecognised` {
			t.Errorf("list schema-drift warning = %q (present=%v)", msg, ok)
		}
	})
}

func TestListProviderValidationAtBoundary(t *testing.T) {
	t.Run("unknown provider fails even when result is empty", func(t *testing.T) {
		srv := modelsdevtest.Server(t, []string{"anthropic"})
		idx := openAgents(t, testCatalog,
			WithSearchDirs(binDir(t)),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(srv.URL),
		)
		_, err := idx.Agents.List(context.Background(), AgentQuery{Enrich: EnrichFull, Installed: true, Providers: []string{"bogus"}})
		if !errors.Is(err, ErrUnknownProvider) {
			t.Errorf("err = %v, want ErrUnknownProvider regardless of which binaries are present", err)
		}
	})

	t.Run("unreachable degrades not rejects", func(t *testing.T) {
		idx := openAgents(t, testCatalog,
			WithSearchDirs(binDir(t, fixtureBinAlpha)),
			WithEnvLookup(envFn(t.TempDir())),
			WithModelsURL(modelsdevtest.Closed(t)),
		)
		res, err := idx.Agents.List(context.Background(), AgentQuery{Enrich: EnrichFull, Providers: []string{"anthropic"}})
		if err != nil {
			t.Fatalf("List: %v, want no rejection on an outage", err)
		}
		if !hasWarning(res.Warnings, WarnModelsUnreachable) {
			t.Errorf("expected a degrade warning, got %v", res.Warnings)
		}
	})
}

func TestListFetchesModelsDevOnce(t *testing.T) {
	srv, count := modelsdevtest.CountingServer(t, []string{"anthropic", "google", "openai"})
	idx := openAgents(t, testCatalog,
		WithSearchDirs(binDir(t, fixtureBinAlpha, fixtureBinGamma)),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(srv.URL),
	)
	if _, err := idx.Agents.List(context.Background(), AgentQuery{Enrich: EnrichFull}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if n := count.Load(); n != 1 {
		t.Errorf("models.dev fetched %d times, want once", n)
	}
}

func TestGetNoLocalConfigOrSkills(t *testing.T) {
	body := fmt.Sprintf(`
agents: "beta-tool": {
	name: "Beta Tool"
	bin:  %q
	config: {global: "~/.config/beta"}
	provider: ["openai"]
}
`, fixtureBinBeta)
	home := t.TempDir()
	idx := openAgents(t, body,
		WithSearchDirs(binDir(t, fixtureBinBeta)),
		WithEnvLookup(envFn(home)),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	d, err := idx.Agents.Get(context.Background(), "beta-tool", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if d.Detection.Config.Local != "" {
		t.Errorf("Config.Local = %q, want empty", d.Detection.Config.Local)
	}
	// SkillsPaths has slices; compare field-by-field, not with ==.
	if sk := d.Detection.Skills; !skillsPathsZero(sk) {
		t.Errorf("Skills = %+v, want zero value for an agent with no skills", sk)
	}
}

func skillsPathsZero(sk SkillsPaths) bool {
	return skillsScopeZero(sk.Global) && skillsScopeZero(sk.Local)
}

func skillsScopeZero(sc SkillsScope) bool {
	return sc.Agents.Path == "" && !sc.Agents.Exists &&
		sc.Native.Path == "" && !sc.Native.Exists &&
		len(sc.Alternatives) == 0 &&
		sc.Primary.Path == "" && !sc.Primary.Exists
}

func TestGetSkillsRolesAndPrimary(t *testing.T) {
	body := fmt.Sprintf(`
agents: "agy-like": {
	name: "Agy Like"
	bin:  %q
	config: {global: "~/.gemini/antigravity-cli", local: ".agents"}
	skills: {
		global: {native: "~/.gemini/antigravity-cli/skills"}
		local: {
			agents: ".agents/skills"
			alternatives: [".claude/skills", ".opencode/skills"]
		}
	}
	provider: ["google"]
}
agents: "open-like": {
	name: "Open Like"
	bin:  %q
	config: {global: "~/.config/open", local: ".open"}
	skills: {
		global: {
			agents: "~/.agents/skills"
			alternatives: ["~/.claude/skills"]
		}
		local: {
			agents: ".agents/skills"
			native: ".open/skills"
			alternatives: [".claude/skills"]
		}
	}
	provider: ["openai"]
}
`, fixtureBinAlpha, fixtureBinGamma)
	home := t.TempDir()
	mustMkdirAll(t, filepath.Join(home, ".gemini", "antigravity-cli", "skills"))
	mustMkdirAll(t, filepath.Join(home, ".agents", "skills"))
	wd := t.TempDir()
	mustMkdirAll(t, filepath.Join(wd, ".agents", "skills"))
	idx := openAgents(t, body,
		WithSearchDirs(binDir(t, fixtureBinAlpha, fixtureBinGamma)),
		WithEnvLookup(envFn(home)),
		WithWorkingDir(wd),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)

	agy, err := idx.Agents.Get(context.Background(), "agy-like", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("Get agy-like: %v", err)
	}
	wantNative := filepath.Join(home, ".gemini", "antigravity-cli", "skills")
	if agy.Detection.Skills.Global.Native.Path != wantNative || !agy.Detection.Skills.Global.Native.Exists {
		t.Errorf("agy global.native = %+v, want %q exists", agy.Detection.Skills.Global.Native, wantNative)
	}
	if agy.Detection.Skills.Global.Agents.Path != "" {
		t.Errorf("agy global.agents = %q, want empty", agy.Detection.Skills.Global.Agents.Path)
	}
	if agy.Detection.Skills.Global.Primary.Path != wantNative {
		t.Errorf("agy global.primary = %q, want native %q", agy.Detection.Skills.Global.Primary.Path, wantNative)
	}
	wantAgentsLocal := filepath.Join(wd, ".agents", "skills")
	if agy.Detection.Skills.Local.Agents.Path != wantAgentsLocal || !agy.Detection.Skills.Local.Agents.Exists {
		t.Errorf("agy local.agents = %+v, want %q exists", agy.Detection.Skills.Local.Agents, wantAgentsLocal)
	}
	wantAlts := []string{
		filepath.Join(wd, ".claude", "skills"),
		filepath.Join(wd, ".opencode", "skills"),
	}
	if len(agy.Detection.Skills.Local.Alternatives) != len(wantAlts) {
		t.Fatalf("agy local.alternatives len = %d, want %d", len(agy.Detection.Skills.Local.Alternatives), len(wantAlts))
	}
	for i, want := range wantAlts {
		if got := agy.Detection.Skills.Local.Alternatives[i].Path; got != want {
			t.Errorf("agy local.alternatives[%d] = %q, want %q", i, got, want)
		}
	}
	if agy.Detection.Skills.Local.Primary.Path != wantAgentsLocal {
		t.Errorf("agy local.primary = %q, want agents %q", agy.Detection.Skills.Local.Primary.Path, wantAgentsLocal)
	}

	open, err := idx.Agents.Get(context.Background(), "open-like", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("Get open-like: %v", err)
	}
	wantAgentsGlobal := filepath.Join(home, ".agents", "skills")
	if open.Detection.Skills.Global.Primary.Path != wantAgentsGlobal {
		t.Errorf("open global.primary = %q, want agents %q", open.Detection.Skills.Global.Primary.Path, wantAgentsGlobal)
	}
	if open.Detection.Skills.Local.Primary.Path != wantAgentsLocal {
		t.Errorf("open local.primary = %q, want agents %q", open.Detection.Skills.Local.Primary.Path, wantAgentsLocal)
	}
	if open.Detection.Skills.Local.Native.Path != filepath.Join(wd, ".open", "skills") {
		t.Errorf("open local.native = %q, want .open/skills", open.Detection.Skills.Local.Native.Path)
	}
}

func TestGetSkillsPrimaryFromAlternativesOnly(t *testing.T) {
	body := fmt.Sprintf(`
agents: "alt-only": {
	name: "Alt Only"
	bin:  %q
	config: {global: "~/.alt"}
	skills: {
		global: {alternatives: ["~/.other/skills", "~/.third/skills"]}
		local:  {alternatives: [".other/skills", ".third/skills"]}
	}
	provider: ["openai"]
}
`, fixtureBinAlpha)
	home := t.TempDir()
	wd := t.TempDir()
	idx := openAgents(t, body,
		WithSearchDirs(binDir(t, fixtureBinAlpha)),
		WithEnvLookup(envFn(home)),
		WithWorkingDir(wd),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	d, err := idx.Agents.Get(context.Background(), "alt-only", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	wantG := filepath.Join(home, ".other", "skills")
	if d.Detection.Skills.Global.Primary.Path != wantG {
		t.Errorf("global.primary = %q, want first alternative %q", d.Detection.Skills.Global.Primary.Path, wantG)
	}
	if d.Detection.Skills.Global.Agents.Path != "" || d.Detection.Skills.Global.Native.Path != "" {
		t.Errorf("global agents/native should be empty, got agents=%q native=%q",
			d.Detection.Skills.Global.Agents.Path, d.Detection.Skills.Global.Native.Path)
	}
	wantL := filepath.Join(wd, ".other", "skills")
	if d.Detection.Skills.Local.Primary.Path != wantL {
		t.Errorf("local.primary = %q, want first alternative %q", d.Detection.Skills.Local.Primary.Path, wantL)
	}
}

func TestGetSkillsPrimaryNativeOverAlternatives(t *testing.T) {
	body := fmt.Sprintf(`
agents: "native-first": {
	name: "Native First"
	bin:  %q
	config: {global: "~/.nf"}
	skills: {
		global: {
			native: "~/.nf/skills"
			alternatives: ["~/.other/skills", "~/.third/skills"]
		}
		local: {
			native: ".nf/skills"
			alternatives: [".other/skills", ".third/skills"]
		}
	}
	provider: ["openai"]
}
`, fixtureBinAlpha)
	home := t.TempDir()
	wd := t.TempDir()
	idx := openAgents(t, body,
		WithSearchDirs(binDir(t, fixtureBinAlpha)),
		WithEnvLookup(envFn(home)),
		WithWorkingDir(wd),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	d, err := idx.Agents.Get(context.Background(), "native-first", AgentGetQuery{Enrich: EnrichNone})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	wantG := filepath.Join(home, ".nf", "skills")
	if d.Detection.Skills.Global.Primary.Path != wantG {
		t.Errorf("global.primary = %q, want native %q", d.Detection.Skills.Global.Primary.Path, wantG)
	}
	if d.Detection.Skills.Global.Agents.Path != "" {
		t.Errorf("global.agents = %q, want empty", d.Detection.Skills.Global.Agents.Path)
	}
	if got := len(d.Detection.Skills.Global.Alternatives); got != 2 {
		t.Errorf("global.alternatives len = %d, want 2", got)
	}
	wantL := filepath.Join(wd, ".nf", "skills")
	if d.Detection.Skills.Local.Primary.Path != wantL {
		t.Errorf("local.primary = %q, want native %q", d.Detection.Skills.Local.Primary.Path, wantL)
	}
	if got := len(d.Detection.Skills.Local.Alternatives); got != 2 {
		t.Errorf("local.alternatives len = %d, want 2", got)
	}
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func TestGetHonoursCancelledContext(t *testing.T) {
	idx := openAgents(t, testCatalog,
		WithSearchDirs(binDir(t, fixtureBinAlpha)),
		WithEnvLookup(envFn(t.TempDir())),
		WithModelsURL(modelsdevtest.MustNotFetch(t)),
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := idx.Agents.List(ctx, AgentQuery{Enrich: EnrichNone})
	if err == nil {
		t.Error("cancelled context should surface an error")
	}
}

func agentIDs(agents []Agent) []string {
	out := make([]string, len(agents))
	for i, a := range agents {
		out[i] = a.ID
	}
	return out
}

func byID(agents []Agent) map[string]Agent {
	m := make(map[string]Agent, len(agents))
	for _, a := range agents {
		m[a.ID] = a
	}
	return m
}

func modelIDs(models []Model) []string {
	out := make([]string, len(models))
	for i, m := range models {
		out[i] = m.ID
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
