package cli

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/start-cli/agentdex"
	"github.com/start-cli/agentdex/internal/config"
	"github.com/start-cli/agentdex/modelsdev"
)

// Exit codes are agentdex's taxonomy, shared with the wider start CLI. Commands
// classify failures into these rather than inventing per-command codes.
const (
	codeOK         = 0
	codeFailure    = 1
	codeUsage      = 2
	codeNotFound   = 3
	codePermission = 4
	codeConflict   = 5
	codeTransient  = 75
	codeConfig     = 78
)

// exitError carries the process exit code. Its message is already rendered by the
// time it is returned, so Execute and main print nothing further.
type exitError struct {
	code int
}

func (e *exitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

// ExitCode satisfies the convention other tooling looks for.
func (e *exitError) ExitCode() int { return e.code }

// Validity → 78, permission → 4, other read failure → 1. config.Load preserves
// the OS error so permission resolves through errors.Is without naming exit codes.
func codeForConfig(err error) int {
	switch {
	case errors.Is(err, config.ErrConfig):
		return codeConfig
	case errors.Is(err, fs.ErrPermission):
		return codePermission
	default:
		return codeFailure
	}
}

// Single classifier: malformed models.dev is config, never transient; outage is transient.
func codeFor(err error) int {
	switch {
	case errors.Is(err, config.ErrConfig),
		errors.Is(err, agentdex.ErrCatalogInvalid),
		errors.Is(err, modelsdev.ErrModelsSchema):
		return codeConfig
	case errors.Is(err, agentdex.ErrCatalogUnavailable),
		errors.Is(err, agentdex.ErrModelsUnavailable):
		return codeTransient
	case errors.Is(err, agentdex.ErrAgentUnknown),
		errors.Is(err, agentdex.ErrNotFound):
		return codeNotFound
	case errors.Is(err, agentdex.ErrUnknownProvider),
		errors.Is(err, agentdex.ErrProvidersRequired),
		errors.Is(err, agentdex.ErrProvidersNotAllowed),
		errors.Is(err, agentdex.ErrMalformedModelID):
		return codeUsage
	default:
		return codeFailure
	}
}
