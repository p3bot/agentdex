package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/p3bot/agentdex/modelsdev"
)

func TestProviderRecordEnvAndPresence(t *testing.T) {
	// Set vars gain "(set)" in the env cell; present map carries bare booleans.
	// Partial presence is still set, not a third state.
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

	if got, ok := by["set"].val.(bool); !ok || !got {
		t.Errorf("set val = %v, want true", by["set"].val)
	}
	if by["set"].text != "set" {
		t.Errorf("set cell = %q, want set", by["set"].text)
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

func TestProviderRecordNoEnvDashCell(t *testing.T) {
	p := modelsdev.Provider{ID: "acme", Name: "Acme"}
	fs, _ := providerRecord(p, nil).resolve(nil)
	for _, f := range fs {
		if f.key == "env" && f.text != "-" {
			t.Errorf("env cell for a provider with no declared var = %q, want -", f.text)
		}
		if f.key == "set" {
			if f.text != "-" {
				t.Errorf("set cell for a provider with no declared var = %q, want -", f.text)
			}
			if f.val != nil {
				t.Errorf("set val for a provider with no declared var = %v, want nil", f.val)
			}
		}
	}
}

func TestProviderSetField(t *testing.T) {
	for _, tc := range []struct {
		name    string
		names   []string
		present map[string]bool
		wantVal any
		wantTxt string
	}{
		{"no names", nil, nil, nil, "-"},
		{"all unset", []string{"A", "B"}, map[string]bool{"A": false, "B": false}, false, "unset"},
		{"all set", []string{"A", "B"}, map[string]bool{"A": true, "B": true}, true, "set"},
		{"partial is set", []string{"A", "B"}, map[string]bool{"A": true, "B": false}, true, "set"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotVal, gotTxt := providerSetField(tc.names, tc.present)
			if gotVal != tc.wantVal || gotTxt != tc.wantTxt {
				t.Errorf("providerSetField = (%v, %q), want (%v, %q)", gotVal, gotTxt, tc.wantVal, tc.wantTxt)
			}
		})
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
	unsetEnv(t, "ANTHROPIC_API_KEY")

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
	if !strings.Contains(text.stdout, "(unset)") {
		t.Errorf("--fields present should keep per-variable (unset) markers:\n%s", text.stdout)
	}
	for _, col := range []string{"NAME", "SET", "ENV", "MODELS"} {
		if strings.Contains(text.stdout, col) {
			t.Errorf("--fields id,present text output should drop the default %s column:\n%s", col, text.stdout)
		}
	}

	bad := runCLI("providers", "list", "anthropic", "--fields", "bogus")
	if bad.code != codeUsage {
		t.Fatalf("unknown --fields exit = %d, want 2; stderr=%q", bad.code, bad.stderr)
	}
}

func TestProvidersListDefaultSetColumn(t *testing.T) {
	for _, tc := range []struct {
		name     string
		setup    func(t *testing.T)
		filter   string
		wantSet  string
		wantJSON any
	}{
		{
			name: "all unset",
			setup: func(t *testing.T) {
				newScenario(t, modelsServer(t, []string{"anthropic"}).URL)
				unsetEnv(t, "ANTHROPIC_API_KEY")
			},
			filter:   "anthropic",
			wantSet:  "unset",
			wantJSON: false,
		},
		{
			name: "all set",
			setup: func(t *testing.T) {
				newScenario(t, modelsServer(t, []string{"anthropic"}).URL)
				t.Setenv("ANTHROPIC_API_KEY", "test")
			},
			filter:   "anthropic",
			wantSet:  "set",
			wantJSON: true,
		},
		{
			name: "partial multi-var is set",
			setup: func(t *testing.T) {
				newScenario(t, multiEnvProviderServer(t))
				t.Setenv("ACME_KEY", "test")
				unsetEnv(t, "ACME_ALT")
			},
			filter:   "acme",
			wantSet:  "set",
			wantJSON: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)

			text := runCLI("providers", "list", tc.filter)
			if text.code != codeOK {
				t.Fatalf("providers list exit = %d, stderr=%q", text.code, text.stderr)
			}
			if !strings.Contains(text.stdout, "SET") {
				t.Errorf("default list missing SET column:\n%s", text.stdout)
			}
			if got := tableCell(t, text.stdout, tc.filter, "set"); got != tc.wantSet {
				t.Errorf("SET cell = %q, want %q:\n%s", got, tc.wantSet, text.stdout)
			}

			js := runCLI("--json", "providers", "list", tc.filter)
			if js.code != codeOK {
				t.Fatalf("providers list --json exit = %d, stderr=%q", js.code, js.stderr)
			}
			row := js.envelope(t).Data.([]any)[0].(map[string]any)
			if row["set"] != tc.wantJSON {
				t.Errorf("JSON set = %v (%T), want %v", row["set"], row["set"], tc.wantJSON)
			}
		})
	}
}

func TestProvidersListDefaultSetColumnContrastsInOneTable(t *testing.T) {
	newScenario(t, modelsServer(t, []string{"anthropic", "openai"}).URL)
	unsetEnv(t, "ANTHROPIC_API_KEY")
	t.Setenv("OPENAI_API_KEY", "test")

	text := runCLI("providers", "list")
	if text.code != codeOK {
		t.Fatalf("providers list exit = %d, stderr=%q", text.code, text.stderr)
	}
	if got, want := tableCell(t, text.stdout, "anthropic", "set"), "unset"; got != want {
		t.Errorf("anthropic SET = %q, want %q:\n%s", got, want, text.stdout)
	}
	if got, want := tableCell(t, text.stdout, "openai", "set"), "set"; got != want {
		t.Errorf("openai SET = %q, want %q:\n%s", got, want, text.stdout)
	}
}

func TestProvidersListOrderBySet(t *testing.T) {
	newScenario(t, modelsServer(t, []string{"anthropic", "openai"}).URL)
	unsetEnv(t, "ANTHROPIC_API_KEY")
	t.Setenv("OPENAI_API_KEY", "test")

	got := runCLI("--json", "providers", "list", "--order-by", "set")
	if got.code != codeOK {
		t.Fatalf("providers list --order-by set exit = %d, stderr=%q", got.code, got.stderr)
	}
	rows := got.envelope(t).Data.([]any)
	order := make([]string, len(rows))
	for i, row := range rows {
		order[i], _ = row.(map[string]any)["id"].(string)
	}
	assertOrder(t, order, []string{"anthropic", "openai"})

	text := runCLI("providers", "list", "--order-by", "set")
	if text.code != codeOK {
		t.Fatalf("providers list --order-by set text exit = %d, stderr=%q", text.code, text.stderr)
	}
	if got := strings.Index(text.stdout, "SET"); got < 0 || strings.Index(text.stdout, "ID") < got {
		t.Errorf("--order-by set should lead with the SET column:\n%s", text.stdout)
	}
}

func TestProvidersListHelpDescribesSetColumn(t *testing.T) {
	got := runCLI("providers", "list", "--help")
	if got.code != codeOK {
		t.Fatalf("providers list --help exit = %d, stderr=%q", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "The set column shows set") {
		t.Errorf("Long should name the set column and its set/unset cells:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, "--fields present") {
		t.Errorf("Long should point at --fields present for per-variable status:\n%s", got.stdout)
	}
	if strings.Contains(got.stdout, "API-key environment variables and whether they are set") {
		t.Errorf("Long still oversells ENV as full per-variable status:\n%s", got.stdout)
	}
}

func multiEnvProviderServer(t *testing.T) string {
	t.Helper()
	cat := modelsdev.Catalog{
		Models: map[string]modelsdev.Model{
			"acme/m1": {ID: "acme/m1", Name: "M1", Limit: modelsdev.Limit{Context: 1}},
		},
		Providers: map[string]modelsdev.Provider{
			"acme": {
				ID:   "acme",
				Name: "Acme",
				Env:  []string{"ACME_KEY", "ACME_ALT"},
				Models: map[string]modelsdev.Model{
					"m1": {ID: "m1", Name: "M1", Limit: modelsdev.Limit{Context: 1}},
				},
			},
		},
	}
	data, err := json.Marshal(cat)
	if err != nil {
		t.Fatalf("marshal multi-env catalog: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func tableCell(t *testing.T, stdout, rowID, col string) string {
	t.Helper()
	want := strings.ToUpper(col)
	var header string
	var rows []string
	for line := range strings.SplitSeq(stdout, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if header == "" {
			header = line
			continue
		}
		rows = append(rows, line)
	}
	if header == "" {
		t.Fatalf("no table header in:\n%s", stdout)
	}
	cols := strings.Fields(header)
	idx := -1
	for i, c := range cols {
		if c == want {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("column %q not in header %q", want, header)
	}
	start := strings.Index(header, want)
	end := len(header)
	if idx+1 < len(cols) {
		end = strings.Index(header, cols[idx+1])
	}
	for _, row := range rows {
		if !strings.HasPrefix(strings.TrimSpace(row), rowID) {
			continue
		}
		if start >= len(row) {
			t.Fatalf("row too short for column %s:\n%s", want, row)
		}
		cellEnd := min(end, len(row))
		return strings.TrimSpace(row[start:cellEnd])
	}
	t.Fatalf("no row starting with %q in:\n%s", rowID, stdout)
	return ""
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
	if !strings.Contains(text.stdout, "1 model") {
		t.Errorf("providers get default Models section should name the count:\n%s", text.stdout)
	}
	if !strings.Contains(text.stdout, "--models") {
		t.Errorf("providers get default Models section should hint --models:\n%s", text.stdout)
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

func TestProvidersGetFieldsModelsFillsTable(t *testing.T) {
	srv := modelsServer(t, []string{"anthropic"})
	newScenario(t, srv.URL)

	text := runCLI("providers", "get", "anthropic", "--fields", "models")
	if text.code != codeOK {
		t.Fatalf("providers get --fields models exit = %d, stderr=%q", text.code, text.stderr)
	}
	if strings.TrimSpace(text.stdout) == "1" {
		t.Errorf("--fields models text should not be a bare count:\n%s", text.stdout)
	}
	if !strings.Contains(text.stdout, "ID") || !strings.Contains(text.stdout, "NAME") {
		t.Errorf("--fields models text should render the model table:\n%s", text.stdout)
	}
	if !strings.Contains(text.stdout, priceUnitNote) {
		t.Errorf("--fields models text should carry the price footer:\n%s", text.stdout)
	}

	// Multi-field: the non-models field keeps its key (not bare), then models: table.
	mixed := runCLI("providers", "get", "anthropic", "--fields", "id,models")
	if mixed.code != codeOK {
		t.Fatalf("providers get --fields id,models exit = %d, stderr=%q", mixed.code, mixed.stderr)
	}
	if !strings.Contains(mixed.stdout, "id: anthropic") {
		t.Errorf("--fields id,models should key the id line:\n%s", mixed.stdout)
	}
	if !strings.Contains(mixed.stdout, "models:") {
		t.Errorf("--fields id,models should label the models block:\n%s", mixed.stdout)
	}
	if !strings.Contains(mixed.stdout, "ID") || !strings.Contains(mixed.stdout, priceUnitNote) {
		t.Errorf("--fields id,models should still render the model table:\n%s", mixed.stdout)
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
