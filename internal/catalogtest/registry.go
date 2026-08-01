package catalogtest

import (
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"

	"cuelang.org/go/mod/modcache"
	"cuelang.org/go/mod/modregistrytest"
)

// StartRegistry publishes the catalog-valid fixture to an in-process OCI
// registry and points CUE_REGISTRY and CUE_CACHE_DIR at it so the production
// loader resolves …/catalog@v1 (to v1.0.0) offline. Returns the registry and a
// once-guarded close (also t.Cleanup) so a test can take the registry offline
// mid-run without double-close.
func StartRegistry(t *testing.T) (*modregistrytest.Registry, func()) {
	t.Helper()
	dir := FixtureDir(t, "catalog-valid")
	const moduleDir = "github.com_start-cli_agentdex_catalog_v1.0.0"

	fsys := fstest.MapFS{}
	for _, rel := range []string{"cue.mod/module.cue", "schema.cue", "agents.cue"} {
		data, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatalf("catalogtest: read fixture %s: %v", rel, err)
		}
		fsys[path.Join(moduleDir, rel)] = &fstest.MapFile{Data: data}
	}

	reg, err := modregistrytest.New(fsys, "")
	if err != nil {
		t.Fatalf("catalogtest: start local registry: %v", err)
	}
	var once sync.Once
	closeReg := func() { once.Do(reg.Close) }
	t.Cleanup(closeReg)

	t.Setenv("CUE_REGISTRY", reg.Host()+"+insecure")
	t.Setenv("CUE_CACHE_DIR", CueCacheDir(t))
	return reg, closeReg
}

// CueCacheDir returns a fresh CUE content-cache directory. CUE writes extracted
// modules read-only, so t.TempDir's RemoveAll cannot unlink them;
// modcache.RemoveAll handles the read-only tree.
func CueCacheDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "agentdex-cue-cache")
	if err != nil {
		t.Fatalf("catalogtest: create cue cache dir: %v", err)
	}
	t.Cleanup(func() { _ = modcache.RemoveAll(dir) })
	return dir
}
