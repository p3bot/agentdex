package cli

import (
	"slices"
	"strings"
	"testing"
)

func TestListDetectsInstalledAgents(t *testing.T) {
	newScenario(t, "", "alpha-cli", "beta-tool", "gamma-agent")

	got := runCLI("agents", "list", "--installed")
	if got.code != codeOK {
		t.Fatalf("list exit = %d, stderr=%q", got.code, got.stderr)
	}
	for _, id := range []string{"alpha-cli", "beta-tool", "gamma-agent"} {
		if !strings.Contains(got.stdout, id) {
			t.Errorf("list output missing %q:\n%s", id, got.stdout)
		}
	}
}

func TestListFilterNarrowsByIDAndName(t *testing.T) {
	// Positional filter is browse narrowing over id and name, not a selector.
	newScenario(t, "", "alpha-cli", "beta-tool", "gamma-agent")

	got := runCLI("--json", "agents", "list", "alpha")
	if got.code != codeOK {
		t.Fatalf("list filter exit = %d, stderr=%q", got.code, got.stderr)
	}
	rows := got.envelope(t).Data.([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["id"] != "alpha-cli" {
		t.Errorf("filter %q rows = %v, want just alpha-cli", "alpha", rows)
	}

	byName := runCLI("--json", "agents", "list", "TOOL")
	if byName.code != codeOK {
		t.Fatalf("list name filter exit = %d, stderr=%q", byName.code, byName.stderr)
	}
	rows = byName.envelope(t).Data.([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["id"] != "beta-tool" {
		t.Errorf("name filter %q rows = %v, want just beta-tool", "TOOL", rows)
	}
}

func TestListFilterNoMatchIsEmptyExitZero(t *testing.T) {
	// No match is a normal browse outcome, not not-found.
	newScenario(t, "", "alpha-cli")

	got := runCLI("--json", "agents", "list", "no-such-agent")
	if got.code != codeOK {
		t.Fatalf("no-match filter exit = %d, want 0; stderr=%q", got.code, got.stderr)
	}
	if rows := got.envelope(t).Data.([]any); len(rows) != 0 {
		t.Errorf("no-match filter data = %v, want empty", rows)
	}

	text := runCLI("agents", "list", "no-such-agent")
	if !strings.Contains(text.stdout, `No agents match "no-such-agent".`) {
		t.Errorf("no-match text output missing filter-aware empty-state line:\n%s", text.stdout)
	}
}

func TestListJSONEnvelope(t *testing.T) {
	newScenario(t, "", "alpha-cli")

	got := runCLI("--json", "agents", "list", "--installed")
	if got.code != codeOK {
		t.Fatalf("list exit = %d, stderr=%q", got.code, got.stderr)
	}
	env := got.envelope(t)
	if env.Status != "ok" {
		t.Errorf("status = %q, want ok", env.Status)
	}
	rows, ok := env.Data.([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("data = %#v, want one row", env.Data)
	}
	row := rows[0].(map[string]any)
	if row["id"] != "alpha-cli" {
		t.Errorf("row id = %v, want alpha-cli", row["id"])
	}
}

func TestListUnknownFieldRejectedRegardlessOfCardinality(t *testing.T) {
	// --fields validation is a command property, not dependent on row count.
	for _, tc := range []struct {
		name string
		bins []string
	}{
		{"empty result set", nil},
		{"non-empty result set", []string{"alpha-cli"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			newScenario(t, "", tc.bins...)
			got := runCLI("agents", "list", "--fields", "bogus")
			if got.code != codeUsage {
				t.Fatalf("list --fields bogus exit = %d, want 2; stderr=%q", got.code, got.stderr)
			}
		})
	}
}

func TestListValidButAbsentFieldResolvesBlank(t *testing.T) {
	// provider_env is declared but only populated by get; JSON val stays blank, text is "-".
	newScenario(t, "", "alpha-cli")

	got := runCLI("--json", "agents", "list", "--fields", "id,provider_env")
	if got.code != codeOK {
		t.Fatalf("list --fields id,provider_env exit = %d, stderr=%q", got.code, got.stderr)
	}
	row := got.envelope(t).Data.([]any)[0].(map[string]any)
	if v, ok := row["provider_env"]; !ok || v != "" {
		t.Errorf("provider_env = %v (present=%t), want present and blank", v, ok)
	}

	text := runCLI("agents", "list", "--fields", "id,provider_env")
	if text.code != codeOK {
		t.Fatalf("list --fields id,provider_env text exit = %d, stderr=%q", text.code, text.stderr)
	}
	// Parse cells: kebab-case ids contain "-", so a bare Contains("-") never fails.
	cells := textRowCells(t, text.stdout, "alpha-cli")
	if len(cells) != 2 || cells[1] != "-" {
		t.Errorf("id,provider_env cells = %v, want [alpha-cli -]:\n%s", cells, text.stdout)
	}
}

func TestListEmptyProvidersIsDashNotBlank(t *testing.T) {
	// Agnostic / no-home-provider rows use text "-" for providers, matching models N/A.
	newScenario(t, "", "delta-agent")

	got := runCLI("agents", "list", "--fields", "id,providers,models")
	if got.code != codeOK {
		t.Fatalf("list exit = %d, stderr=%q", got.code, got.stderr)
	}
	// Three columns: blank providers would collapse the middle cell away.
	cells := textRowCells(t, got.stdout, "delta-agent")
	if len(cells) != 3 || cells[1] != "-" || cells[2] != "-" {
		t.Errorf("delta-agent id,providers,models cells = %v, want [delta-agent - -]:\n%s", cells, got.stdout)
	}
}

func TestListDefaultColumns(t *testing.T) {
	// Default text columns omit config_dir; --fields is the way to add it.
	newScenario(t, "", "alpha-cli")

	plain := runCLI("agents", "list")
	if plain.code != codeOK {
		t.Fatalf("list exit = %d, stderr=%q", plain.code, plain.stderr)
	}
	want := []string{"ID", "NAME", "PROVIDERS", "MODELS", "BIN"}
	if got := tableHeader(t, plain.stdout); !slices.Equal(got, want) {
		t.Errorf("default headers = %v, want %v:\n%s", got, want, plain.stdout)
	}

	wider := runCLI("agents", "list", "--fields", "id,name,config_dir,bin")
	if wider.code != codeOK {
		t.Fatalf("list --fields exit = %d, stderr=%q", wider.code, wider.stderr)
	}
	if !strings.Contains(wider.stdout, "CONFIG_DIR") {
		t.Errorf("list --fields should show the config_dir column:\n%s", wider.stdout)
	}
}

func TestListDefaultIncludesMissingAgents(t *testing.T) {
	// Default is the whole catalog: missing agents trail with "missing" bin text.
	// JSON uses found:false and blank bin, never the "missing" marker.
	newScenario(t, "", "beta-tool")

	installed := runCLI("agents", "list", "--installed")
	if installed.code != codeOK {
		t.Fatalf("list --installed exit = %d, stderr=%q", installed.code, installed.stderr)
	}
	if strings.Contains(installed.stdout, "alpha-cli") || strings.Contains(installed.stdout, "missing") {
		t.Errorf("list --installed should omit missing agents:\n%s", installed.stdout)
	}

	all := runCLI("agents", "list")
	if all.code != codeOK {
		t.Fatalf("list exit = %d, stderr=%q", all.code, all.stderr)
	}
	for _, want := range []string{"alpha-cli", "beta-tool", "gamma-agent", "delta-agent", "missing"} {
		if !strings.Contains(all.stdout, want) {
			t.Errorf("default list missing %q:\n%s", want, all.stdout)
		}
	}
	if strings.Index(all.stdout, "beta-tool") > strings.Index(all.stdout, "alpha-cli") {
		t.Errorf("default list should order detected agents first:\n%s", all.stdout)
	}

	got := runCLI("--json", "agents", "list")
	if got.code != codeOK {
		t.Fatalf("list --json exit = %d, stderr=%q", got.code, got.stderr)
	}
	rows := got.envelope(t).Data.([]any)
	if len(rows) != 4 {
		t.Fatalf("default list rows = %d, want 4", len(rows))
	}
	byID := map[string]map[string]any{}
	for _, r := range rows {
		row := r.(map[string]any)
		byID[row["id"].(string)] = row
	}
	if found, _ := byID["beta-tool"]["found"].(bool); !found {
		t.Errorf("beta-tool found = %v, want true", byID["beta-tool"]["found"])
	}
	if found, _ := byID["alpha-cli"]["found"].(bool); found {
		t.Errorf("alpha-cli found = %v, want false", byID["alpha-cli"]["found"])
	}
	if bin := byID["alpha-cli"]["bin"]; bin != "" {
		t.Errorf("missing agent bin = %v, want blank", bin)
	}
	// Agnostic without --provider: lists with models null, not a degraded empty list.
	if _, ok := byID["delta-agent"]; !ok {
		t.Fatalf("default list omitted delta-agent:\n%s", got.stdout)
	}
	if models := byID["delta-agent"]["models"]; models != nil {
		t.Errorf("agnostic delta-agent models = %v, want null", models)
	}
}

func TestListShowsModelsColumn(t *testing.T) {
	srv := modelsServer(t, []string{"anthropic"})
	newScenario(t, srv.URL, "alpha-cli")

	got := runCLI("agents", "list")
	if got.code != codeOK {
		t.Fatalf("list exit = %d, stderr=%q", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "MODELS") {
		t.Errorf("list missing the models column:\n%s", got.stdout)
	}
	pi, mi, bi := strings.Index(got.stdout, "PROVIDERS"), strings.Index(got.stdout, "MODELS"), strings.Index(got.stdout, "BIN")
	if pi < 0 || pi >= mi || mi >= bi {
		t.Errorf("columns out of order, want PROVIDERS < MODELS < BIN:\n%s", got.stdout)
	}

	j := runCLI("--json", "agents", "list")
	row := j.envelope(t).Data.([]any)[0].(map[string]any)
	models, ok := row["models"].([]any)
	if !ok || len(models) != 1 {
		t.Errorf("list --json should carry one enriched model for alpha-cli: %v", row["models"])
	}
}

func TestListDegradesWhenModelsUnreachable(t *testing.T) {
	// Outage with no cache degrades to zero count + warning, not a failed listing.
	newScenario(t, closedModelsServer(t), "alpha-cli")

	got := runCLI("agents", "list")
	if got.code != codeOK {
		t.Fatalf("list with unreachable models.dev exit = %d, want 0; stderr=%q", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "MODELS") {
		t.Errorf("degraded list should still show the models column:\n%s", got.stdout)
	}

	// Degraded JSON carries [] (not null) to match the "0" cell and stay scripting-safe.
	j := runCLI("--json", "agents", "list")
	env := j.envelope(t)
	if !anyContains(env.Warnings, "unreachable") {
		t.Errorf("degraded list should warn that model counts are unavailable: %v", env.Warnings)
	}
	row := env.Data.([]any)[0].(map[string]any)
	models, ok := row["models"].([]any)
	if !ok || len(models) != 0 {
		t.Errorf("degraded list --json should carry an empty models array, got %#v", row["models"])
	}
}

func TestListDegradesOnModelsSchemaDrift(t *testing.T) {
	// Unlike get/models (where models are central and drift is fatal), list degrades.
	srv := modelsServer(t, nil, "anthropic") // anthropic ships a malformed model
	newScenario(t, srv.URL, "alpha-cli")

	got := runCLI("--json", "agents", "list", "--installed")
	if got.code != codeOK {
		t.Fatalf("list on models schema drift exit = %d, want 0; stderr=%q", got.code, got.stderr)
	}
	env := got.envelope(t)
	rows, ok := env.Data.([]any)
	if !ok || len(rows) != 1 || rows[0].(map[string]any)["id"] != "alpha-cli" {
		t.Fatalf("list should still report the detected agent: %v", env.Data)
	}
	if !anyContains(env.Warnings, "model counts omitted") {
		t.Errorf("list should warn that model counts were omitted: %v", env.Warnings)
	}
}

func TestListJSONCarriesFullRecord(t *testing.T) {
	// --json without --fields is never truncated to default table columns.
	newScenario(t, "", "alpha-cli")

	got := runCLI("--json", "agents", "list")
	if got.code != codeOK {
		t.Fatalf("list exit = %d, stderr=%q", got.code, got.stderr)
	}
	row := got.envelope(t).Data.([]any)[0].(map[string]any)
	for _, key := range []string{"bin", "config_dir", "homepage"} {
		if _, ok := row[key]; !ok {
			t.Errorf("list --json should carry non-default field %q: %v", key, row)
		}
	}
}

func TestListWarnsOnStaleCatalog(t *testing.T) {
	// Zero catalog TTL forces re-resolution so the offline second run yields stale.
	s := newScenario(t, "", "alpha-cli")
	s.writeConfig(t, "color: \"never\"\nsearch_dirs: [\""+s.binDir+"\"]\ncatalog: ttl: \"0s\"\n")

	if got := runCLI("agents", "list"); got.code != codeOK {
		t.Fatalf("warm list exit = %d; stderr=%q", got.code, got.stderr)
	}

	s.closeRegistry() // re-resolution can no longer reach the registry

	got := runCLI("--json", "agents", "list")
	if got.code != codeOK {
		t.Fatalf("stale list exit = %d, want 0; stderr=%q", got.code, got.stderr)
	}
	if !anyContains(got.envelope(t).Warnings, "stale") {
		t.Errorf("stale list should warn about staleness: %v", got.envelope(t).Warnings)
	}
}
