package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/p3bot/agentdex/modelsdev"
)

func TestProviderRecordEnvAndPresence(t *testing.T) {
	// Set vars gain "(set)" in the env cell; present map carries bare booleans.
	p := modelsdev.Provider{
		ID:   "acme",
		Name: "Acme",
		Env:  []string{"FOO_KEY", "BAR_KEY"},
		Models: map[string]modelsdev.Model{
			"m1": {ID: "m1"},
			"m2": {ID: "m2"},
		},
	}
	present := map[string]bool{"FOO_KEY": true, "BAR_KEY": false}
	fs, err := providerRecord(p, present).resolve(nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	by := map[string]field{}
	for _, f := range fs {
		by[f.key] = f
	}

	if got, want := by["env"].text, "BAR_KEY, FOO_KEY (set)"; got != want {
		t.Errorf("env cell = %q, want %q", got, want)
	}
	names, ok := by["env"].val.([]string)
	if !ok || len(names) != 2 || names[0] != "BAR_KEY" || names[1] != "FOO_KEY" {
		t.Errorf("env val = %v, want sorted [BAR_KEY FOO_KEY]", by["env"].val)
	}
	pm, ok := by["present"].val.(map[string]bool)
	if !ok || !pm["FOO_KEY"] || pm["BAR_KEY"] {
		t.Errorf("present val = %v, want {FOO_KEY:true BAR_KEY:false}", by["present"].val)
	}
	models, ok := by["models"].val.([]modelsdev.Model)
	if !ok || len(models) != 2 {
		t.Errorf("models val = %v, want a 2-element slice", by["models"].val)
	}
	if by["models"].text != "2" {
		t.Errorf("models cell = %q, want the count 2", by["models"].text)
	}
}

func TestProviderRecordNoEnvBlankCell(t *testing.T) {
	p := modelsdev.Provider{ID: "acme", Name: "Acme"}
	fs, _ := providerRecord(p, nil).resolve(nil)
	for _, f := range fs {
		if f.key == "env" && f.text != "" {
			t.Errorf("env cell for a provider with no declared var = %q, want blank", f.text)
		}
	}
}

func TestProvidersListAllSortedByID(t *testing.T) {
	srv := modelsServer(t, []string{"anthropic", "google", "openai"})
	newScenario(t, srv.URL)

	got := runCLI("--json", "providers", "list")
	if got.code != codeOK {
		t.Fatalf("providers exit = %d, stderr=%q", got.code, got.stderr)
	}
	rows := got.envelope(t).Data.([]any)
	var ids []string
	for _, r := range rows {
		ids = append(ids, r.(map[string]any)["id"].(string))
	}
	want := []string{"anthropic", "google", "openai"}
	if len(ids) != len(want) {
		t.Fatalf("provider ids = %v, want %v", ids, want)
	}
	for i, id := range want {
		if ids[i] != id {
			t.Errorf("provider ids = %v, want sorted %v", ids, want)
		}
	}
}

func TestProvidersFilterNarrows(t *testing.T) {
	// "E" matches google and openai (case-insensitive); matching several lists all of them.
	srv := modelsServer(t, []string{"anthropic", "google", "openai"})
	newScenario(t, srv.URL)

	got := runCLI("--json", "providers", "list", "E")
	if got.code != codeOK {
		t.Fatalf("providers filter exit = %d, stderr=%q", got.code, got.stderr)
	}
	rows := got.envelope(t).Data.([]any)
	var ids []string
	for _, r := range rows {
		ids = append(ids, r.(map[string]any)["id"].(string))
	}
	if len(ids) != 2 || ids[0] != "google" || ids[1] != "openai" {
		t.Errorf("filter %q ids = %v, want [google openai]", "E", ids)
	}
}

func TestProvidersFilterNoMatchIsEmptyExitZero(t *testing.T) {
	// No match is a normal browse outcome, not not-found.
	srv := modelsServer(t, []string{"anthropic"})
	newScenario(t, srv.URL)

	got := runCLI("--json", "providers", "list", "no-such-provider")
	if got.code != codeOK {
		t.Fatalf("no-match filter exit = %d, want 0; stderr=%q", got.code, got.stderr)
	}
	if rows := got.envelope(t).Data.([]any); len(rows) != 0 {
		t.Errorf("no-match filter data = %v, want empty", rows)
	}

	text := runCLI("providers", "list", "no-such-provider")
	if !strings.Contains(text.stdout, `No providers match "no-such-provider".`) {
		t.Errorf("no-match text output missing filter-aware empty-state line:\n%s", text.stdout)
	}
}

func TestProvidersListWarnsOnStaleModels(t *testing.T) {
	// Seed cache, then point at a failing URL with TTL zero so list serves stale fallback.
	good := modelsServer(t, []string{"anthropic", "google"})
	s := newScenario(t, good.URL)

	if got := runCLI("providers", "list"); got.code != codeOK {
		t.Fatalf("warm providers list exit = %d; stderr=%q", got.code, got.stderr)
	}

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)
	s.writeConfig(t, fmt.Sprintf(
		"color: \"never\"\nsearch_dirs: [%q]\nmodels: {\n\turl: %q\n\tttl: \"0s\"\n}\n",
		s.binDir, failing.URL,
	))

	const wantWarn = "models.dev catalog is stale: refetch failed, using the cached copy"

	got := runCLI("--json", "providers", "list")
	if got.code != codeOK {
		t.Fatalf("stale providers list exit = %d, want 0; stderr=%q", got.code, got.stderr)
	}
	if !anyContains(got.envelope(t).Warnings, wantWarn) {
		t.Errorf("stale providers list warnings = %v, want containing %q", got.envelope(t).Warnings, wantWarn)
	}

	text := runCLI("providers", "list")
	if text.code != codeOK {
		t.Fatalf("stale providers text list exit = %d; stderr=%q", text.code, text.stderr)
	}
	if !strings.Contains(text.stderr, wantWarn) {
		t.Errorf("text mode stderr = %q, want containing %q", text.stderr, wantWarn)
	}
}

func TestProvidersJSONModelsIsArrayCellIsCount(t *testing.T) {
	srv := modelsServer(t, []string{"anthropic"})
	newScenario(t, srv.URL)

	got := runCLI("--json", "providers", "list", "anthropic")
	if got.code != codeOK {
		t.Fatalf("providers exit = %d, stderr=%q", got.code, got.stderr)
	}
	row := got.envelope(t).Data.([]any)[0].(map[string]any)
	models, ok := row["models"].([]any)
	if !ok || len(models) != 1 {
		t.Fatalf("models field = %v, want a 1-element JSON array", row["models"])
	}

	// id,models isolates the MODELS cell from any incidental "1" elsewhere in the row.
	text := runCLI("providers", "list", "anthropic", "--fields", "id,models")
	if text.code != codeOK {
		t.Fatalf("providers --fields id,models exit = %d, stderr=%q", text.code, text.stderr)
	}
	if !strings.Contains(text.stdout, "MODELS") {
		t.Errorf("text output missing MODELS column:\n%s", text.stdout)
	}
	cell := ""
	for line := range strings.SplitSeq(text.stdout, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "anthropic") {
			f := strings.Fields(line)
			cell = f[len(f)-1]
		}
	}
	if cell != "1" {
		t.Errorf("MODELS cell = %q, want the model count 1:\n%s", cell, text.stdout)
	}
}

func TestProvidersFieldsSelectionAndValidation(t *testing.T) {
	srv := modelsServer(t, []string{"anthropic"})
	newScenario(t, srv.URL)

	got := runCLI("--json", "providers", "list", "anthropic", "--fields", "id,present")
	if got.code != codeOK {
		t.Fatalf("providers --fields exit = %d, stderr=%q", got.code, got.stderr)
	}
	row := got.envelope(t).Data.([]any)[0].(map[string]any)
	if _, ok := row["present"]; !ok {
		t.Errorf("--fields id,present should carry present: %v", row)
	}
	if _, ok := row["name"]; ok {
		t.Errorf("--fields id,present should not carry name: %v", row)
	}

	// --fields drives text table columns too, not just the JSON payload.
	text := runCLI("providers", "list", "anthropic", "--fields", "id,present")
	if text.code != codeOK {
		t.Fatalf("providers --fields text exit = %d, stderr=%q", text.code, text.stderr)
	}
	if !strings.Contains(text.stdout, "PRESENT") {
		t.Errorf("--fields id,present text output should include the PRESENT column:\n%s", text.stdout)
	}
	for _, col := range []string{"NAME", "ENV", "MODELS"} {
		if strings.Contains(text.stdout, col) {
			t.Errorf("--fields id,present text output should drop the default %s column:\n%s", col, text.stdout)
		}
	}

	bad := runCLI("providers", "list", "anthropic", "--fields", "bogus")
	if bad.code != codeUsage {
		t.Fatalf("unknown --fields exit = %d, want 2; stderr=%q", bad.code, bad.stderr)
	}
}

func TestProvidersUnknownFieldRejectedOnEmptyResult(t *testing.T) {
	// Validation must not depend on result cardinality.
	srv := modelsServer(t, []string{"anthropic"})
	newScenario(t, srv.URL)

	got := runCLI("providers", "list", "no-such-provider", "--fields", "bogus")
	if got.code != codeUsage {
		t.Fatalf("unknown --fields on empty result exit = %d, want 2; stderr=%q", got.code, got.stderr)
	}
}

func TestProvidersTransientWhenUnreachable(t *testing.T) {
	newScenario(t, closedModelsServer(t))

	got := runCLI("providers", "list")
	if got.code != codeTransient {
		t.Fatalf("providers unreachable exit = %d, want 75; stderr=%q", got.code, got.stderr)
	}
}

func TestProvidersSchemaDriftIsConfig(t *testing.T) {
	// Empty top-level providers map is gross structural drift (validateTopLevel).
	srv := modelsServer(t, nil)
	newScenario(t, srv.URL)

	got := runCLI("providers", "list")
	if got.code != codeConfig {
		t.Fatalf("providers schema drift exit = %d, want 78; stderr=%q", got.code, got.stderr)
	}
}

func TestProvidersGetKnown(t *testing.T) {
	// Default get: facts + symmetric env markers + model count (no full table).
	srv := modelsServer(t, []string{"anthropic"})
	newScenario(t, srv.URL)
	// Unset the key so the marker is "(unset)" regardless of host environment.
	unsetEnv(t, "ANTHROPIC_API_KEY")

	got := runCLI("--json", "providers", "get", "anthropic")
	if got.code != codeOK {
		t.Fatalf("providers get exit = %d, stderr=%q", got.code, got.stderr)
	}
	data := got.envelope(t).Data.(map[string]any)
	if data["id"] != "anthropic" {
		t.Errorf("id = %v, want anthropic", data["id"])
	}
	if _, ok := data["present"]; !ok {
		t.Errorf("providers get should carry the present map: %v", data)
	}
	models, ok := data["models"].([]any)
	if !ok || len(models) != 1 {
		t.Errorf("models field = %v, want a 1-element JSON array", data["models"])
	}

	text := runCLI("providers", "get", "anthropic")
	if text.code != codeOK {
		t.Fatalf("providers get text exit = %d, stderr=%q", text.code, text.stderr)
	}
	for _, section := range []string{"Provider", "Provider env", "Models"} {
		if !hasTextSection(text.stdout, section) {
			t.Errorf("providers get text missing %q section:\n%s", section, text.stdout)
		}
	}
	if !strings.Contains(text.stdout, "(unset)") {
		t.Errorf("providers get should use the symmetric (unset) marker:\n%s", text.stdout)
	}
}

func TestProvidersGetUnknownIsNotFound(t *testing.T) {
	srv := modelsServer(t, []string{"anthropic"})
	newScenario(t, srv.URL)

	got := runCLI("providers", "get", "no-such-provider")
	if got.code != codeNotFound {
		t.Fatalf("providers get unknown exit = %d, want 3; stderr=%q", got.code, got.stderr)
	}
}

func TestProvidersGetModelsFillsTable(t *testing.T) {
	srv := modelsServer(t, []string{"anthropic"})
	newScenario(t, srv.URL)

	text := runCLI("providers", "get", "anthropic", "--models")
	if text.code != codeOK {
		t.Fatalf("providers get --models exit = %d, stderr=%q", text.code, text.stderr)
	}
	if !hasTextSection(text.stdout, "Models") {
		t.Errorf("providers get --models should keep the Models section:\n%s", text.stdout)
	}
	if !strings.Contains(text.stdout, "Claude Sonnet") {
		t.Errorf("providers get --models should list the model row:\n%s", text.stdout)
	}
	if !strings.Contains(text.stdout, priceUnitNote) {
		t.Errorf("providers get --models table should carry the price footer:\n%s", text.stdout)
	}
}

func TestProvidersGetTransientWhenUnreachable(t *testing.T) {
	newScenario(t, closedModelsServer(t))

	got := runCLI("providers", "get", "anthropic")
	if got.code != codeTransient {
		t.Fatalf("providers get unreachable exit = %d, want 75; stderr=%q", got.code, got.stderr)
	}
}

func TestProvidersGetSchemaDriftIsConfig(t *testing.T) {
	// Gross structural drift is config (78), not an outage.
	srv := modelsServer(t, nil)
	newScenario(t, srv.URL)

	got := runCLI("providers", "get", "anthropic")
	if got.code != codeConfig {
		t.Fatalf("providers get schema drift exit = %d, want 78; stderr=%q", got.code, got.stderr)
	}
}
