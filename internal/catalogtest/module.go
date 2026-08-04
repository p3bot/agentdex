package catalogtest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// WriteModule materialises a loadable CUE catalog module in a fresh temp
// directory and returns its path. It bundles the repository's catalog/schema.cue
// so one schema governs every fixture; agentsBody is the agents.cue body after
// the package clause. A body the real schema rejects fails at load time as a
// published module would.
func WriteModule(t *testing.T, agentsBody string) string {
	t.Helper()
	dir := t.TempDir()

	schema := repoFile(t, filepath.Join("catalog", "schema.cue"))
	moduleCue := "module: \"github.com/p3bot/agentdex/catalog@v1\"\nlanguage: {\n\tversion: \"v0.16.0\"\n}\n"

	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("catalogtest: mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("catalogtest: write %s: %v", rel, err)
		}
	}

	write(filepath.Join("cue.mod", "module.cue"), moduleCue)
	write("schema.cue", schema)
	write("agents.cue", "package catalog\n\n"+agentsBody+"\n")
	return dir
}

// repoFile reads a repository file resolved relative to this source file, so it
// works from any test package's working directory.
func repoFile(t *testing.T, rel string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("catalogtest: cannot determine source location")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	data, err := os.ReadFile(filepath.Join(repoRoot, rel))
	if err != nil {
		t.Fatalf("catalogtest: read %s: %v", rel, err)
	}
	return string(data)
}
