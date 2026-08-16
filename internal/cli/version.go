package cli

import (
	"fmt"
	"io"
	"runtime/debug"

	"github.com/spf13/cobra"
)

const (
	defaultVersion = "dev"
	defaultCommit  = "none"
	defaultDate    = "unknown"
)

// Version, Commit, and Date are injected at build time via ldflags into this
// package. The defaults make a plain `go build` self-describing as a dev build
// rather than printing empty fields. writeVersion fills any leftover default
// from debug.ReadBuildInfo so go install is not anonymous.
var (
	Version = defaultVersion
	Commit  = defaultCommit
	Date    = defaultDate
)

func resolveIdentity(version, commit, date string, read func() (*debug.BuildInfo, bool)) (string, string, string) {
	if version != defaultVersion && commit != defaultCommit && date != defaultDate {
		return version, commit, date
	}
	info, ok := read()
	if !ok || info == nil {
		return version, commit, date
	}
	if version == defaultVersion {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			version = v
		}
	}
	var revision, modified, vcsTime string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value
		case "vcs.time":
			vcsTime = s.Value
		}
	}
	if commit == defaultCommit && revision != "" {
		commit = revision
		if modified == "true" {
			commit += "+dirty"
		}
	}
	if date == defaultDate && vcsTime != "" {
		date = vcsTime
	}
	return version, commit, date
}

func versionParts() (version, commit, date string) {
	return resolveIdentity(Version, Commit, Date, debug.ReadBuildInfo)
}

func versionBanner() string {
	v, c, d := versionParts()
	return fmt.Sprintf("agentdex %s (commit %s, built %s)", v, c, d)
}

func (a *app) writeVersion(cmd *cobra.Command) error {
	version, commit, date := versionParts()
	data := map[string]any{"version": version, "commit": commit, "date": date}
	return a.ok(cmd, data, nil, func(w io.Writer) {
		fmt.Fprintln(w, versionBanner())
	})
}

func (a *app) newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		GroupID: groupCore,
		Short:   "Print the agentdex version, commit, and build date",
		Args:    noPositionalArgs(),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.writeVersion(cmd)
		},
	}
}
