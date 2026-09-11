package catalog

import "errors"

var (
	// ErrUnavailable signals that the catalog could not be loaded under the
	// configured Fetch: the registry was unreachable and no fallback was used
	// (none existed, or Fetch forbade using it). The root package wraps this;
	// this sentinel states only the cause.
	ErrUnavailable = errors.New("could not load catalog")

	// ErrInvalidCatalog signals that a fetched catalog module failed to load,
	// validate against its bundled schema, or decode. Distinct from
	// ErrUnavailable: the data was reachable but does not satisfy the contract.
	ErrInvalidCatalog = errors.New("invalid agentdex catalog")
)
