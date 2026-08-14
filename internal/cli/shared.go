package cli

import (
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

// flattenProviders drops empties and duplicates so a repeated id cannot double-list
// models or break unique query resolution.
func flattenProviders(raw []string) []string {
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
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

// Singular --field is an invisible alias.
func registerFieldsFlag(cmd *cobra.Command, fields *[]string) {
	cmd.Flags().StringSliceVar(fields, "fields", nil, "Select output fields (csv)")
	cmd.Flags().SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "field" {
			name = "fields"
		}
		return pflag.NormalizedName(name)
	})
}
