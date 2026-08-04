package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/p3bot/agentdex"
)

// refreshTargets is the single source for target validation, unknown-target error,
// Targets help, and success notes.
var refreshTargets = []struct{ name, desc, note string }{
	{"catalog", "Re-resolve the agentdex catalog version", "Refreshed agentdex catalog (agent data)"},
	{"models.dev", "Refetch the models.dev catalog", "Refreshed models.dev catalog (provider and model data)"},
	{"all", "Both (default)", ""},
}

func refreshTargetFor(name string) agentdex.Target {
	switch name {
	case "catalog":
		return agentdex.TargetCatalog
	case "models.dev":
		return agentdex.TargetModels
	default:
		return agentdex.TargetAll
	}
}

func validRefreshTarget(name string) bool {
	for _, t := range refreshTargets {
		if t.name == name {
			return true
		}
	}
	return false
}

func refreshNote(name string) string {
	for _, t := range refreshTargets {
		if t.name == name {
			return t.note
		}
	}
	return ""
}

// Oxford-style list ("a, b, or c") for the unknown-target error.
func refreshTargetList() string {
	names := make([]string, len(refreshTargets))
	for i, t := range refreshTargets {
		names[i] = t.name
	}
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

func (a *app) newRefreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "refresh [target]",
		GroupID: groupCore,
		Short:   "Force a refresh: " + refreshTargetList(),
		Long: "Force a refresh of the agentdex catalog (agent data) and/or the " +
			"models.dev catalog (provider and model data). The target defaults to all.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, err := a.index(cmd)
			if err != nil {
				return err
			}
			target := "all"
			if len(args) == 1 {
				target = args[0]
			}
			if !validRefreshTarget(target) {
				return a.usage(cmd, fmt.Errorf("unknown refresh target %q: want %s", target, refreshTargetList()))
			}

			// Library owns sequencing and which targets refreshed; CLI maps names and renders.
			done, err := idx.Refresh(cmd.Context(), refreshTargetFor(target))
			if err != nil {
				return a.fail(cmd, codeFor(err), err)
			}
			var refreshed []string
			if done.Catalog {
				refreshed = append(refreshed, "catalog")
			}
			if done.Models {
				refreshed = append(refreshed, "models.dev")
			}

			data := map[string]any{"refreshed": refreshed}
			return a.ok(cmd, data, nil, func(w io.Writer) {
				for _, r := range refreshed {
					fmt.Fprintln(w, refreshNote(r))
				}
			})
		},
	}
	// Targets help section, derived from refreshTargets so it cannot drift.
	width := 0
	for _, t := range refreshTargets {
		if len(t.name) > width {
			width = len(t.name)
		}
	}
	var body strings.Builder
	for _, t := range refreshTargets {
		fmt.Fprintf(&body, "  %-*s  %s\n", width, t.name, t.desc)
	}
	addHelpSection(cmd, "Targets", strings.TrimRight(body.String(), "\n"))
	return cmd
}
