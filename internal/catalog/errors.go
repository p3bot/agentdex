package catalog

import "errors"

var (
	// ErrUnavailable signals that the catalog could not be loaded at all: the
	// registry was unreachable and no cached version or module data remained.
	// The root package wraps this; this sentinel states only the cause.
	ErrUnavailable = errors.New("no cached fallback available")

	// ErrInvalidCatalog signals that a fetched catalog module failed to load,
	// validate against its bundled schema, or decode. Distinct from
	// ErrUnavailable: the data was reachable but does not satisfy the contract.
	ErrInvalidCatalog = errors.New("invalid agentdex catalog")
)
