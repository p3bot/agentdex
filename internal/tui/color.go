// Package tui renders agentdex's human-facing text output: NO_COLOR-aware colour
// styling and aligned tables. It is presentation only; commands decide what to
// show and hand strings here to format. Colour is controlled by Configure, which
// must be called once before any styled output.
package tui

import (
	"os"

	"github.com/fatih/color"
	"golang.org/x/term"
)

// Configure sets colour output according to the --color mode, honouring NO_COLOR
// and terminal detection in auto mode. Process-wide; call once at startup.
//
//   - "always": colour on regardless of TTY or NO_COLOR
//   - "never":  colour off
//   - anything else ("auto"): colour on only when out is a terminal and NO_COLOR
//     is unset
//
// fatih/color stamps each Color with a per-instance noColor flag when NO_COLOR is
// set at construction, so flipping only the package global is not enough for
// "always" to override NO_COLOR; Configure drives both.
func Configure(mode string, out *os.File) {
	on := false
	switch mode {
	case "always":
		on = true
	case "never":
		on = false
	default:
		_, noColor := os.LookupEnv("NO_COLOR")
		on = !noColor && term.IsTerminal(int(out.Fd()))
	}
	color.NoColor = !on
	for _, s := range styles() {
		if on {
			s.EnableColor()
		} else {
			s.DisableColor()
		}
	}
}

// Package styles; Configure sets both the global toggle and each style's
// per-instance flag so mode wins over init-time NO_COLOR stamping.
var (
	Header = color.New(color.Bold, color.FgGreen)
	Label  = color.New(color.FgCyan)
	Muted  = color.New(color.Faint)
	Warn   = color.New(color.FgYellow)
	// White, not HiCyan: field labels are already cyan, and two cyans side by
	// side on every path line read as one.
	Path  = color.New(color.FgHiWhite)
	Good  = color.New(color.FgGreen)
	Delim = color.New(color.FgCyan)
	URL   = color.New(color.FgBlue)
)

func styles() []*color.Color {
	return []*color.Color{Header, Label, Muted, Warn, Path, Good, Delim, URL}
}
