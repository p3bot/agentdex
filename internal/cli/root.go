// Package cli is the agentdex command-line interface: a thin wrapper over the
// agentdex library and the modelsdev client. It owns the cobra command tree, the
// JSON envelope, the exit-code taxonomy, --fields selection, and the
// catalog/models.dev coverage rollup that drives get reporting. It reimplements no
// library behaviour; detection, resolution, the merge, and caching all live in the
// library, and the one piece of CLI-only policy — the coverage rollup — composes
// public library facts rather than reaching past the public API.
package cli

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/p3bot/agentdex"
	"github.com/p3bot/agentdex/internal/config"
	"github.com/p3bot/agentdex/internal/tui"
)

// groupCore keeps real commands together in help so cobra's help/completion stay under Additional Commands.
const groupCore = "core"

type app struct {
	jsonOut      bool
	verbose      bool
	quiet        bool
	color        string
	debug        bool
	printVersion bool
	searchDirs   []string
	binPaths     []string

	cfg    *config.Config
	cfgErr error
	log    *slog.Logger
}

// NewRootCommand builds the agentdex command tree with global flags bound. It is
// the single construction point so tests can drive the CLI with a fresh tree and
// captured output.
func NewRootCommand() *cobra.Command {
	a := &app{}
	root := &cobra.Command{
		Use:   "agentdex",
		Short: "Browse AI coding agents, providers, and models as data",
		Long: "agentdex indexes AI coding agents, the models.dev providers that power " +
			"them, and the models those providers offer, and serves all three as " +
			"browsable data. For an agent it reports the binary, version, config and " +
			"skills directories, providers, and available models, and whether it is " +
			"installed on the local machine.",
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: a.preRun,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if a.printVersion {
				return a.writeVersion(cmd)
			}
			return cmd.Help()
		},
	}

	f := root.PersistentFlags()
	f.BoolVar(&a.jsonOut, "json", false, "Emit a JSON envelope on stdout")
	f.BoolVar(&a.verbose, "verbose", false, "Add detail to output")
	f.BoolVar(&a.quiet, "quiet", false, "Suppress non-essential output")
	f.StringVar(&a.color, "color", "auto", "Colour output: auto, always, never")
	f.BoolVar(&a.debug, "debug", false, "Diagnostic logging to stderr")
	root.Flags().BoolVar(&a.printVersion, "version", false, "Print the agentdex version, commit, and build date")

	root.AddGroup(&cobra.Group{ID: groupCore, Title: "Core Commands:"})
	root.AddCommand(
		a.newAgentsCmd(),
		a.newProvidersCmd(),
		a.newModelsCmd(),
		a.newRefreshCmd(),
		a.newVersionCmd(),
	)
	return root
}

func (a *app) newNounCmd(use, alias, short string, subs ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     use,
		Aliases: []string{alias},
		GroupID: groupCore,
		Short:   short,
		// ArbitraryArgs routes bare/unknown verbs to RunE so the envelope and exit
		// code stay under our control rather than cobra's terse error.
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.nounUsage(cmd, args)
		},
	}
	cmd.AddCommand(subs...)
	return cmd
}

// Short usage error with a help pointer; no full help dump (that is --help). Exit 2.
// --json still gets the envelope alone via a.usage.
func (a *app) nounUsage(cmd *cobra.Command, args []string) error {
	hint := fmt.Sprintf(`run "agentdex %s --help"`, cmd.Name())
	if len(args) > 0 {
		return a.usage(cmd, fmt.Errorf("unknown %s subcommand %q: use list or get; %s", cmd.Name(), args[0], hint))
	}
	return a.usage(cmd, fmt.Errorf("%s requires a subcommand: list or get; %s", cmd.Name(), hint))
}

// Execute runs the command tree and returns the process exit code. Command failures
// arrive as *exitError; cobra-originated errors are usage (exit 2).
func Execute() int {
	root := NewRootCommand()
	err := root.Execute()
	if err == nil {
		return codeOK
	}
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	fmt.Fprintln(os.Stderr, "error: "+err.Error())
	return codeUsage
}

// Malformed config is not fatal here — version and completion must still work —
// so it is stashed and surfaced only by commands that need config. Colour is
// settled here because it applies to every command, including those that ignore config.
func (a *app) preRun(cmd *cobra.Command, _ []string) error {
	switch a.color {
	case "auto", "always", "never":
	default:
		// Shared usage path so --json carries the envelope like every other usage fault.
		return a.usage(cmd, fmt.Errorf("invalid --color %q: want auto, always, or never", a.color))
	}

	a.cfg, a.cfgErr = config.Load(config.Path())

	mode := "auto"
	if a.cfg != nil {
		mode = a.cfg.Color
	}
	if cmd.Flags().Changed("color") {
		mode = a.color
	}
	tui.Configure(mode, os.Stdout)

	level := slog.LevelWarn
	if a.debug {
		level = slog.LevelDebug
	}
	a.log = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	return nil
}

func (a *app) requireConfig() (*config.Config, error) {
	if a.cfgErr != nil {
		return nil, a.cfgErr
	}
	return a.cfg, nil
}

// Open performs no I/O; catalog and models.dev resolve lazily. Errors are already *exitError.
func (a *app) index(cmd *cobra.Command) (*agentdex.Index, error) {
	cfg, err := a.requireConfig()
	if err != nil {
		return nil, a.failConfig(cmd, err)
	}
	flags, err := a.mapFlags()
	if err != nil {
		return nil, a.usage(cmd, err)
	}
	opts := append(cfg.Options(flags), agentdex.WithLogger(a.log))
	idx, err := agentdex.Open(opts...)
	if err != nil {
		return nil, a.fail(cmd, codeFor(err), err)
	}
	return idx, nil
}

// WarnProvidersRequired gets a CLI-only remedy clause naming --provider.
func libWarnings(ws []agentdex.Warning) []string {
	if len(ws) == 0 {
		return nil
	}
	out := make([]string, len(ws))
	for i, w := range ws {
		msg := w.Msg
		if w.Kind == agentdex.WarnProvidersRequired {
			msg += ": supply --provider with models.dev provider ids to enrich providers, provider-env, and models"
		}
		out[i] = msg
	}
	return out
}

func (a *app) failConfig(cmd *cobra.Command, err error) error {
	return a.fail(cmd, codeForConfig(err), err)
}

func (a *app) mapFlags() (config.Flags, error) {
	bin := make(map[string]string, len(a.binPaths))
	for _, entry := range a.binPaths {
		id, path, ok := strings.Cut(entry, "=")
		if !ok || id == "" || path == "" {
			return config.Flags{}, fmt.Errorf("invalid --bin-path %q: want id=path", entry)
		}
		bin[id] = path
	}
	return config.Flags{SearchDirs: a.searchDirs, BinPaths: bin}, nil
}

// JSON envelope under --json; else warnings to stderr then text. --quiet suppresses warnings in text mode.
func (a *app) ok(cmd *cobra.Command, data any, warnings []string, text func(io.Writer)) error {
	if a.jsonOut {
		writeJSON(cmd.OutOrStdout(), envelope{Status: "ok", Data: data, Warnings: warnings})
		return nil
	}
	if !a.quiet {
		emitWarnings(cmd.ErrOrStderr(), warnings)
	}
	if text != nil {
		text(cmd.OutOrStdout())
	}
	return nil
}

// Under --json error and warnings go into the envelope; otherwise to stderr.
// *exitError carries only the code.
func (a *app) fail(cmd *cobra.Command, code int, err error, warnings ...string) error {
	if a.jsonOut {
		writeJSON(cmd.OutOrStdout(), envelope{Status: "error", Error: err.Error(), Warnings: warnings})
	} else {
		if !a.quiet {
			emitWarnings(cmd.ErrOrStderr(), warnings)
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "error: "+err.Error())
	}
	return &exitError{code: code}
}

func (a *app) failData(cmd *cobra.Command, code int, err error, data any, text func(io.Writer), warnings []string) error {
	if a.jsonOut {
		writeJSON(cmd.OutOrStdout(), envelope{Status: "error", Data: data, Error: err.Error(), Warnings: warnings})
		return &exitError{code: code}
	}
	if !a.quiet {
		emitWarnings(cmd.ErrOrStderr(), warnings)
	}
	if text != nil {
		text(cmd.OutOrStdout())
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "error: "+err.Error())
	return &exitError{code: code}
}

func (a *app) usage(cmd *cobra.Command, err error) error {
	return a.fail(cmd, codeUsage, err)
}
