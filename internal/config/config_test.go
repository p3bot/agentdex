package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/p3bot/agentdex"
	"github.com/p3bot/agentdex/internal/catalogtest"
	"github.com/p3bot/agentdex/internal/modelsdevtest"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.cue")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestOptionsCatalogDirOmitsModule(t *testing.T) {
	// catalog.dir set while CatalogModule still carries the default: Options must
	// not pass both, or Open rejects the pair.
	dir := catalogtest.WriteModule(t, `agents: "alpha-cli": {
	name: "Alpha CLI"
	bin:  "alpha"
	config: {global: "~/.alpha"}
	provider: ["anthropic"]
}`)
	c := &Config{
		CatalogModule: "github.com/p3bot/agentdex/catalog@v1",
		CatalogDir:    dir,
		CatalogTTL:    DefaultTTL,
		ModelsTTL:     DefaultTTL,
	}
	opts := append(c.Options(Flags{}), agentdex.WithModelsURL(modelsdevtest.MustNotFetch(t)))
	if _, err := agentdex.Open(opts...); err != nil {
		t.Fatalf("Open via Options with catalog.dir: %v", err)
	}
}

func TestLoadMissingFileIsEmptyDefaults(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "absent.cue"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if got.CatalogModule != "github.com/p3bot/agentdex/catalog@v1" {
		t.Errorf("CatalogModule = %q, want the default module path", got.CatalogModule)
	}
	if got.Color != "auto" {
		t.Errorf("Color = %q, want default auto", got.Color)
	}
	if got.CatalogTTL != DefaultTTL || got.ModelsTTL != DefaultTTL {
		t.Errorf("TTLs = %v/%v, want default %v", got.CatalogTTL, got.ModelsTTL, DefaultTTL)
	}
}

func TestLoadFieldsAndTTLResolution(t *testing.T) {
	path := writeConfig(t, `
cache_ttl: "1h"
catalog: ttl: "2h"
catalog: dir: "./local-catalog"
models: url: "https://mirror.example/catalog.json"
search_dirs: ["/opt/bin", "/usr/local/bin"]
bin_paths: "claude-code": "/custom/claude"
color: "never"
`)
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.CatalogTTL != 2*time.Hour {
		t.Errorf("CatalogTTL = %v, want section ttl 2h", got.CatalogTTL)
	}
	if got.CatalogDir != "./local-catalog" {
		t.Errorf("CatalogDir = %q, want ./local-catalog", got.CatalogDir)
	}
	if got.ModelsTTL != time.Hour {
		t.Errorf("ModelsTTL = %v, want cache_ttl fallback 1h", got.ModelsTTL)
	}
	if got.ModelsURL != "https://mirror.example/catalog.json" {
		t.Errorf("ModelsURL = %q", got.ModelsURL)
	}
	if got.Color != "never" {
		t.Errorf("Color = %q, want never", got.Color)
	}
	if len(got.SearchDirs) != 2 || got.BinPaths["claude-code"] != "/custom/claude" {
		t.Errorf("collection fields decoded wrong: %+v", got)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	path := writeConfig(t, `unknown_field: true`)
	_, err := Load(path)
	if !errors.Is(err, ErrConfig) {
		t.Fatalf("Load unknown field err = %v, want ErrConfig", err)
	}
}

func TestLoadRejectsRemovedEnrichModels(t *testing.T) {
	// enrich_models was removed; leftover keys fail closed-schema validation.
	path := writeConfig(t, `enrich_models: false`)
	_, err := Load(path)
	if !errors.Is(err, ErrConfig) {
		t.Fatalf("Load enrich_models err = %v, want ErrConfig", err)
	}
}

func TestLoadRejectsRemovedDisabledAgents(t *testing.T) {
	// disabled_agents was removed; leftover keys must fail closed rather than
	// silently ignore a key that no longer does anything.
	path := writeConfig(t, `disabled_agents: ["foo"]`)
	_, err := Load(path)
	if !errors.Is(err, ErrConfig) {
		t.Fatalf("Load disabled_agents err = %v, want ErrConfig", err)
	}
}

func TestLoadRejectsBadType(t *testing.T) {
	path := writeConfig(t, `color: "purple"`)
	_, err := Load(path)
	if !errors.Is(err, ErrConfig) {
		t.Fatalf("Load bad enum err = %v, want ErrConfig", err)
	}
}

func TestLoadRejectsBadDuration(t *testing.T) {
	path := writeConfig(t, `cache_ttl: "not-a-duration"`)
	_, err := Load(path)
	if !errors.Is(err, ErrConfig) {
		t.Fatalf("Load bad duration err = %v, want ErrConfig", err)
	}
}

func TestLoadRejectsSyntaxError(t *testing.T) {
	path := writeConfig(t, `color: "never`)
	_, err := Load(path)
	if !errors.Is(err, ErrConfig) {
		t.Fatalf("Load syntax error err = %v, want ErrConfig", err)
	}
}

func TestPathXDGAndHomeFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg-config")
	if got := Path(); got != filepath.Join("/xdg-config", "agentdex", "config.cue") {
		t.Errorf("Path with XDG_CONFIG_HOME = %q", got)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/tester")
	if got := Path(); got != filepath.Join("/home/tester", ".config", "agentdex", "config.cue") {
		t.Errorf("Path with HOME fallback = %q", got)
	}
}

func TestOptionsMergesFlagsOverConfig(t *testing.T) {
	// mergeSlices: config order, then flags, exact dups dropped.
	if got := mergeSlices([]string{"/config/bin", "/shared"}, []string{"/shared", "/flag/bin"}); !slices.Equal(got, []string{"/config/bin", "/shared", "/flag/bin"}) {
		t.Errorf("mergeSlices = %v, want config then flags with /shared once", got)
	}

	// mergeBinPaths: flags win on id collision; config-only ids remain.
	merged := mergeBinPaths(
		map[string]string{"alpha-cli": "/config/alpha", "beta-tool": "/config/beta"},
		map[string]string{"alpha-cli": "/flag/alpha"},
	)
	if merged["alpha-cli"] != "/flag/alpha" {
		t.Errorf("mergeBinPaths alpha = %q, want flag win /flag/alpha", merged["alpha-cli"])
	}
	if merged["beta-tool"] != "/config/beta" {
		t.Errorf("mergeBinPaths beta = %q, want config /config/beta", merged["beta-tool"])
	}
	if mergeBinPaths(nil, nil) != nil {
		t.Error("mergeBinPaths(nil, nil) should be nil")
	}

	// Options emits WithBinPaths / WithSearchDirs / WithModelsURL for a full config.
	dir := catalogtest.WriteModule(t, `agents: "alpha-cli": {
	name: "Alpha CLI"
	bin:  "alpha"
	config: {global: "~/.alpha"}
	provider: ["anthropic"]
}`)
	c := &Config{
		CatalogDir: dir,
		CatalogTTL: DefaultTTL,
		ModelsTTL:  DefaultTTL,
		ModelsURL:  "https://mirror.example/catalog.json",
		SearchDirs: []string{"/config/bin"},
		BinPaths:   map[string]string{"alpha-cli": "/config/alpha"},
	}
	flags := Flags{
		SearchDirs: []string{"/flag/bin"},
		BinPaths:   map[string]string{"alpha-cli": "/flag/alpha"},
	}
	opts := append(c.Options(flags), agentdex.WithModelsURL(modelsdevtest.MustNotFetch(t)), agentdex.WithLookPath(func(string) (string, error) {
		return "", os.ErrNotExist
	}))
	// ModelsURL in Config is overridden by the trailing WithModelsURL for offline Open;
	// still assert Open accepts the Options-built slice (CatalogDir, TTLs, bin/search merge).
	idx, err := agentdex.Open(opts...)
	if err != nil {
		t.Fatalf("Open via Options: %v", err)
	}
	// Flag bin path is non-existent: override is sole candidate → not Found (no fallthrough).
	d, err := idx.Agents.Get(t.Context(), "alpha-cli", agentdex.AgentGetQuery{Enrich: agentdex.EnrichNone})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if d.Detection.Found {
		t.Errorf("Found via non-existent flag bin path, BinaryPath=%q", d.Detection.BinaryPath)
	}
}
