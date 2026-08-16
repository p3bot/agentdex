package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"cuelang.org/go/mod/modcache"
	"cuelang.org/go/mod/modregistrytest"

	"github.com/p3bot/agentdex/internal/catalogtest"
	"github.com/p3bot/agentdex/internal/modelsdevtest"
)

type result struct {
	stdout string
	stderr string
	code   int
}

func (r result) envelope(t *testing.T) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal([]byte(r.stdout), &env); err != nil {
		t.Fatalf("decode envelope from %q: %v", r.stdout, err)
	}
	return env
}

func rawEnvelope(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("decode raw envelope from %q: %v", stdout, err)
	}
	return m
}

func assertEmptyWarnings(t *testing.T, m map[string]any) {
	t.Helper()
	ws, ok := m["warnings"]
	if !ok {
		t.Errorf("envelope should include warnings")
		return
	}
	arr, ok := ws.([]any)
	if !ok || len(arr) != 0 {
		t.Errorf("envelope with no warnings should be [], got %v", ws)
	}
}

func assertErrorEnvelope(t *testing.T, r result, wantCode int) map[string]any {
	t.Helper()
	if r.code != wantCode {
		t.Fatalf("exit = %d, want %d; stderr=%q stdout=%q", r.code, wantCode, r.stderr, r.stdout)
	}
	if r.stderr != "" {
		t.Errorf("stderr must be empty under --json: %q", r.stderr)
	}
	m := rawEnvelope(t, r.stdout)
	if m["status"] != "error" {
		t.Errorf("status = %v, want error", m["status"])
	}
	if msg, ok := m["error"].(string); !ok || msg == "" {
		t.Errorf("error = %v, want a non-empty message", m["error"])
	}
	if _, ok := m["data"]; ok {
		t.Errorf("failure envelope should omit data: %v", m["data"])
	}
	assertEmptyWarnings(t, m)
	return m
}

func runCLI(args ...string) result {
	root := NewRootCommand()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	code := execute(root, args)
	return result{stdout: out.String(), stderr: errb.String(), code: code}
}

// scenario is the per-test world: temp XDG dirs, fake binaries, local catalog registry,
// optional models.dev server, wired through config.cue.
type scenario struct {
	home          string
	binDir        string
	configDir     string
	closeRegistry func() // shuts down the in-process catalog registry mid-test
}

// newScenario stands up an isolated world. Empty modelsURL wires a local server with
// every fixture provider so enrichment stays deterministic. bins lists which fixture
// binaries to install (omit to leave an agent not installed). PATH is restricted to
// binDir so host tools cannot satisfy fixture binary names via LookPath.
func newScenario(t *testing.T, modelsURL string, bins ...string) *scenario {
	t.Helper()
	if modelsURL == "" {
		modelsURL = modelsServer(t, []string{"anthropic", "openai", "google"}).URL
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	closeRegistry := startCatalogRegistry(t)

	binDir := filepath.Join(home, "bin")
	mustMkdir(t, binDir)
	// Isolate PATH before installing fakes so LookPath only sees this dir. Host
	// tools (e.g. git-delta as "delta") must not mark an uninstalled fixture agent Found.
	t.Setenv("PATH", binDir)
	for _, name := range bins {
		installFakeBin(t, binDir, name)
	}

	configDir := filepath.Join(home, ".config", "agentdex")
	mustMkdir(t, configDir)
	var b strings.Builder
	b.WriteString("color: \"never\"\n")
	fmt.Fprintf(&b, "search_dirs: [%q]\n", binDir)
	if modelsURL != "" {
		fmt.Fprintf(&b, "models: url: %q\n", modelsURL)
	}
	writeFile(t, filepath.Join(configDir, "config.cue"), b.String())

	return &scenario{home: home, binDir: binDir, configDir: configDir, closeRegistry: closeRegistry}
}

func (s *scenario) writeConfig(t *testing.T, body string) {
	t.Helper()
	writeFile(t, filepath.Join(s.configDir, "config.cue"), body)
}

func installFakeBin(t *testing.T, dir, agentID string) {
	t.Helper()
	path := filepath.Join(dir, catalogtest.FixtureBin(t, agentID))
	writeFile(t, path, "#!/bin/sh\necho \"v1.0.0\"\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod fake bin: %v", err)
	}
}

func installCountingBin(t *testing.T, dir, agentID, counterPath string) {
	t.Helper()
	path := filepath.Join(dir, catalogtest.FixtureBin(t, agentID))
	writeFile(t, path, fmt.Sprintf("#!/bin/sh\necho run >> %q\necho \"v1.0.0\"\n", counterPath))
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod counting bin: %v", err)
	}
}

func probeCount(t *testing.T, counterPath string) int {
	t.Helper()
	data, err := os.ReadFile(counterPath)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatalf("read probe counter: %v", err)
	}
	return strings.Count(string(data), "\n")
}

// modelsServer / closedModelsServer keep call sites short; fixtures live in modelsdevtest.
func modelsServer(t *testing.T, present []string, malformed ...string) *httptest.Server {
	return modelsdevtest.Server(t, present, malformed...)
}

func closedModelsServer(t *testing.T) string {
	return modelsdevtest.Closed(t)
}

// startCatalogRegistry publishes the fixture catalog to an in-process OCI registry
// and points CUE_REGISTRY/CUE_CACHE_DIR at it. The closer can take the registry offline mid-run.
func startCatalogRegistry(t *testing.T) func() {
	t.Helper()
	dir := catalogtest.FixtureDir(t, "catalog-valid")
	const moduleDir = "github.com_p3bot_agentdex_catalog_v1.0.0"

	fsys := fstest.MapFS{}
	for _, rel := range []string{"cue.mod/module.cue", "schema.cue", "agents.cue"} {
		data, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatalf("read fixture %s: %v", rel, err)
		}
		fsys[path.Join(moduleDir, rel)] = &fstest.MapFile{Data: data}
	}
	reg, err := modregistrytest.New(fsys, "")
	if err != nil {
		t.Fatalf("start local registry: %v", err)
	}
	var once sync.Once
	closeReg := func() { once.Do(reg.Close) }
	t.Cleanup(closeReg)

	t.Setenv("CUE_REGISTRY", reg.Host()+"+insecure")
	t.Setenv("CUE_CACHE_DIR", cueCacheDir(t))
	return closeReg
}

func cueCacheDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "agentdex-cli-cue-cache")
	if err != nil {
		t.Fatalf("create cue cache dir: %v", err)
	}
	t.Cleanup(func() { _ = modcache.RemoveAll(dir) })
	return dir
}

// unsetEnv guarantees name is absent for the test; testing has no Unsetenv.
func unsetEnv(t *testing.T, name string) {
	t.Helper()
	orig, had := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("unset %s: %v", name, err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(name, orig)
		}
	})
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
