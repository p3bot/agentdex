// Package catalogtest provides a stub registry and fixture helpers shared by
// the catalog loader tests and the root-package mapping tests. It lets the
// loader run its real load, validate, and cache logic against an on-disk
// fixture module with no registry, and supports injecting resolve/fetch
// failures.
package catalogtest

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// StubRegistry implements catalog.Registry. OnResolve and OnFetch supply canned
// outcomes; call counts are per module path so tests can assert isolation.
type StubRegistry struct {
	OnResolve func(ctx context.Context, modulePath string) (string, error)
	OnFetch   func(ctx context.Context, modulePath string) (string, error)

	mu           sync.Mutex
	resolveCalls map[string]int
	fetchCalls   map[string]int
}

// Resolves any module path to version and fetches from sourceDir.
// Tests that care about per-path behaviour set handlers directly.
func Serve(version, sourceDir string) *StubRegistry {
	return &StubRegistry{
		OnResolve: func(context.Context, string) (string, error) { return version, nil },
		OnFetch:   func(context.Context, string) (string, error) { return sourceDir, nil },
	}
}

func (s *StubRegistry) ResolveLatestVersion(ctx context.Context, modulePath string) (string, error) {
	s.record(&s.resolveCalls, modulePath)
	return s.OnResolve(ctx, modulePath)
}

func (s *StubRegistry) Fetch(ctx context.Context, modulePath string) (string, error) {
	s.record(&s.fetchCalls, modulePath)
	return s.OnFetch(ctx, modulePath)
}

func (s *StubRegistry) ResolveCalls(modulePath string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resolveCalls[modulePath]
}

// FetchCalls returns how many times a canonical form of the given path was
// fetched. Fetches are recorded under module@version, so a base path matches
// its canonical fetches via the "base." prefix as well as an exact key.
func (s *StubRegistry) FetchCalls(modulePath string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	total := 0
	for key, n := range s.fetchCalls {
		if key == modulePath || strings.HasPrefix(key, modulePath+".") {
			total += n
		}
	}
	return total
}

func (s *StubRegistry) TotalResolveCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	total := 0
	for _, n := range s.resolveCalls {
		total += n
	}
	return total
}

func (s *StubRegistry) record(m *map[string]int, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if *m == nil {
		*m = make(map[string]int)
	}
	(*m)[key]++
}

// Absolute fixture path under testdata, relative to this source file so any test package can use it.
func FixtureDir(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("catalogtest: cannot determine source location")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	return filepath.Join(repoRoot, "testdata", name)
}
