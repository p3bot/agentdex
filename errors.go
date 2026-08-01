package agentdex

import (
	"errors"
	"fmt"

	"github.com/start-cli/agentdex/modelsdev"
)

// Exported sentinels for errors.Is. Detail rides the wrapping message (library-
// owned). ErrModelsSchema aliases modelsdev.ErrModelsSchema (same value).
var (
	// ErrCatalogUnavailable is cold-offline with no fallback. Never raised under WithCatalogDir.
	ErrCatalogUnavailable = errors.New("agentdex catalog unavailable")

	// ErrCatalogInvalid is a module that loaded but failed schema evaluation (data, not network).
	ErrCatalogInvalid = errors.New("agentdex catalog invalid")

	// ErrModelsUnavailable is a non-schema models.dev fetch failure on Providers/Models.
	// Agent operations degrade instead.
	ErrModelsUnavailable = errors.New("models.dev unavailable")

	// ErrModelsSchema is the same value as modelsdev.ErrModelsSchema.
	ErrModelsSchema = modelsdev.ErrModelsSchema

	// ErrAgentUnknown is an agent id absent from the catalog.
	ErrAgentUnknown = errors.New("unknown agent id")

	// ErrUnknownProvider is a caller provider id models.dev does not know.
	ErrUnknownProvider = errors.New("unknown provider id")

	// ErrProvidersRequired is a model listing scoped to an agnostic agent with no providers.
	ErrProvidersRequired = errors.New("providers required for agnostic agent")

	// ErrProvidersNotAllowed is a home-provider agent given an explicit provider set.
	ErrProvidersNotAllowed = errors.New("providers not allowed for home-provider agent")

	// ErrMalformedModelID is a model composite with no "/".
	ErrMalformedModelID = errors.New("malformed model id")

	// ErrNotFound is a provider or model exact-get miss.
	ErrNotFound = errors.New("not found")
)

// wrapped keeps Error() as the library message while Unwrap yields the sentinel.
// fmt.Errorf %w would splice the sentinel text into the message.
type wrapped struct {
	msg      string
	sentinel error
}

func (e *wrapped) Error() string { return e.msg }
func (e *wrapped) Unwrap() error { return e.sentinel }

func errf(sentinel error, format string, args ...any) error {
	return &wrapped{msg: fmt.Sprintf(format, args...), sentinel: sentinel}
}
