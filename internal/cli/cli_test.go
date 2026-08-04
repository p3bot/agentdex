package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/p3bot/agentdex/internal/catalogtest"
)

func TestVersionEnvelope(t *testing.T) {
	got := runCLI("--json", "version")
	if got.code != codeOK {
		t.Fatalf("version exit = %d", got.code)
	}
	data := got.envelope(t).Data.(map[string]any)
	for _, k := range []string{"version", "commit", "date"} {
		if _, ok := data[k]; !ok {
			t.Errorf("version data missing %q: %v", k, data)
		}
	}
}

func TestMalformedConfigExits78(t *testing.T) {
	s := newScenario(t, "", "alpha-cli")
	s.writeConfig(t, `color: "not-a-mode"`)

	got := runCLI("agents", "list")
	if got.code != codeConfig {
		t.Fatalf("malformed config exit = %d, want 78; stderr=%q", got.code, got.stderr)
	}
}

func TestUnreadableConfigExits4(t *testing.T) {
	// Permission denial is exit 4, distinct from validity fault (78).
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	s := newScenario(t, "", "alpha-cli")
	cfgPath := filepath.Join(s.configDir, "config.cue")
	if err := os.Chmod(cfgPath, 0o000); err != nil {
		t.Fatalf("chmod config unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(cfgPath, 0o644) })

	got := runCLI("agents", "list")
	if got.code != codePermission {
		t.Fatalf("unreadable config exit = %d, want 4; stderr=%q", got.code, got.stderr)
	}
}

func TestMalformedConfigDoesNotBreakVersion(t *testing.T) {
	s := newScenario(t, "", "alpha-cli")
	s.writeConfig(t, `bogus_field: 1`)

	got := runCLI("version")
	if got.code != codeOK {
		t.Fatalf("version with malformed config exit = %d, want 0", got.code)
	}
}

func TestConfigRejectsRemovedEnrichModels(t *testing.T) {
	// leftover enrich_models fails closed-schema validation (exit 78).
	s := newScenario(t, "", "alpha-cli")
	s.writeConfig(t, configBody("", s.binDir, "enrich_models: false\n"))

	got := runCLI("agents", "list")
	if got.code != codeConfig {
		t.Fatalf("enrich_models leftover exit = %d, want 78; stderr=%q", got.code, got.stderr)
	}
}

func TestListFieldsSelection(t *testing.T) {
	newScenario(t, "", "alpha-cli")

	got := runCLI("--json", "agents", "list", "--fields", "id,version")
	if got.code != codeOK {
		t.Fatalf("list --fields exit = %d, stderr=%q", got.code, got.stderr)
	}
	row := got.envelope(t).Data.([]any)[0].(map[string]any)
	if len(row) != 2 {
		t.Errorf("expected exactly id,version: %v", row)
	}
	if _, ok := row["id"]; !ok {
		t.Errorf("missing id: %v", row)
	}
}

func TestUnknownFieldIsUsageError(t *testing.T) {
	newScenario(t, "", "alpha-cli")

	got := runCLI("agents", "list", "--fields", "nope")
	if got.code != codeUsage {
		t.Fatalf("unknown field exit = %d, want 2; stderr=%q", got.code, got.stderr)
	}
}

func TestFieldSingularAliasSelectsFields(t *testing.T) {
	// Singular --field is a normalize-func alias so a common slip still selects.
	newScenario(t, "", "alpha-cli")

	got := runCLI("--json", "agents", "list", "--field", "id,version")
	if got.code != codeOK {
		t.Fatalf("list --field exit = %d, stderr=%q", got.code, got.stderr)
	}
	row := got.envelope(t).Data.([]any)[0].(map[string]any)
	if len(row) != 2 {
		t.Errorf("--field should select exactly id,version: %v", row)
	}
	if _, ok := row["id"]; !ok {
		t.Errorf("--field selection missing id: %v", row)
	}
}

func TestInvalidColorFlagIsUsageError(t *testing.T) {
	// Settled in preRun before any command runs, so usage regardless of subcommand.
	newScenario(t, "", "alpha-cli")

	got := runCLI("--color", "rainbow", "agents", "list")
	if got.code != codeUsage {
		t.Fatalf("invalid --color exit = %d, want 2; stderr=%q", got.code, got.stderr)
	}

	gotJSON := runCLI("--json", "--color", "rainbow", "agents", "list")
	if gotJSON.code != codeUsage {
		t.Fatalf("invalid --color --json exit = %d, want 2; stderr=%q", gotJSON.code, gotJSON.stderr)
	}
	env := gotJSON.envelope(t)
	if env.Status != "error" || !strings.Contains(env.Error, "--color") {
		t.Errorf("invalid --color --json envelope = %+v, want an error naming --color", env)
	}
}

func TestMalformedBinPathIsUsageError(t *testing.T) {
	newScenario(t, "", "alpha-cli")

	got := runCLI("agents", "list", "--bin-path", "no-equals-sign")
	if got.code != codeUsage {
		t.Fatalf("malformed --bin-path exit = %d, want 2; stderr=%q", got.code, got.stderr)
	}
}

func TestBinPathOverridesDetection(t *testing.T) {
	// No fixture bin on PATH/search_dirs; --bin-path is the sole way to Found.
	s := newScenario(t, "")
	elsewhere := filepath.Join(s.home, "elsewhere")
	mustMkdir(t, elsewhere)
	installFakeBin(t, elsewhere, "alpha-cli")
	override := filepath.Join(elsewhere, catalogtest.FixtureBin(t, "alpha-cli"))

	got := runCLI("--json", "agents", "list", "--installed", "--bin-path", "alpha-cli="+override)
	if got.code != codeOK {
		t.Fatalf("list --bin-path exit = %d, stderr=%q", got.code, got.stderr)
	}
	rows := got.envelope(t).Data.([]any)
	if len(rows) != 1 {
		t.Fatalf("installed rows = %d, want 1 (alpha-cli via override)", len(rows))
	}
	row := rows[0].(map[string]any)
	if row["id"] != "alpha-cli" {
		t.Errorf("id = %v, want alpha-cli", row["id"])
	}
	if bin, _ := row["bin"].(string); bin != override {
		t.Errorf("bin = %v, want override path %q", row["bin"], override)
	}
	// Without override the agent must not appear as installed.
	noOverride := runCLI("--json", "agents", "list", "--installed")
	if noOverride.code != codeOK {
		t.Fatalf("list without override exit = %d, stderr=%q", noOverride.code, noOverride.stderr)
	}
	if rows := noOverride.envelope(t).Data.([]any); len(rows) != 0 {
		t.Errorf("without --bin-path installed rows = %v, want empty", rows)
	}
}

func TestSearchDirValueTakenLiterally(t *testing.T) {
	// StringArray: a path containing a comma is one location, never csv-split.
	s := newScenario(t, "")
	commaDir := filepath.Join(s.home, "odd,dir")
	mustMkdir(t, commaDir)
	installFakeBin(t, commaDir, "alpha-cli")

	got := runCLI("agents", "list", "--search-dir", commaDir)
	if got.code != codeOK {
		t.Fatalf("list --search-dir exit = %d, stderr=%q", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "alpha-cli") {
		t.Errorf("agent in a comma-containing search dir not detected:\n%s", got.stdout)
	}
}

func configBody(modelsURL, binDir, extra string) string {
	var b strings.Builder
	b.WriteString("color: \"never\"\n")
	b.WriteString("search_dirs: [\"" + binDir + "\"]\n")
	if modelsURL != "" {
		b.WriteString("models: url: \"" + modelsURL + "\"\n")
	}
	b.WriteString(extra)
	return b.String()
}
