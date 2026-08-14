package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/p3bot/agentdex"
	"github.com/p3bot/agentdex/internal/tui"
	"github.com/p3bot/agentdex/modelsdev"
)

func (a *app) newProvidersCmd() *cobra.Command {
	return a.newNounCmd(
		"providers", "provider", "Model providers from models.dev and their API-key status",
		a.newProvidersListCmd(),
		a.newProvidersGetCmd(),
	)
}

func (a *app) newProvidersListCmd() *cobra.Command {
	var (
		fields  []string
		orderBy string
		reverse bool
	)
	cmd := &cobra.Command{
		Use:   "list [filter]",
		Short: "List model providers from models.dev",
		Long: "List the models.dev providers usable with --provider on agents and models, " +
			"with each provider's id, display name, API-key environment variables and whether they " +
			"are set, and its model count. Rows are ordered by id by default; --order-by sorts by " +
			"any field (for example models for model count) and --reverse flips the direction. The " +
			"optional filter narrows the list to providers whose id or name contains it " +
			"(case-insensitive); it is a browse narrowing, not a selector, so a filter matching " +
			"nothing prints an empty listing and exits 0.",
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
			return a.providersList(cmd, idx, filter, fields, orderBy, reverse)
		},
	}
	registerFieldsFlag(cmd, &fields)
	registerOrderFlags(cmd, &orderBy, &reverse)
	addFieldsHelpSection(cmd, providerFieldSet)
	return cmd
}

// providersList loads no agent catalog; stale models.dev rides as WarnModelsStale.
func (a *app) providersList(cmd *cobra.Command, idx *agentdex.Index, filter string, fields []string, orderBy string, reverse bool) error {
	res, err := idx.Providers.List(cmd.Context(), agentdex.ProviderQuery{Filter: filter})
	warnings := libWarnings(res.Warnings)
	if err != nil {
		return a.fail(cmd, codeFor(err), err, warnings...)
	}

	recs := make([]*record, len(res.Items))
	for i := range res.Items {
		p := res.Items[i]
		recs[i] = providerRecord(p.Provider, p.EnvPresent)
	}
	sortKey, err := applyOrder(recs, providerFieldSet, orderBy, reverse)
	if err != nil {
		return a.usage(cmd, err)
	}

	tableCols := fields
	if len(tableCols) == 0 {
		tableCols = orderColumns(providerFieldSet.defaults, sortKey)
	}
	data, headers, rows, err := tabulate(recs, fields, tableCols, providerFieldSet)
	if err != nil {
		return a.usage(cmd, err)
	}
	empty := emptyListMessage(filter, "providers", "No providers.")
	return a.ok(cmd, data, warnings, func(w io.Writer) {
		fmt.Fprintln(w)
		renderTable(w, headers, rows, empty)
	})
}

func (a *app) newProvidersGetCmd() *cobra.Command {
	var (
		models bool
		fields []string
	)
	cmd := &cobra.Command{
		Use:     "get <id>",
		Aliases: []string{"view", "show"},
		Short:   "Show detail for one models.dev provider",
		Long: "Show detail for one models.dev provider, selected exactly by its provider id: " +
			"its facts (id, name, doc, npm, api), its API-key env presence, and its model count. " +
			"Pass --models or include models in --fields for the full model table (not the count). " +
			"An id that names no provider is not-found (exit 3).",
		Args: exactGetID("a provider id", "provider id", "provider ids"),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, err := a.index(cmd)
			if err != nil {
				return err
			}
			return a.providersGet(cmd, idx, args[0], models, fields)
		},
	}
	cmd.Flags().BoolVar(&models, "models", false, "Fill the per-model table from models.dev")
	registerFieldsFlag(cmd, &fields)
	addFieldsHelpSection(cmd, providerFieldSet)
	return cmd
}

func (a *app) providersGet(cmd *cobra.Command, idx *agentdex.Index, id string, models bool, fields []string) error {
	p, err := idx.Providers.Get(cmd.Context(), id)
	if err != nil {
		return a.fail(cmd, codeFor(err), providersGetError(err))
	}
	present := p.EnvPresent
	// Unlike agents get, models are always in JSON: already in hand from this fetch,
	// so --models governs only the text view and .data.models stays uniform with list.
	r := providerRecord(p.Provider, present)
	fs, err := r.resolve(fields)
	if err != nil {
		return a.usage(cmd, err)
	}
	return a.ok(cmd, jsonObject(fs), nil, func(w io.Writer) {
		if len(fields) > 0 {
			// models in --fields is the full table (same as --models), not the count cell.
			renderProviderSelectedFields(w, fs, p.Provider)
			return
		}
		// Empty fields ⇒ full resolved record.
		renderProviderDetail(w, fs, p.Provider, present, models)
	})
}

// providersGetError appends a CLI-only list remedy. Kept separate from classification
// so codeFor still sees ErrNotFound before wrapping.
func providersGetError(err error) error {
	if errors.Is(err, agentdex.ErrNotFound) {
		return errors.New(err.Error() + "; run \"agentdex providers list\" to see provider ids")
	}
	return err
}

// Scalars rendered inline; env/present/models are sections.
var providerFactFields = map[string]bool{"id": true, "name": true, "doc": true, "npm": true, "api": true}

func renderProviderDetail(w io.Writer, fs []field, p modelsdev.Provider, present map[string]bool, showModels bool) {
	detail := make([]field, 0, len(fs))
	for _, f := range fs {
		if !providerFactFields[f.key] {
			continue
		}
		if f.key == "doc" && f.text != "-" {
			f.text = tui.URL.Sprint(f.text)
		}
		detail = append(detail, f)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, tui.Header.Sprint("Provider"))
	renderDetail(w, detail)

	fmt.Fprintln(w)
	fmt.Fprintln(w, tui.Header.Sprint("Provider env"))
	fmt.Fprintln(w, "  "+styledProviderEnv(present))

	fmt.Fprintln(w)
	fmt.Fprintln(w, tui.Header.Sprint("Models"))
	if !showModels {
		n := len(p.Models)
		noun := "models"
		if n == 1 {
			noun = "model"
		}
		fmt.Fprintf(w, "  %d %s\n", n, noun)
		fmt.Fprintln(w, "  "+tui.Muted.Sprint("pass --models to list them"))
		return
	}
	renderProviderModelsTable(w, p)
}

// --fields path: models expands to the per-model table, matching --models.
// Providers always carry models from the fetch, so expand is unconditional.
func renderProviderSelectedFields(w io.Writer, fs []field, p modelsdev.Provider) {
	renderSelectedFields(w, fs, true, func(w io.Writer) {
		renderProviderModelsTable(w, p)
	})
}

func renderProviderModelsTable(w io.Writer, p modelsdev.Provider) {
	models := make([]modelsdev.Model, 0, len(p.Models))
	for _, key := range sortedKeys(p.Models) {
		models = append(models, p.Models[key])
	}
	modelsdev.SortByRelease(models)
	recs := make([]*record, len(models))
	for i, m := range models {
		recs[i] = modelRecord(m, p.ID, "")
	}
	_, headers, rows, _ := tabulate(recs, nil, modelFieldSet.defaults, modelFieldSet)
	renderTable(w, headers, rows, "(none)")
	if len(rows) > 0 {
		renderPriceFooter(w, modelFieldSet.defaults)
	}
}
