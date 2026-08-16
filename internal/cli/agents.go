package cli

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/p3bot/agentdex"
	"github.com/p3bot/agentdex/internal/tui"
)

func (a *app) newAgentsCmd() *cobra.Command {
	cmd := a.newNounCmd(
		"agents", "agent", "AI coding agents in the catalog and their local detection",
		a.newAgentsListCmd(),
		a.newAgentsGetCmd(),
	)
	// Binary-resolution flags belong here, not root: providers/models resolve no binary.
	f := cmd.PersistentFlags()
	// StringArray, not StringSlice: paths may contain commas and must not be csv-split.
	f.StringArrayVar(&a.searchDirs, "search-dir", nil, "Extra binary search locations (repeatable)")
	f.StringArrayVar(&a.binPaths, "bin-path", nil, "Override an agent's binary path as id=path (repeatable)")
	return cmd
}

func (a *app) newAgentsListCmd() *cobra.Command {
	var (
		installed bool
		fields    []string
		providers []string
		orderBy   string
		reverse   bool
	)
	cmd := &cobra.Command{
		Use:   "list [filter]",
		Short: "List AI coding agents",
		Long: "List catalogued AI coding agents and their local detection status. The BIN " +
			"column shows the resolved binary or \"missing\". The models column is a count from " +
			"models.dev (the same number in --json, null when not applicable) and is served " +
			"from cache when warm, degrading when models.dev cannot be reached. An optional " +
			"filter narrows the list to agents whose id or name contains it (case-insensitive); " +
			"a filter matching nothing prints an empty listing and exits 0.",
		Args: atMostOne("filter"),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, err := a.index(cmd)
			if err != nil {
				return err
			}
			filter := ""
			if len(args) == 1 {
				filter = args[0]
			}
			res, err := idx.Agents.List(cmd.Context(), agentdex.AgentQuery{
				Filter:    filter,
				Installed: installed,
				Providers: flattenProviders(providers),
				Enrich:    agentdex.EnrichCount,
			})
			warnings := libWarnings(res.Warnings)
			if err != nil {
				return a.fail(cmd, codeFor(err), withProviderList(err), warnings...)
			}
			a.log.Debug("list agents", "count", len(res.Items), "installed", installed, "filter", filter)

			recs := make([]*record, len(res.Items))
			for i := range res.Items {
				ag := &res.Items[i]
				r := agentRecord(ag)
				if ag.Enrichment == agentdex.EnrichmentNotApplicable {
					// Not-applicable is JSON null / text "-", not the degrade 0 shape.
					withModelsNA(r)
				} else {
					withModelCount(r, ag.ModelCount)
				}
				recs[i] = r
			}
			sortKey, err := applyOrder(recs, agentFieldSet, orderBy, reverse)
			if err != nil {
				return a.usage(cmd, err)
			}
			if orderBy == "" {
				// Default view groups found ahead of missing; stable sort keeps id order
				// within each group. Explicit --order-by is a pure field sort.
				sort.SliceStable(recs, func(i, j int) bool { return recordFound(recs[i]) && !recordFound(recs[j]) })
			}

			// Text-table only: sort key moves leftmost. JSON always carries the
			// full record; explicit --fields wins over the default column set.
			tableCols := fields
			if len(tableCols) == 0 {
				tableCols = orderColumns(agentFieldSet.defaults, sortKey)
			}

			data, headers, rows, err := tabulate(recs, fields, tableCols, agentFieldSet)
			if err != nil {
				return a.usage(cmd, err)
			}
			fallback := "No agents catalogued."
			noun := "agents"
			if installed {
				fallback = "No agents detected."
				noun = "installed agents"
			}
			empty := emptyListMessage(filter, noun, fallback)
			return a.ok(cmd, data, warnings, func(w io.Writer) {
				fmt.Fprintln(w)
				renderTable(w, headers, rows, empty)
			})
		},
	}
	cmd.Flags().BoolVar(&installed, "installed", false, "Limit to agents detected on this machine")
	cmd.Flags().StringSliceVar(&providers, "provider", nil, "models.dev provider ids for agnostic agents (repeatable or csv); without this, those agents show \"-\" for providers and models")
	registerFieldsFlag(cmd, &fields)
	registerOrderFlags(cmd, &orderBy, &reverse, "Sort rows by this field (default: detected agents first, then id; an explicit value drops that grouping)")
	addFieldsHelpSection(cmd, agentFieldSet)
	return cmd
}

func recordFound(r *record) bool {
	found, _ := r.value("found").(bool)
	return found
}

func (a *app) newAgentsGetCmd() *cobra.Command {
	var (
		models    bool
		fields    []string
		providers []string
	)
	cmd := &cobra.Command{
		Use:     "get <id>",
		Aliases: []string{"view", "show"},
		Short:   "Show detail for one agent",
		Long: "Show detection detail for one agent, selected exactly by its catalog id: its " +
			"binary, config and skills paths, and provider-env presence. Models are " +
			"off by default; pass --models or include models in --fields for the full per-model " +
			"list (not the list-column count). Provider-agnostic agents omit provider fields " +
			"until --provider is supplied. An id that names no catalogued agent is not-found " +
			"(exit 3).",
		Args: exactGetID("an agent id", "agent id", "agent ids"),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, err := a.index(cmd)
			if err != nil {
				return err
			}
			id := args[0]

			detail, err := idx.Agents.Get(cmd.Context(), id, agentdex.AgentGetQuery{
				Providers: flattenProviders(providers),
				Enrich:    agentGetLevel(models, fields),
			})
			warnings := libWarnings(detail.Warnings)
			if err != nil {
				return a.fail(cmd, codeFor(err), agentGetError(err), warnings...)
			}

			// Agnostic without providers: soft-path browse at exit 0. Naming an
			// unfillable field/flag is a usage fault instead.
			if detail.Enrichment == agentdex.EnrichmentNotApplicable {
				if namedProviderField(models, fields) {
					uerr := fmt.Errorf("%w: %q is provider-agnostic; supply --provider with models.dev provider ids", agentdex.ErrProvidersRequired, id)
					return a.fail(cmd, codeUsage, uerr, warnings...)
				}
				return a.reportSoftPathAgent(cmd, &detail.Agent, warnings)
			}

			// Coverage data faults: report the agent then exit 78; the library never fails on them.
			switch detail.Coverage.Status {
			case agentdex.CoverageNonePresent:
				cerr := fmt.Errorf("catalog data error: no provider of %q is present in models.dev (providers: %s)", id, strings.Join(detail.ResolvedProviders, ", "))
				return a.reportAgentError(cmd, &detail.Agent, fields, codeConfig, cerr, warnings)
			case agentdex.CoverageSchemaDrift:
				return a.reportAgentError(cmd, &detail.Agent, fields, codeConfig, detail.Coverage.Err, warnings)
			case agentdex.CoverageNotProbed, agentdex.CoverageAllPresent,
				agentdex.CoverageSomePresent, agentdex.CoverageUnreachable:
				// Not data faults: not-probed skipped models.dev, some-absent rides on
				// coverage data, outage is a warning. New fault verdicts must land here.
			}
			return a.reportAgent(cmd, &detail.Agent, fields, warnings)
		},
	}
	cmd.Flags().BoolVar(&models, "models", false, "Fill the per-model list from models.dev")
	cmd.Flags().StringSliceVar(&providers, "provider", nil, "models.dev provider ids (repeatable or csv); required for agnostic agents, a subset of catalog providers for a home-provider agent")
	registerFieldsFlag(cmd, &fields)
	addFieldsHelpSection(cmd, agentFieldSet)
	return cmd
}

// Lowest enrichment that fills the demand so field selection never pays for unused data.
func agentGetLevel(models bool, fields []string) agentdex.Enrich {
	switch {
	case models || containsField(fields, "models"):
		return agentdex.EnrichFull
	case len(fields) == 0 || containsField(fields, "provider_env"):
		return agentdex.EnrichCount
	case containsField(fields, "providers"):
		return agentdex.EnrichProviders
	default:
		return agentdex.EnrichNone
	}
}

// namedProviderField is true when output explicitly names a not-applicable empty
// field (providers, provider_env, models, or --models). Unfiltered detail names none.
func namedProviderField(models bool, fields []string) bool {
	return models || containsField(fields, "providers") ||
		containsField(fields, "provider_env") || containsField(fields, "models")
}

func containsField(fields []string, key string) bool {
	return slices.Contains(fields, key)
}

// agentGetError appends CLI-only remedy clauses (subcommand/flag names the library
// does not own). Exit code is taken from the sentinel before wrapping.
func agentGetError(err error) error {
	switch {
	case errors.Is(err, agentdex.ErrAgentUnknown):
		return errors.New(err.Error() + "; run \"agentdex agents list\" to see agent ids")
	case errors.Is(err, agentdex.ErrProvidersNotAllowed):
		return errors.New(err.Error() + "; --provider must be a subset of the agent's catalog providers")
	default:
		return withProviderList(err)
	}
}

func (a *app) reportAgent(cmd *cobra.Command, agent *agentdex.Agent, fields, warnings []string) error {
	r := agentReportRecord(agent, fields)
	fs, err := r.resolve(fields)
	if err != nil {
		return a.usage(cmd, err)
	}
	return a.ok(cmd, jsonObject(fs), warnings, func(w io.Writer) {
		if len(fields) > 0 {
			// models in --fields is the full list (same as --models), not the count cell.
			renderAgentSelectedFields(w, agent, fs)
			return
		}
		renderAgentDetail(w, agent)
	})
}

// Data-fault rows that surface the agent and still exit non-zero.
func (a *app) reportAgentError(cmd *cobra.Command, agent *agentdex.Agent, fields []string, code int, cause error, warnings []string) error {
	r := agentReportRecord(agent, fields)
	fs, ferr := r.resolve(fields)
	if ferr != nil {
		return a.usage(cmd, ferr)
	}
	return a.failData(cmd, code, cause, jsonObject(fs), func(w io.Writer) {
		if len(fields) > 0 {
			renderAgentSelectedFields(w, agent, fs)
			return
		}
		renderAgentDetail(w, agent)
	}, warnings)
}

func agentReportRecord(agent *agentdex.Agent, fields []string) *record {
	r := agentRecord(agent)
	withProviderEnv(r, agent.ProviderEnv)
	switch {
	case agent.Models != nil:
		withAgentModels(r, agent.Models)
	case containsField(fields, "models"):
		// Selected but unfilled (degrade): JSON null / text "-", not "".
		withModelsNA(r)
	}
	return r
}

// Not-applicable (agnostic, no provider) at exit 0: without-providers record so
// the three provider-related keys are absent.
func (a *app) reportSoftPathAgent(cmd *cobra.Command, agent *agentdex.Agent, warnings []string) error {
	r := agentRecordWithoutProviders(agent)
	fs, _ := r.resolve(nil)
	return a.ok(cmd, jsonObject(fs), warnings, func(w io.Writer) {
		renderAgentDetailFields(w, r, agent)
		renderSkillsSection(w, agent.Detection.Skills)
	})
}

var detailSections = map[string]bool{"skills": true, "provider_env": true, "models": true}

// pathFields get tui.Path styling in the text detail only so table/--fields stay plain.
var pathFields = map[string]bool{
	"bin": true, "config_dir": true, "config_local_dir": true,
	"skills_dir": true, "skills_local_dir": true,
}

// renderAgentDetailFields writes the Agent heading and inline scalars from the
// record in declared order. found is omitted — bin already states presence.
func renderAgentDetailFields(w io.Writer, r *record, agent *agentdex.Agent) {
	fs, _ := r.resolve(nil)
	detail := make([]field, 0, len(fs))
	for _, f := range fs {
		if detailSections[f.key] || f.key == "found" {
			continue
		}
		if pathFields[f.key] && f.text != "-" && f.text != "missing" {
			f.text = tui.Path.Sprint(f.text)
		}
		if f.key == "homepage" && f.text != "-" {
			f.text = tui.URL.Sprint(f.text)
		}
		// Bin always states presence, mirroring provider env's (set)/(unset).
		if f.key == "bin" {
			if agent.Detection.Found {
				f.text += " " + styledState("found", true)
			} else {
				f.text = tui.Warn.Sprint(f.text)
			}
		}
		detail = append(detail, f)
	}
	// Leading blank line sets the first heading off from the shell prompt.
	fmt.Fprintln(w)
	fmt.Fprintln(w, tui.Header.Sprint("Agent"))
	renderDetail(w, detail)
}

func renderAgentDetail(w io.Writer, agent *agentdex.Agent) {
	renderAgentDetailFields(w, agentReportRecord(agent, nil), agent)
	renderSkillsSection(w, agent.Detection.Skills)

	if agent.ProviderEnv != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, tui.Header.Sprint("Provider env"))
		fmt.Fprintln(w, "  "+styledProviderEnv(agent.ProviderEnv))
	}
	if len(agent.Models) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, tui.Header.Sprint("Models"))
		renderAgentModelsTable(w, agent.Models)
	}
}

// --fields path: models expands to the per-model table (same data as --models), not the
// list-column count. Expand only when models were filled; absent/N/A keep scalar text.
func renderAgentSelectedFields(w io.Writer, agent *agentdex.Agent, fs []field) {
	renderSelectedFields(w, fs, agent.Models != nil, func(w io.Writer) {
		renderAgentModelsTable(w, agent.Models)
	})
}

func renderAgentModelsTable(w io.Writer, models []agentdex.Model) {
	recs := make([]*record, len(models))
	for i, m := range models {
		recs[i] = modelRecord(m.Model, m.Provider, m.CanonicalID)
	}
	_, headers, rows, _ := tabulate(recs, nil, modelFieldSet.defaults, modelFieldSet)
	renderTable(w, headers, rows, "(none)")
	if len(rows) > 0 {
		renderPriceFooter(w, modelFieldSet.defaults)
	}
}

// renderSkillsSection lists roles per scope. Primary is omitted — it is already
// skills_dir / skills_local_dir on the Agent block.
func renderSkillsSection(w io.Writer, sp agentdex.SkillsPaths) {
	payload, _ := skillsField(sp)
	if payload == nil {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, tui.Header.Sprint("Skills"))
	if payload.Global != nil {
		renderSkillsScopeBlock(w, "global", payload.Global)
	}
	if payload.Local != nil {
		renderSkillsScopeBlock(w, "local", payload.Local)
	}
}

func renderSkillsScopeBlock(w io.Writer, scope string, s *skillsScopePayload) {
	fmt.Fprintln(w, "  "+scope)
	type line struct {
		role string
		text string
	}
	var lines []line
	if s.Agents != nil {
		lines = append(lines, line{"agents", formatSkillsPathText(s.Agents)})
	}
	if s.Native != nil {
		lines = append(lines, line{"native", formatSkillsPathText(s.Native)})
	}
	if len(s.Alternatives) > 0 {
		alts := make([]string, len(s.Alternatives))
		for i := range s.Alternatives {
			alts[i] = formatSkillsPathText(&s.Alternatives[i])
		}
		lines = append(lines, line{"alternatives", strings.Join(alts, ", ")})
	}
	width := 0
	for _, l := range lines {
		if n := len(l.role); n > width {
			width = n
		}
	}
	for _, l := range lines {
		fmt.Fprintf(w, "    %s  %s\n", padRight(l.role, width), l.text)
	}
}
