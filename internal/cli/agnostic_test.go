package cli

import (
	"strings"
	"testing"
)

// delta-agent is the fixture's provider-agnostic agent: agnostic:true, no catalog provider list.

func TestGetAgnosticSoftPathOmitsProviderFields(t *testing.T) {
	// Unfiltered get without --provider: outside facts only, omits provider fields, warns how to enrich.
	newScenario(t, "", "delta-agent")

	got := runCLI("--json", "agents", "get", "delta-agent")
	if got.code != codeOK {
		t.Fatalf("get exit = %d, stderr=%q", got.code, got.stderr)
	}
	env := got.envelope(t)
	data := env.Data.(map[string]any)
	for _, key := range []string{"providers", "provider_env", "models"} {
		if _, ok := data[key]; ok {
			t.Errorf("agnostic soft-path get should omit %q: %v", key, data)
		}
	}
	if !anyContains(env.Warnings, "provider-agnostic") {
		t.Errorf("expected a provider-agnostic warning, got %v", env.Warnings)
	}
}

func TestGetAgnosticModelsWithoutProviderIsUsage(t *testing.T) {
	newScenario(t, "", "delta-agent")

	got := runCLI("agents", "get", "delta-agent", "--models")
	if got.code != codeUsage {
		t.Fatalf("get --models exit = %d, want %d; stderr=%q", got.code, codeUsage, got.stderr)
	}
	if !strings.Contains(got.stderr, "provider-agnostic") {
		t.Errorf("expected provider-agnostic error, got %q", got.stderr)
	}
}

func TestGetAgnosticEnrichesWithProvider(t *testing.T) {
	newScenario(t, "", "delta-agent")

	got := runCLI("--json", "agents", "get", "delta-agent", "--models", "--provider", "anthropic")
	if got.code != codeOK {
		t.Fatalf("get --models --provider exit = %d, stderr=%q", got.code, got.stderr)
	}
	data := got.envelope(t).Data.(map[string]any)
	provs, ok := data["providers"].([]any)
	if !ok || len(provs) != 1 || provs[0] != "anthropic" {
		t.Errorf("providers = %v, want [anthropic]", data["providers"])
	}
	if models, ok := data["models"].([]any); !ok || len(models) == 0 {
		t.Errorf("models missing or empty with --provider: %v", data["models"])
	}
}

func TestGetAgnosticUnknownProviderIsUsage(t *testing.T) {
	newScenario(t, "", "delta-agent")

	got := runCLI("agents", "get", "delta-agent", "--models", "--provider", "bogus")
	if got.code != codeUsage {
		t.Fatalf("get --provider bogus exit = %d, want %d; stderr=%q", got.code, codeUsage, got.stderr)
	}
	if !strings.Contains(got.stderr, "unknown provider") {
		t.Errorf("expected unknown-provider error, got %q", got.stderr)
	}
}

func TestGetProviderRejectedOnHomeProviderAgent(t *testing.T) {
	// A set outside the catalog providers is still a usage fault.
	newScenario(t, "", "alpha-cli")

	got := runCLI("agents", "get", "alpha-cli", "--provider", "google")
	if got.code != codeUsage {
		t.Fatalf("get --provider outside catalog set exit = %d, want %d; stderr=%q", got.code, codeUsage, got.stderr)
	}
}

func TestGetProviderAcceptedOnHomeProviderAgent(t *testing.T) {
	newScenario(t, "", "alpha-cli")

	got := runCLI("--json", "agents", "get", "alpha-cli", "--provider", "anthropic")
	if got.code != codeOK {
		t.Fatalf("get --provider catalog subset exit = %d, stderr=%q", got.code, got.stderr)
	}
}

func TestGetProviderNameQueryIsNotFound(t *testing.T) {
	// Exact miss whether or not --provider is supplied; no provider fallthrough.
	newScenario(t, "")

	got := runCLI("agents", "get", "anthropic", "--provider", "openai")
	if got.code != codeNotFound {
		t.Fatalf("provider-name query exit = %d, want %d; stderr=%q", got.code, codeNotFound, got.stderr)
	}
}

func TestModelsAgnosticAgentWithoutProviderIsUsage(t *testing.T) {
	newScenario(t, "", "delta-agent")

	got := runCLI("models", "list", "--agent", "delta-agent")
	if got.code != codeUsage {
		t.Fatalf("models list --agent exit = %d, want %d; stderr=%q", got.code, codeUsage, got.stderr)
	}
	if !strings.Contains(got.stderr, "provider-agnostic") {
		t.Errorf("expected provider-agnostic error, got %q", got.stderr)
	}
}

func TestModelsAgnosticAgentWithProviderLists(t *testing.T) {
	newScenario(t, "", "delta-agent")

	got := runCLI("--json", "models", "list", "--agent", "delta-agent", "--provider", "anthropic")
	if got.code != codeOK {
		t.Fatalf("models list --agent --provider exit = %d, stderr=%q", got.code, got.stderr)
	}
	rows := got.envelope(t).Data.([]any)
	if len(rows) == 0 {
		t.Errorf("models list --agent delta-agent --provider anthropic listed nothing: %s", got.stdout)
	}
}

func TestModelsAgnosticAgentUnknownProviderIsUsage(t *testing.T) {
	// Unknown --provider is usage, not a silent empty listing.
	newScenario(t, "", "delta-agent")

	got := runCLI("models", "list", "--agent", "delta-agent", "--provider", "bogus")
	if got.code != codeUsage {
		t.Fatalf("models list --agent --provider bogus exit = %d, want %d; stderr=%q", got.code, codeUsage, got.stderr)
	}
	if !strings.Contains(got.stderr, "unknown provider") {
		t.Errorf("expected unknown-provider error, got %q", got.stderr)
	}
}

func TestModelsProviderRejectedOnHomeProviderAgent(t *testing.T) {
	newScenario(t, "", "alpha-cli")

	got := runCLI("models", "list", "--agent", "alpha-cli", "--provider", "google")
	if got.code != codeUsage {
		t.Fatalf("models list --agent --provider outside catalog set exit = %d, want %d; stderr=%q", got.code, codeUsage, got.stderr)
	}
}

func TestModelsProviderAcceptedOnHomeProviderAgent(t *testing.T) {
	newScenario(t, "", "alpha-cli")

	got := runCLI("--json", "models", "list", "--agent", "alpha-cli", "--provider", "anthropic")
	if got.code != codeOK {
		t.Fatalf("models list --agent --provider catalog subset exit = %d, stderr=%q", got.code, got.stderr)
	}
	if rows := got.envelope(t).Data.([]any); len(rows) == 0 {
		t.Error("models list --agent alpha-cli --provider anthropic listed nothing")
	}
}

func TestGetAgnosticSoftPathNotInstalled(t *testing.T) {
	// Not installed is status, not miss: exit 0 with soft-path shape, no not-installed warning.
	newScenario(t, "") // delta binary not installed

	got := runCLI("--json", "agents", "get", "delta-agent")
	if got.code != codeOK {
		t.Fatalf("get exit = %d, want %d; stderr=%q", got.code, codeOK, got.stderr)
	}
	env := got.envelope(t)
	if env.Status != "ok" || env.Error != "" {
		t.Errorf("envelope status/error = %q/%q, want ok with no error", env.Status, env.Error)
	}
	if !anyContains(env.Warnings, "provider-agnostic") {
		t.Errorf("expected a provider-agnostic warning, got %v", env.Warnings)
	}
	if anyContains(env.Warnings, "not installed") {
		t.Errorf("not-installed should not be a warning: %v", env.Warnings)
	}
	data := env.Data.(map[string]any)
	for _, key := range []string{"providers", "provider_env", "models"} {
		if _, ok := data[key]; ok {
			t.Errorf("not-installed soft-path get should omit %q: %v", key, data)
		}
	}
	if data["found"] != false {
		t.Errorf("found = %v, want false", data["found"])
	}
}

func TestGetAgnosticProviderNotInstalled(t *testing.T) {
	// Enrichment does not depend on installation: --provider fills like an installed
	// agnostic agent; no soft-path warning once providers are supplied.
	newScenario(t, "") // delta binary not installed

	got := runCLI("--json", "agents", "get", "delta-agent", "--provider", "anthropic")
	if got.code != codeOK {
		t.Fatalf("get --provider exit = %d, want %d; stderr=%q", got.code, codeOK, got.stderr)
	}
	env := got.envelope(t)
	if env.Status != "ok" || env.Error != "" {
		t.Errorf("envelope status/error = %q/%q, want ok with no error", env.Status, env.Error)
	}
	if anyContains(env.Warnings, "not installed") {
		t.Errorf("not-installed should not be a warning: %v", env.Warnings)
	}
	data := env.Data.(map[string]any)
	provs, ok := data["providers"].([]any)
	if !ok || len(provs) != 1 || provs[0] != "anthropic" {
		t.Errorf("providers = %v, want [anthropic]", data["providers"])
	}
	if _, ok := data["provider_env"]; !ok {
		t.Errorf("not-installed get --provider should now fill provider_env like an installed agent: %v", data)
	}
	if _, ok := data["models"]; ok {
		t.Errorf("get --provider without --models should omit models: %v", data)
	}
	if anyContains(env.Warnings, "provider-agnostic") {
		t.Errorf("soft-path warning should not fire with --provider: %v", env.Warnings)
	}
}

func TestGetAgnosticBareProviderKeepsProviderEnvOmitsModels(t *testing.T) {
	// Bare --provider attaches a client (provider-env filled); Models stays opt-in.
	newScenario(t, "", "delta-agent")

	got := runCLI("--json", "agents", "get", "delta-agent", "--provider", "anthropic")
	if got.code != codeOK {
		t.Fatalf("get --provider exit = %d, stderr=%q", got.code, got.stderr)
	}
	data := got.envelope(t).Data.(map[string]any)
	if _, ok := data["provider_env"]; !ok {
		t.Errorf("provider_env missing from bare --provider get: %v", data)
	}
	if _, ok := data["models"]; ok {
		t.Errorf("bare --provider get should omit models: %v", data["models"])
	}
}

func TestGetAgnosticNonProviderFieldsStayOffline(t *testing.T) {
	// No provider-related field: catalog alone, no models.dev fetch, no agnostic warning.
	newScenario(t, mustNotFetchModelsServer(t), "delta-agent")

	got := runCLI("--json", "agents", "get", "delta-agent", "--fields", "skills_dir")
	if got.code != codeOK {
		t.Fatalf("get --fields skills_dir exit = %d, stderr=%q", got.code, got.stderr)
	}
	env := got.envelope(t)
	if len(env.Warnings) != 0 {
		t.Errorf("non-provider field selection should carry no warnings: %v", env.Warnings)
	}
	assertSkillsPrimaryPath(t, env.Data.(map[string]any)["skills_dir"])
}

func TestGetAgnosticFieldsProvidersValidatesCallerIds(t *testing.T) {
	// On agnostic agents providers is caller input, not catalog truth: validated with --provider.
	newScenario(t, "", "delta-agent")

	got := runCLI("agents", "get", "delta-agent", "--fields", "providers", "--provider", "bogus")
	if got.code != codeUsage {
		t.Fatalf("get --fields providers --provider bogus exit = %d, want %d; stderr=%q", got.code, codeUsage, got.stderr)
	}
	if !strings.Contains(got.stderr, "unknown provider") {
		t.Errorf("expected unknown-provider error, got %q", got.stderr)
	}

	valid := runCLI("--json", "agents", "get", "delta-agent", "--fields", "providers", "--provider", "anthropic")
	if valid.code != codeOK {
		t.Fatalf("get --fields providers --provider anthropic exit = %d, stderr=%q", valid.code, valid.stderr)
	}
	data := valid.envelope(t).Data.(map[string]any)
	provs, ok := data["providers"].([]any)
	if !ok || len(provs) != 1 || provs[0] != "anthropic" {
		t.Errorf("providers = %v, want [anthropic]", data["providers"])
	}
}

func TestGetAgnosticNonProviderFieldsWithProviderStaysOffline(t *testing.T) {
	// Nothing caller-provided is reported, so nothing needs validating — stays offline.
	newScenario(t, mustNotFetchModelsServer(t), "delta-agent")

	got := runCLI("--json", "agents", "get", "delta-agent", "--fields", "skills_dir", "--provider", "anthropic")
	if got.code != codeOK {
		t.Fatalf("get --fields skills_dir --provider exit = %d, stderr=%q", got.code, got.stderr)
	}
	assertSkillsPrimaryPath(t, got.envelope(t).Data.(map[string]any)["skills_dir"])
}

func TestModelsListDuplicateProviderDeduplicates(t *testing.T) {
	newScenario(t, "", "delta-agent")

	list := runCLI("--json", "models", "list", "--agent", "delta-agent", "--provider", "anthropic,anthropic")
	if list.code != codeOK {
		t.Fatalf("models list with duplicate --provider exit = %d, stderr=%q", list.code, list.stderr)
	}
	if rows := list.envelope(t).Data.([]any); len(rows) != 1 {
		t.Errorf("models rows = %d, want 1 (deduplicated)", len(rows))
	}
}

func TestGetAgnosticProviderDegradesWithWarningWhenUnreachable(t *testing.T) {
	// Unreachable+uncached degrades like home-provider: exit 0, omit provider_env/models, warn.
	newScenario(t, closedModelsServer(t), "delta-agent")

	got := runCLI("--json", "agents", "get", "delta-agent", "--provider", "anthropic")
	if got.code != codeOK {
		t.Fatalf("get --provider degrade exit = %d, want 0; stderr=%q", got.code, got.stderr)
	}
	env := got.envelope(t)
	if !anyContains(env.Warnings, "models.dev") {
		t.Errorf("degrade should warn about models.dev: %v", env.Warnings)
	}
	data := env.Data.(map[string]any)
	for _, key := range []string{"provider_env", "models"} {
		if _, ok := data[key]; ok {
			t.Errorf("degraded get --provider should omit %q: %v", key, data)
		}
	}
}

func TestListAgnosticProviderShowsCount(t *testing.T) {
	// With --provider, agnostic row matches home-provider shape: count, not null/-.
	newScenario(t, "", "delta-agent")

	got := runCLI("--json", "agents", "list", "--installed", "--provider", "anthropic")
	if got.code != codeOK {
		t.Fatalf("list --provider exit = %d, stderr=%q", got.code, got.stderr)
	}
	rows := got.envelope(t).Data.([]any)
	if len(rows) != 1 {
		t.Fatalf("list rows = %d, want 1", len(rows))
	}
	row := rows[0].(map[string]any)
	if n, ok := row["models"].(float64); !ok || n < 1 {
		t.Errorf("agnostic models with --provider = %v, want a positive count", row["models"])
	}
}

func TestListAgnosticUnknownProviderIsUsage(t *testing.T) {
	// Unknown id fails the listing; missing-provider soft-skips enrichment instead.
	newScenario(t, "", "delta-agent")

	got := runCLI("agents", "list", "--provider", "bogus")
	if got.code != codeUsage {
		t.Fatalf("list --provider bogus exit = %d, want %d; stderr=%q", got.code, codeUsage, got.stderr)
	}
	if !strings.Contains(got.stderr, "unknown provider") {
		t.Errorf("expected unknown-provider error, got %q", got.stderr)
	}
}

func TestListUnknownProviderIsUsageWithoutAgnosticInstalled(t *testing.T) {
	// --provider is validated at the boundary even with no agnostic agent present.
	newScenario(t, "", "alpha-cli") // only a home-provider agent installed; delta absent

	got := runCLI("agents", "list", "--provider", "bogus")
	if got.code != codeUsage {
		t.Fatalf("list --provider bogus exit = %d, want %d; stderr=%q", got.code, codeUsage, got.stderr)
	}
	if !strings.Contains(got.stderr, "unknown provider") {
		t.Errorf("expected unknown-provider error, got %q", got.stderr)
	}
}
