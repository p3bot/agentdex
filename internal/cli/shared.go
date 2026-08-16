package cli

import (
	"encoding/csv"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/p3bot/agentdex"
)

func quoteArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = fmt.Sprintf("%q", a)
	}
	return strings.Join(quoted, " ")
}

// Oxford-style list ("a, b, or c") for usage errors that name a valid set.
func orList(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " or " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + ", or " + names[len(names)-1]
	}
}

func commandPath(cmd *cobra.Command) string {
	return strings.TrimPrefix(cmd.CommandPath(), "agentdex ")
}

// ValidateArgs runs before preRun, so a missing get id stays offline.
func exactGetID(required, kind, listWhat string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		noun := cmd.Parent().Name()
		switch len(args) {
		case 1:
			return nil
		case 0:
			return fmt.Errorf("%s get requires %s; run \"agentdex %s list\" to see %s", noun, required, noun, listWhat)
		default:
			return fmt.Errorf("%s get takes one %s, got %s; run \"agentdex %s get --help\"", noun, kind, quoteArgs(args), noun)
		}
	}
}

func atMostOne(kind string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) <= 1 {
			return nil
		}
		path := commandPath(cmd)
		return fmt.Errorf("%s takes at most one %s, got %s; run \"agentdex %s --help\"", path, kind, quoteArgs(args), path)
	}
}

func noPositionalArgs() cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		path := commandPath(cmd)
		return fmt.Errorf("%s takes no arguments, got %s; run \"agentdex %s --help\"", path, quoteArgs(args), path)
	}
}

func withProviderList(err error) error {
	if errors.Is(err, agentdex.ErrUnknownProvider) {
		return errors.New(err.Error() + "; run \"agentdex providers list\" to see provider ids")
	}
	return err
}

// Non-empty filter names itself so narrowing is distinguishable from a genuine empty catalog.
func emptyListMessage(filter, noun, fallback string) string {
	if filter != "" {
		return fmt.Sprintf("No %s match %q.", noun, filter)
	}
	return fallback
}

// csvTokens trims pflag StringSlice pieces and drops empties so "a, b" and "a,,b"
// are the same list. pflag splits on commas and keeps surrounding spaces.
func csvTokens(raw []string) []string {
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

// flattenProviders drops empties and duplicates so a repeated id cannot double-list
// models or break unique query resolution.
func flattenProviders(raw []string) []string {
	raw = csvTokens(raw)
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, p := range raw {
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func addHelpSection(cmd *cobra.Command, title, body string) {
	section := "\n\n" + title + ":\n" + body
	tmpl := strings.Replace(cmd.UsageTemplate(),
		"{{if .HasAvailableInheritedFlags}}",
		section+"{{if .HasAvailableInheritedFlags}}", 1)
	cmd.SetUsageTemplate(tmpl)
}

func addFieldsHelpSection(cmd *cobra.Command, set fieldSet) {
	// Two indented rows keep the section compact rather than one wide line.
	half := (len(set.all) + 1) / 2
	rows := "  " + strings.Join(set.all[:half], ", ") + "\n  " + strings.Join(set.all[half:], ", ")
	addHelpSection(cmd, "Fields", rows)
}

// fieldsValue is a StringSlice that trims tokens and drops empties on Set, so
// --fields "id, name" is ["id","name"] before any command reads the slice.
type fieldsValue struct {
	value   *[]string
	changed bool
}

func newFieldsValue(p *[]string) *fieldsValue {
	return &fieldsValue{value: p}
}

func readFieldsCSV(val string) ([]string, error) {
	if val == "" {
		return []string{}, nil
	}
	return csv.NewReader(strings.NewReader(val)).Read()
}

func (f *fieldsValue) Set(val string) error {
	parts, err := readFieldsCSV(val)
	if err != nil {
		return err
	}
	parts = csvTokens(parts)
	if !f.changed {
		*f.value = parts
	} else {
		*f.value = append(*f.value, parts...)
	}
	f.changed = true
	return nil
}

func (f *fieldsValue) Type() string { return "stringSlice" }

func (f *fieldsValue) String() string {
	if f.value == nil || len(*f.value) == 0 {
		return ""
	}
	return "[" + strings.Join(*f.value, ",") + "]"
}

func (f *fieldsValue) Append(val string) error {
	*f.value = append(*f.value, csvTokens([]string{val})...)
	return nil
}

func (f *fieldsValue) Replace(val []string) error {
	*f.value = csvTokens(val)
	return nil
}

func (f *fieldsValue) GetSlice() []string {
	if f.value == nil {
		return nil
	}
	return *f.value
}

// Singular --field is an invisible alias.
func registerFieldsFlag(cmd *cobra.Command, fields *[]string) {
	cmd.Flags().Var(newFieldsValue(fields), "fields", "Select output fields (csv)")
	cmd.Flags().SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "field" {
			name = "fields"
		}
		return pflag.NormalizedName(name)
	})
}
