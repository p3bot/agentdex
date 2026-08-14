package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBareNounIsUsageFault(t *testing.T) {
	// Bare noun is usage (exit 2): short error + help pointer, not a full help dump.
	newScenario(t, "", "alpha-cli")

	for _, noun := range []string{"agents", "providers", "models"} {
		got := runCLI(noun)
		if got.code != codeUsage {
			t.Errorf("bare %q exit = %d, want 2; stderr=%q", noun, got.code, got.stderr)
		}
		if strings.TrimSpace(got.stdout) != "" {
			t.Errorf("bare %q should not dump help on stdout:\n%s", noun, got.stdout)
		}
		if !strings.Contains(got.stderr, "list or get") {
			t.Errorf("bare %q stderr should name list or get:\n%s", noun, got.stderr)
		}
		if !strings.Contains(got.stderr, noun+" --help") {
			t.Errorf("bare %q stderr should point at --help:\n%s", noun, got.stderr)
		}
	}
}

func TestBareNounJSONIsEnvelopeAlone(t *testing.T) {
	// Under --json the envelope alone sits on stdout with no help text mixed in.
	newScenario(t, "", "alpha-cli")

	got := runCLI("--json", "agents")
	if got.code != codeUsage {
		t.Fatalf("bare agents --json exit = %d, want 2; stderr=%q", got.code, got.stderr)
	}
	var env envelope
	if err := json.Unmarshal([]byte(got.stdout), &env); err != nil {
		t.Fatalf("bare-noun --json stdout is not pure JSON: %v\nstdout=%q", err, got.stdout)
	}
	if env.Status != "error" || !strings.Contains(env.Error, "subcommand") {
		t.Errorf("envelope = %+v, want an error naming the missing subcommand", env)
	}
}

func TestSingularNounAliasIsSynonym(t *testing.T) {
	srv := modelsServer(t, []string{"anthropic"})
	newScenario(t, srv.URL, "alpha-cli")

	plural := runCLI("--json", "agents", "get", "alpha-cli")
	singular := runCLI("--json", "agent", "get", "alpha-cli")
	if plural.code != codeOK || singular.code != codeOK {
		t.Fatalf("agents/agent get exits = %d/%d, want 0/0", plural.code, singular.code)
	}
	if plural.stdout != singular.stdout {
		t.Errorf("agent get differs from agents get:\nplural:\n%s\nsingular:\n%s", plural.stdout, singular.stdout)
	}

	if got := runCLI("--json", "provider", "list"); got.code != codeOK {
		t.Errorf("provider list exit = %d, want 0; stderr=%q", got.code, got.stderr)
	}
	if got := runCLI("--json", "model", "list", "--provider", "anthropic"); got.code != codeOK {
		t.Errorf("model list exit = %d, want 0; stderr=%q", got.code, got.stderr)
	}
}

func TestJSONEnvelopeCoversCobraUsageErrors(t *testing.T) {
	// Find / ValidateArgs fail before preRun; no catalog or models.dev.
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"agents get arity", []string{"--json", "agents", "get"}, `agents get requires an agent id; run "agentdex agents list" to see agent ids`},
		{"agents get arity flag last", []string{"agents", "get", "--json"}, `agents get requires an agent id; run "agentdex agents list" to see agent ids`},
		{"agents get extra args", []string{"--json", "agents", "get", "a", "b"}, `agents get takes one agent id, got "a" "b"; run "agentdex agents get --help"`},
		{"models get arity", []string{"--json", "models", "get"}, `models get requires a provider-id/model-id; run "agentdex models list" to see model ids`},
		{"models get extra args", []string{"--json", "models", "get", "a", "b"}, `models get takes one provider-id/model-id, got "a" "b"; run "agentdex models get --help"`},
		{"providers get arity", []string{"--json", "providers", "get"}, `providers get requires a provider id; run "agentdex providers list" to see provider ids`},
		{"providers get extra args", []string{"--json", "providers", "get", "a", "b"}, `providers get takes one provider id, got "a" "b"; run "agentdex providers get --help"`},
		{"agents list extra args", []string{"--json", "agents", "list", "a", "b"}, `agents list takes at most one filter, got "a" "b"; run "agentdex agents list --help"`},
		{"models list extra args", []string{"--json", "models", "list", "a", "b"}, `models list takes at most one filter, got "a" "b"; run "agentdex models list --help"`},
		{"providers list extra args", []string{"--json", "providers", "list", "a", "b"}, `providers list takes at most one filter, got "a" "b"; run "agentdex providers list --help"`},
		{"refresh extra args", []string{"--json", "refresh", "a", "b"}, `refresh takes at most one target, got "a" "b"; run "agentdex refresh --help"`},
		{"version extra args", []string{"--json", "version", "x"}, `version takes no arguments, got "x"; run "agentdex version --help"`},
		{"unknown command", []string{"--json", "foobar"}, `unknown command "foobar" for "agentdex"`},
		{"unknown command flag last", []string{"foobar", "--json"}, `unknown command "foobar" for "agentdex"`},
		{"unknown flag", []string{"--json", "--not-a-flag"}, "unknown flag: --not-a-flag"},
		{"unknown shorthand then json", []string{"-v", "--json"}, "unknown shorthand flag: 'v' in -v"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := assertErrorEnvelope(t, runCLI(tc.args...), codeUsage)
			if msg, _ := m["error"].(string); msg != tc.want {
				t.Errorf("error %q, want %q", msg, tc.want)
			}
		})
	}
}

func TestJSONEnvelopePreservesClassifiedFailures(t *testing.T) {
	newScenario(t, "", "alpha-cli")

	tests := []struct {
		name    string
		args    []string
		code    int
		errPart string
	}{
		{"unknown noun subcommand", []string{"--json", "agents", "foobar"}, codeUsage, "unknown agents subcommand"},
		{"unknown agent id", []string{"--json", "agents", "get", "no-such-thing"}, codeNotFound, "no agent"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := assertErrorEnvelope(t, runCLI(tc.args...), tc.code)
			if msg, _ := m["error"].(string); !strings.Contains(msg, tc.errPart) {
				t.Errorf("error %q, want substring %q", msg, tc.errPart)
			}
		})
	}
}

func TestCobraUsageErrorsStayTextWithoutJSON(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		errPart string
	}{
		{"agents get arity", []string{"agents", "get"}, `agents get requires an agent id; run "agentdex agents list" to see agent ids`},
		{"agents get extra args", []string{"agents", "get", "a", "b"}, `agents get takes one agent id, got "a" "b"; run "agentdex agents get --help"`},
		{"agents list extra args", []string{"agents", "list", "a", "b"}, `agents list takes at most one filter, got "a" "b"; run "agentdex agents list --help"`},
		{"version extra args", []string{"version", "x"}, `version takes no arguments, got "x"; run "agentdex version --help"`},
		{"unknown command", []string{"foobar"}, "unknown command"},
		{"json explicitly off", []string{"--json=false", "foobar"}, "unknown command"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runCLI(tc.args...)
			if got.code != codeUsage {
				t.Fatalf("exit = %d, want 2; stderr=%q stdout=%q", got.code, got.stderr, got.stdout)
			}
			if !strings.Contains(got.stderr, "error:") || !strings.Contains(got.stderr, tc.errPart) {
				t.Errorf("stderr = %q, want error: line containing %q", got.stderr, tc.errPart)
			}
			if strings.TrimSpace(got.stdout) != "" {
				t.Errorf("stdout should stay empty in text mode: %q", got.stdout)
			}
		})
	}
}

func TestRemovedFlatCommandsAreGone(t *testing.T) {
	// Old flat get/list are unknown top-level commands; bare providers/models are
	// noun usage faults, never the old listing.
	srv := modelsServer(t, []string{"anthropic", "google", "openai"})
	newScenario(t, srv.URL, "alpha-cli")

	for _, args := range [][]string{{"get", "alpha-cli"}, {"list"}} {
		got := runCLI(args...)
		if got.code != codeUsage {
			t.Errorf("%v exit = %d, want 2 (unknown command); stderr=%q", args, got.code, got.stderr)
		}
	}

	prov := runCLI("providers")
	if prov.code != codeUsage {
		t.Errorf("providers exit = %d, want 2; stderr=%q", prov.code, prov.stderr)
	}
	if strings.Contains(prov.stdout, "ANTHROPIC_API_KEY") {
		t.Errorf("bare providers should not render the old listing:\n%s", prov.stdout)
	}

	mod := runCLI("models", "alpha-cli")
	if mod.code != codeUsage {
		t.Errorf("models <id> exit = %d, want 2; stderr=%q", mod.code, mod.stderr)
	}
	if strings.Contains(mod.stdout, "claude-sonnet") {
		t.Errorf("models <id> should not render the old model listing:\n%s", mod.stdout)
	}
}
