package cli

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/p3bot/agentdex"
	"github.com/p3bot/agentdex/internal/tui"
	"github.com/p3bot/agentdex/modelsdev"
)

var agentFieldSet = newFieldSet(
	[]string{"id", "name", "bin", "found", "config_dir", "config_local_dir", "skills_dir", "skills_local_dir", "skills", "providers", "homepage", "provider_env", "models"},
	[]string{"id", "name", "providers", "models", "bin"},
).ordered("id")

type skillsPathPayload struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

// primary is a bare path (same value as skills_dir / skills_local_dir); roles carry path+exists.
type skillsScopePayload struct {
	Agents       *skillsPathPayload  `json:"agents,omitempty"`
	Native       *skillsPathPayload  `json:"native,omitempty"`
	Alternatives []skillsPathPayload `json:"alternatives,omitempty"`
	Primary      string              `json:"primary,omitempty"`
}

type skillsPayload struct {
	Global *skillsScopePayload `json:"global,omitempty"`
	Local  *skillsScopePayload `json:"local,omitempty"`
}

// Optional absent fields are not added; they remain valid to select (JSON empty, text "-").
func agentRecord(a *agentdex.Agent) *record {
	return buildAgentRecord(a, true)
}

// Agnostic soft-path: outside facts only (no provider-related keys).
func agentRecordWithoutProviders(a *agentdex.Agent) *record {
	return buildAgentRecord(a, false)
}

func buildAgentRecord(a *agentdex.Agent, includeProviders bool) *record {
	d := a.Detection
	r := newRecord(agentFieldSet)
	r.add("id", a.ID, a.ID)
	r.add("name", a.Name, a.Name)
	// Not-found bin cell is "missing"; JSON value stays blank with found carrying the fact.
	binText := orDash(d.BinaryPath)
	if !d.Found {
		binText = "missing"
	}
	r.add("bin", d.BinaryPath, binText)
	r.add("found", d.Found, fmt.Sprintf("%t", d.Found))
	r.add("config_dir", d.Config.Global, orDash(d.Config.Global))
	// Add order matches agentFieldSet.all so text detail order stays aligned.
	if d.Config.Local != "" {
		r.add("config_local_dir", d.Config.Local, d.Config.Local)
	}
	// skills_dir/skills_local_dir are derived primaries; skills is the full matrix.
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
		// Empty list is text "-", same as other absent scalars; never a blank cell.
		r.add("providers", a.ResolvedProviders, orDash(strings.Join(a.ResolvedProviders, ", ")))
	}
	r.add("homepage", a.Homepage, orDash(a.Homepage))
	return r
}

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

// Compact one-line --fields form; full get detail uses renderSkillsSection.
// Path colour stays on skills_dir scalars only.
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

// withModelsNA marks models not-applicable: JSON null and text "-", distinct from
// withModels's nil→[] degrade shape.
func withModelsNA(r *record) {
	r.add("models", nil, "-")
}

// Present whenever a client was consulted, independent of Models fill.
func withProviderEnv(r *record, env map[string]bool) {
	if env == nil {
		return
	}
	r.add("provider_env", env, formatProviderEnv(env))
}

func withModelCount(r *record, n int) {
	r.add("models", n, fmt.Sprintf("%d", n))
}

// nil becomes [] so JSON matches the "0" cell and `jq '.models|length'` works.
func withModels(r *record, models []modelsdev.Model) {
	if models == nil {
		models = []modelsdev.Model{}
	}
	r.add("models", models, fmt.Sprintf("%d", len(models)))
}

func withAgentModels(r *record, models []agentdex.Model) {
	if models == nil {
		models = []agentdex.Model{}
	}
	r.add("models", models, fmt.Sprintf("%d", len(models)))
}

var modelFieldSet = newFieldSet(
	[]string{"id", "provider", "name", "family", "context", "input", "output", "total", "reasoning", "tool_call", "attachment", "released", "canonical_id"},
	[]string{"id", "name", "context", "input", "output"},
).ordered("released", "released")

// canonical_id is added only when non-empty; selecting it still yields empty
// JSON and text "-" (same as other selected-but-absent fields).
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

// providerFieldSet: set is any-listed-key presence (boolean, orderable); env is
// the name list with optional (set) markers; present is the structured map so
// scripts avoid parsing "(set)". models stays array-typed (cell is the count).
var providerFieldSet = newFieldSet(
	[]string{"id", "name", "set", "env", "present", "models", "doc", "npm", "api"},
	[]string{"id", "name", "set", "env", "models"},
).ordered("id")

// present is resolved at the library boundary so the builder is testable from inputs.
func providerRecord(p modelsdev.Provider, present map[string]bool) *record {
	r := newRecord(providerFieldSet)
	r.add("id", p.ID, p.ID)
	r.add("name", p.Name, p.Name)
	// Copy p.Env so a no-env provider keeps [] rather than null in JSON.
	envNames := make([]string, len(p.Env))
	copy(envNames, p.Env)
	sort.Strings(envNames)
	setVal, setText := providerSetField(envNames, present)
	r.add("set", setVal, setText)
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

// Any-set, not all-set: models.dev env lists have no required/optional metadata.
// No declared names is JSON null / text "-", not a false unset.
func providerSetField(names []string, present map[string]bool) (any, string) {
	if len(names) == 0 {
		return nil, "-"
	}
	for _, k := range names {
		if present[k] {
			return true, "set"
		}
	}
	return false, "unset"
}

// providerEnvCell folds presence for the browse ENV column: set vars get "(set)",
// unset stay bare. Differs from get's symmetric (set)/(unset) to keep wide listings legible.
// No declared env vars is "-", not a blank cell (same as other absent table values).
func providerEnvCell(names []string, present map[string]bool) string {
	parts := make([]string, 0, len(names))
	for _, k := range names {
		if present[k] {
			parts = append(parts, k+" "+plainState("set", true))
			continue
		}
		parts = append(parts, k)
	}
	return orDash(strings.Join(parts, ", "))
}

type costKind int

const (
	costInput costKind = iota
	costOutput
)

func costFor(c *modelsdev.Cost, kind costKind) (float64, bool) {
	if c == nil {
		return 0, false
	}
	if kind == costOutput {
		return c.Output, true
	}
	return c.Input, true
}

// costValue is nil when pricing is unknown so JSON is null rather than a misleading zero.
func costValue(c *modelsdev.Cost, kind costKind) any {
	v, ok := costFor(c, kind)
	if !ok {
		return nil
	}
	return v
}

// costText keeps full precision with trailing zeros trimmed so sub-cent prices stay truthful.
func costText(c *modelsdev.Cost, kind costKind) string {
	v, ok := costFor(c, kind)
	if !ok {
		return "-"
	}
	return "$" + strconv.FormatFloat(v, 'f', -1, 64)
}

// combinedCost rounds away binary float artifacts (0.05+0.025 → 0.07500000000000001)
// at nano-dollar precision, well below any realistic price.
func combinedCost(c *modelsdev.Cost) (float64, bool) {
	if c == nil {
		return 0, false
	}
	return math.Round((c.Input+c.Output)*1e9) / 1e9, true
}

// totalValue is a rough comparison signal (null when unknown so it sorts last), not
// a workload cost: real usage rarely splits input and output evenly.
func totalValue(c *modelsdev.Cost) any {
	v, ok := combinedCost(c)
	if !ok {
		return nil
	}
	return v
}

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

// Plain for table/--fields (no colour codes).
func formatProviderEnv(env map[string]bool) string {
	return providerEnvText(env, plainState)
}

// Detail section: (set) green, (unset) yellow.
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

// Cyan delimiters with green/yellow state text per the start colour standard.
func styledState(state string, good bool) string {
	inner := tui.Warn
	if good {
		inner = tui.Good
	}
	return tui.Delim.Sprint("(") + inner.Sprint(state) + tui.Delim.Sprint(")")
}
