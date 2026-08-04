package agentdex

import (
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/p3bot/agentdex/internal/catalog"
)

// Best-effort: a slow or hung binary must not stall detection.
const versionTimeout = 5 * time.Second

// Bounds memory if a binary floods output; timeout alone does not cap writes.
const maxVersionOutput = 64 << 10 // 64 KiB per stream

// probeVersion combines stdout and stderr (some CLIs print version on stderr).
// Failures are non-fatal: empty version, not a detection error. Path is used as
// given, never re-resolved through PATH.
func probeVersion(ctx context.Context, binPath string, vp catalog.VersionProbe) string {
	ctx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, vp.Args...)
	stdout := &cappedBuffer{cap: maxVersionOutput}
	stderr := &cappedBuffer{cap: maxVersionOutput}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	_ = cmd.Run() // non-fatal; parse whatever output exists

	return extractVersion(stdout.String()+stderr.String(), vp.Pattern)
}

// Reports a full write so the child is never blocked by the cap.
type cappedBuffer struct {
	buf bytes.Buffer
	cap int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if room := c.cap - c.buf.Len(); room > 0 {
		if len(p) > room {
			c.buf.Write(p[:room])
		} else {
			c.buf.Write(p)
		}
	}
	return len(p), nil
}

func (c *cappedBuffer) String() string { return c.buf.String() }

// First capture group when present, else whole match; empty on no match or bad pattern.
func extractVersion(output, pattern string) string {
	output = strings.TrimSpace(output)
	if pattern == "" {
		return output
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Catalog authoring error: treat as non-match so probing stays non-fatal.
		// Pattern validity is the module's publish-time job, not runtime.
		return ""
	}
	m := re.FindStringSubmatch(output)
	if m == nil {
		return ""
	}
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(m[0])
}
