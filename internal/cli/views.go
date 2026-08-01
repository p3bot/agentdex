package cli

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/start-cli/agentdex"
	"github.com/start-cli/agentdex/internal/tui"
	"github.com/start-cli/agentdex/modelsdev"
)

// agentFieldSet is the declared field authority for a detected agent: every valid
// --fields key in canonical order, and the subset shown as default list columns.
// It governs --fields validation and the get text-detail ordering, so both
// surfaces stay in step when a field is added or renamed.
var agentFieldSet = newFieldSet(
	[]string{"id", "name", "version", "bin", "found", "config_dir", "config_local_dir", "skills_dir", "skills_local_dir", "skills", "providers", "homepage", "provider_env", "models"},
	[]string{"id", "name", "version", "providers", "models", "bin"},
).ordered("id")

// agentVerboseFields are the list table columns under --verbose: the default
// columns widened with the global config dir. models sits between providers and
// bin in both sets; bin stays last, the widest, most variable column, whose
// "missing" cell is the list detection signal.
var agentVerboseFields = []string{"id", "name", "version", "config_dir", "providers", "models", "bin"}

// skillsPathPayload is one expanded skill root in the structured skills field:
// path plus on-disk existence, matching library PathEntry.
type skillsPathPayload struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

// skillsScopePayload is one scope in the structured skills field (JSON).
// Role entries carry path and exists; primary stays a bare path string (the
// fast product answer — same value as skills_dir / skills_local_dir).
type skillsScopePayload struct {
	Agents       *skillsPathPayload  `json:"agents,omitempty"`
	Native       *skillsPathPayload  `json:"native,omitempty"`
	Alternatives []skillsPathPayload `json:"alternatives,omitempty"`
	Primary      string              `json:"primary,omitempty"`
}

// skillsPayload is the full skills matrix for CLI JSON: roles plus derived primary.
type skillsPayload struct {
	Global *skillsScopePayload `json:"global,omitempty"`
	Local  *skillsScopePayload `json:"local,omitempty"`
}

// agentRecord builds the field values for one detected agent. Optional fields that
// are absent (no local config, no skills concept, no enrichment) are simply not
// added; they remain valid to select per agentFieldSet and resolve to a blank.
func agentRecord(a *agentdex.Agent) *record {
	return buildAgentRecord(a, true)
}

// agentRecordWithoutProviders is the agnostic soft-path record: outside facts
// only, omitting the providers field (and never adding provider_env / models).
func agentRecordWithoutProviders(a *agentdex.Agent) *record {
	return buildAgentRecord(a, false)
}

func buildAgentRecord(a *agentdex.Agent, includeProviders bool) *record {
	d := a.Detection
	r := newRecord(agentFieldSet)
	r.add("id", a.ID, a.ID)
	r.add("name", a.Name, a.Name)
	r.add("version", d.Version, orDash(d.Version))
	// A not-found agent renders "missing" in the bin cell (the list detection
	// signal); the JSON value stays blank with found carrying the fact.
	binText := orDash(d.BinaryPath)
	if !d.Found {
		binText = "missing"
	}
	r.add("bin", d.BinaryPath, binText)
	r.add("found", d.Found, fmt.Sprintf("%t", d.Found))
	r.add("config_dir", d.Config.Global, orDash(d.Config.Global))
	// config_local_dir and skills paths are added here, in their declared position,
	// so the add order matches agentFieldSet.all and the text detail view order.
	if d.Config.Local != "" {
		r.add("config_local_dir", d.Config.Local, d.Config.Local)
	}
	// skills_dir / skills_local_dir are the derived primary for each scope
	// (fast product answer). skills is the full agents/native/alternatives matrix.
	if d.Skills.Global.Primary.Path != "" {
		r.add("skills_dir", d.Skills.Global.Primary.Path, d.Skills.Global.Primary.Path)
	}
	if d.Skills.Local.Primary.Path != "" {
		r.add("skills_local_dir", d.Skills.Local.Primary.Path, d.Skills.Local.Primary.Path)
	}
	if payload, text := skillsField(d.Skills); payload != nil {
		r.add("skills", payload, text)
	}
	if includeProviders {
		r.add("providers", a.ResolvedProviders, strings.Join(a.ResolvedProviders, ", "))
	}
	r.add("homepage", a.Homepage, orDash(a.Homepage))
	return r
}

// skillsField builds the structured skills matrix for JSON and plain text.
// Omitted when the agent has no skills concept (both scopes empty).
func skillsField(sp agentdex.SkillsPaths) (*skillsPayload, string) {
	g := skillsScopePayloadOf(sp.Global)
	l := skillsScopePayloadOf(sp.Local)
	if g == nil && l == nil {
		return nil, ""
	}
	p := &skillsPayload{Global: g, Local: l}
	return p, formatSkillsText(p)
}

func skillsScopePayloadOf(sc agentdex.SkillsScope) *skillsScopePayload {
	if sc.Agents.Path == "" && sc.Native.Path == "" && len(sc.Alternatives) == 0 && sc.Primary.Path == "" {
		return nil
	}
	p := &skillsScopePayload{
		Agents:  skillsPathPayloadOf(sc.Agents),
		Native:  skillsPathPayloadOf(sc.Native),
		Primary: sc.Primary.Path,
	}
	if len(sc.Alternatives) > 0 {
		p.Alternatives = make([]skillsPathPayload, len(sc.Alternatives))
		for i, e := range sc.Alternatives {
			p.Alternatives[i] = skillsPathPayload{Path: e.Path, Exists: e.Exists}
		}
	}
	return p
}

func skillsPathPayloadOf(e agentdex.PathEntry) *skillsPathPayload {
	if e.Path == "" {
		return nil
	}
	return &skillsPathPayload{Path: e.Path, Exists: e.Exists}
}

// formatSkillsText is a compact one-line rendering of the skills matrix for
// --fields and other plain-text surfaces. Full get detail uses the Skills
// section instead (renderSkillsSection). Path colour stays on skills_dir /
// skills_local_dir scalars only; role lines carry (exists)/(missing) inline.
func formatSkillsText(p *skillsPayload) string {
	var parts []string
	if p.Global != nil {
		parts = append(parts, "global: "+formatSkillsScopeText(p.Global))
	}
	if p.Local != nil {
		parts = append(parts, "local: "+formatSkillsScopeText(p.Local))
	}
	return strings.Join(parts, "; ")
}

func formatSkillsScopeText(s *skillsScopePayload) string {
	var bits []string
	if s.Agents != nil {
		bits = append(bits, "agents="+formatSkillsPathText(s.Agents))
	}
	if s.Native != nil {
		bits = append(bits, "native="+formatSkillsPathText(s.Native))
	}
	if len(s.Alternatives) > 0 {
		alts := make([]string, len(s.Alternatives))
		for i := range s.Alternatives {
			alts[i] = formatSkillsPathText(&s.Alternatives[i])
		}
		bits = append(bits, "alternatives=["+strings.Join(alts, ", ")+"]")
	}
	if s.Primary != "" {
		bits = append(bits, "primary="+s.Primary)
	}
	return strings.Join(bits, " ")
}

func formatSkillsPathText(p *skillsPathPayload) string {
	if p == nil {
		return ""
	}
	if p.Exists {
		return p.Path + " (exists)"
	}
	return p.Path + " (missing)"
}

// withModelsNA marks models as not applicable (agnostic agent without --provider):
// JSON null and text "-", distinct from withModels's nil→[] degrade shape.
func withModelsNA(r *record) {
	r.add("models", nil, "-")
}

// withProviderEnv adds the provider-env field to an agent record. Provider-env is
// shown whenever a client was consulted, independent of Models fill.
func withProviderEnv(r *record, env map[string]bool) {
	if env == nil {
		return
	}
	r.add("provider_env", env, formatProviderEnv(env))
}

// withModels adds a bare models.dev model list as the models field (provider
// detail): typed list for JSON and a count for the table cell. A nil list is
// normalised to an empty slice so the JSON carries [] to match the "0" count
// cell, rather than a null that disagrees with the text and breaks
// `jq '.models|length'`.
func withModels(r *record, models []modelsdev.Model) {
	if models == nil {
		models = []modelsdev.Model{}
	}
	r.add("models", models, fmt.Sprintf("%d", len(models)))
}

// withAgentModels adds an attributed agent model list (provider + canonical id
// on each row) for agents list/get JSON and the models count cell.
func withAgentModels(r *record, models []agentdex.Model) {
	if models == nil {
		models = []agentdex.Model{}
	}
	r.add("models", models, fmt.Sprintf("%d", len(models)))
}

// modelFieldSet is the declared field authority for a model: every valid --fields
// key in canonical order, and the subset shown as default models-table columns.
var modelFieldSet = newFieldSet(
	[]string{"id", "provider", "name", "family", "context", "input", "output", "total", "reasoning", "tool_call", "attachment", "released", "canonical_id"},
	[]string{"id", "name", "context", "input", "output"},
).ordered("released", "released")

// modelRecord builds the field values for one model. canonical_id is added only
// when non-empty (the model has a real models.dev agnostic id); it remains valid
// to select per modelFieldSet, so --fields canonical_id yields a blank for a model
// without one rather than an unknown-field error.
func modelRecord(m modelsdev.Model, providerID, canonicalID string) *record {
	r := newRecord(modelFieldSet)
	r.add("id", m.ID, m.ID)
	r.add("provider", providerID, providerID)
	r.add("name", m.Name, m.Name)
	r.add("family", m.Family, orDash(m.Family))
	r.add("context", m.Limit.Context, fmt.Sprintf("%d", m.Limit.Context))
	r.add("input", costValue(m.Cost, costInput), costText(m.Cost, costInput))
	r.add("output", costValue(m.Cost, costOutput), costText(m.Cost, costOutput))
	r.add("total", totalValue(m.Cost), totalText(m.Cost))
	r.add("reasoning", m.Reasoning, fmt.Sprintf("%t", m.Reasoning))
	r.add("tool_call", m.ToolCall, fmt.Sprintf("%t", m.ToolCall))
	r.add("attachment", m.Attachment, fmt.Sprintf("%t", m.Attachment))
	r.add("released", m.ReleaseDate, orDash(m.ReleaseDate))
	if canonicalID != "" {
		r.add("canonical_id", canonicalID, canonicalID)
	}
	return r
}

// providerFieldSet is the declared field authority for a models.dev provider: the
// full ordered set of valid --fields keys and the default listing columns. env is
// the terse presence-folded column; present is the structured per-variable presence
// map, selectable but not a default column, so scripts read booleans without
// parsing the env text's (set) suffix. models stays array-typed (rendered as a
// count) so a caller reads .data[].models uniformly across commands.
var providerFieldSet = newFieldSet(
	[]string{"id", "name", "env", "present", "models", "doc", "npm", "api"},
	[]string{"id", "name", "env", "models"},
).ordered("id")

// providerRecord builds the field values for one provider. present maps each of the
// provider's API-key variable names to whether it is set in the environment; the
// library resolves it at the boundary and passes it in (as Provider.EnvPresent), so
// the record builder is testable from inputs. env carries the sorted names for JSON
// and the presence-folded cell for text; present carries the map itself.
func providerRecord(p modelsdev.Provider, present map[string]bool) *record {
	r := newRecord(providerFieldSet)
	r.add("id", p.ID, p.ID)
	r.add("name", p.Name, p.Name)
	// The declared API-key variables come from p.Env, the source of truth; present
	// only supplies the (set) markers. A copy keeps the [] JSON shape for a
	// no-env provider rather than a null.
	envNames := make([]string, len(p.Env))
	copy(envNames, p.Env)
	sort.Strings(envNames)
	r.add("env", envNames, providerEnvCell(envNames, present))
	r.add("present", present, formatProviderEnv(present))
	models := make([]modelsdev.Model, 0, len(p.Models))
	for _, key := range sortedKeys(p.Models) {
		models = append(models, p.Models[key])
	}
	withModels(r, models)
	r.add("doc", p.Doc, orDash(p.Doc))
	r.add("npm", p.NPM, orDash(p.NPM))
	r.add("api", p.API, orDash(p.API))
	return r
}

// providerEnvCell renders the presence-folded ENV column over names (already
// sorted): a set variable suffixed "(set)" and an unset one left bare, so a bare
// name means unset. A provider with no declared variable renders blank. The terse
// divergence from get's symmetric (set)/(unset) markers keeps the wide browse
// listing legible.
func providerEnvCell(names []string, present map[string]bool) string {
	parts := make([]string, 0, len(names))
	for _, k := range names {
		if present[k] {
			parts = append(parts, k+" "+plainState("set", true))
			continue
		}
		parts = append(parts, k)
	}
	return strings.Join(parts, ", ")
}

type costKind int

const (
	costInput costKind = iota
	costOutput
)

// costFor returns the per-1M-token price for the given kind and whether pricing is
// known. A nil Cost means models.dev carried no pricing for the model.
func costFor(c *modelsdev.Cost, kind costKind) (float64, bool) {
	if c == nil {
		return 0, false
	}
	if kind == costOutput {
		return c.Output, true
	}
	return c.Input, true
}

// costValue returns the per-1M-token price for JSON, or nil when pricing is
// unknown so the field is null rather than a misleading zero.
func costValue(c *modelsdev.Cost, kind costKind) any {
	v, ok := costFor(c, kind)
	if !ok {
		return nil
	}
	return v
}

// costText renders the per-1M-token price at full precision with trailing zeros
// trimmed ("$3", "$0.075", "$0.0001"), so cheap sub-cent prices stay truthful
// rather than rounding to a misleading "$0.00".
func costText(c *modelsdev.Cost, kind costKind) string {
	v, ok := costFor(c, kind)
	if !ok {
		return "-"
	}
	return "$" + strconv.FormatFloat(v, 'f', -1, 64)
}

// combinedCost sums the input and output price per 1M tokens, rounding away the
// binary floating-point artifact the addition can introduce (0.05+0.025 lands at
// 0.07500000000000001) at nano-dollar precision, well below any realistic price so
// no genuine value is distorted. The bool is false when pricing is unknown.
func combinedCost(c *modelsdev.Cost) (float64, bool) {
	if c == nil {
		return 0, false
	}
	return math.Round((c.Input+c.Output)*1e9) / 1e9, true
}

// totalValue is the combined input+output price per 1M tokens for JSON, or nil when
// pricing is unknown so the field is null and sorts last rather than a misleading
// zero. It is a rough comparison signal, not a workload cost: real usage rarely
// splits input and output tokens evenly.
func totalValue(c *modelsdev.Cost) any {
	v, ok := combinedCost(c)
	if !ok {
		return nil
	}
	return v
}

// totalText renders the combined price with the same "$" trimming as costText, or a
// dash when pricing is unknown.
func totalText(c *modelsdev.Cost) string {
	v, ok := combinedCost(c)
	if !ok {
		return "-"
	}
	return "$" + strconv.FormatFloat(v, 'f', -1, 64)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// formatProviderEnv renders the provider-env map deterministically as
// "VAR (set|unset)" entries in sorted key order. It stays plain so record text,
// table cells, and --fields output carry no colour codes.
func formatProviderEnv(env map[string]bool) string {
	return providerEnvText(env, plainState)
}

// styledProviderEnv is formatProviderEnv with the state markers coloured for the
// detail section: (set) green, (unset) yellow.
func styledProviderEnv(env map[string]bool) string {
	return providerEnvText(env, styledState)
}

func providerEnvText(env map[string]bool, state func(string, bool) string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		text, good := "unset", false
		if env[k] {
			text, good = "set", true
		}
		parts[i] = k + " " + state(text, good)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func plainState(state string, _ bool) string {
	return "(" + state + ")"
}

// styledState renders a bracketed state marker per the start terminal colour
// standard: cyan delimiters, with the state text green when positive and yellow
// when negative.
func styledState(state string, good bool) string {
	inner := tui.Warn
	if good {
		inner = tui.Good
	}
	return tui.Delim.Sprint("(") + inner.Sprint(state) + tui.Delim.Sprint(")")
}
