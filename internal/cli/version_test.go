package cli

import (
	"runtime/debug"
	"testing"
)

func TestResolveIdentity(t *testing.T) {
	info := func(version string, settings ...debug.BuildSetting) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{
				Main:     debug.Module{Version: version},
				Settings: settings,
			}, true
		}
	}
	none := func() (*debug.BuildInfo, bool) { return nil, false }

	rev := debug.BuildSetting{Key: "vcs.revision", Value: "deadbeef"}
	when := debug.BuildSetting{Key: "vcs.time", Value: "2026-08-16T00:00:00Z"}
	dirty := debug.BuildSetting{Key: "vcs.modified", Value: "true"}
	clean := debug.BuildSetting{Key: "vcs.modified", Value: "false"}

	for _, tc := range []struct {
		name                              string
		version, commit, date             string
		read                              func() (*debug.BuildInfo, bool)
		wantVersion, wantCommit, wantDate string
	}{
		{
			name:        "ldflags win",
			version:     "v1.0.0",
			commit:      "abc",
			date:        "2026-01-01",
			read:        info("v9.9.9", rev, when),
			wantVersion: "v1.0.0",
			wantCommit:  "abc",
			wantDate:    "2026-01-01",
		},
		{
			name:        "go install module version",
			version:     defaultVersion,
			commit:      defaultCommit,
			date:        defaultDate,
			read:        info("v1.0.1-0.20260816052814-d255f8387667"),
			wantVersion: "v1.0.1-0.20260816052814-d255f8387667",
			wantCommit:  defaultCommit,
			wantDate:    defaultDate,
		},
		{
			name:        "devel keeps dev and takes vcs",
			version:     defaultVersion,
			commit:      defaultCommit,
			date:        defaultDate,
			read:        info("(devel)", rev, when, clean),
			wantVersion: defaultVersion,
			wantCommit:  "deadbeef",
			wantDate:    "2026-08-16T00:00:00Z",
		},
		{
			name:        "dirty suffix",
			version:     defaultVersion,
			commit:      defaultCommit,
			date:        defaultDate,
			read:        info("(devel)", rev, dirty),
			wantVersion: defaultVersion,
			wantCommit:  "deadbeef+dirty",
			wantDate:    defaultDate,
		},
		{
			name:        "no build info",
			version:     defaultVersion,
			commit:      defaultCommit,
			date:        defaultDate,
			read:        none,
			wantVersion: defaultVersion,
			wantCommit:  defaultCommit,
			wantDate:    defaultDate,
		},
		{
			name:        "empty main version",
			version:     defaultVersion,
			commit:      defaultCommit,
			date:        defaultDate,
			read:        info(""),
			wantVersion: defaultVersion,
			wantCommit:  defaultCommit,
			wantDate:    defaultDate,
		},
		{
			name:        "partial ldflags keep version fill commit",
			version:     "v1.2.3",
			commit:      defaultCommit,
			date:        defaultDate,
			read:        info("v9.9.9", rev),
			wantVersion: "v1.2.3",
			wantCommit:  "deadbeef",
			wantDate:    defaultDate,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotV, gotC, gotD := resolveIdentity(tc.version, tc.commit, tc.date, tc.read)
			if gotV != tc.wantVersion || gotC != tc.wantCommit || gotD != tc.wantDate {
				t.Errorf("resolveIdentity = %q %q %q, want %q %q %q",
					gotV, gotC, gotD, tc.wantVersion, tc.wantCommit, tc.wantDate)
			}
		})
	}
}

func TestVersionCommandUsesResolvedIdentity(t *testing.T) {
	got := runCLI("version")
	if got.code != codeOK {
		t.Fatalf("version exit = %d, want 0; stderr=%q", got.code, got.stderr)
	}
	want := versionBanner() + "\n"
	if got.stdout != want {
		t.Errorf("version stdout = %q, want %q", got.stdout, want)
	}
}
