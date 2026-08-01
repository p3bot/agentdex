// Command agentdex is the thin CLI over the agentdex detection library.
// Version/commit/date are ldflags into internal/cli (Version, Commit, Date).
package main

import (
	"os"

	"github.com/start-cli/agentdex/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
