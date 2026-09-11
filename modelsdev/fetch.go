package modelsdev

import "fmt"

// Fetch is when a catalog.json load may use HTTP. Names and semantics match
// agentdex.Fetch; this package stays a leaf.
type Fetch int

const (
	// FetchCached is the default: use the cache if present and fetch only on a
	// cold miss. No freshness check and no TTL refetch.
	FetchCached Fetch = iota
	// FetchCacheOnly is the on-disk cache only. No HTTP. A miss is an error.
	FetchCacheOnly
	// FetchTTL: within TTL use the file cache; after TTL refetch; a failed
	// refetch may serve the stale file.
	FetchTTL
	// FetchLatest requires HTTP. Failure does not silently serve stale.
	// WithForceRefresh is this mode.
	FetchLatest
)

// String returns the constant name for known modes, or Fetch(n) for others.
func (s Fetch) String() string {
	switch s {
	case FetchTTL:
		return "FetchTTL"
	case FetchCacheOnly:
		return "FetchCacheOnly"
	case FetchCached:
		return "FetchCached"
	case FetchLatest:
		return "FetchLatest"
	default:
		return fmt.Sprintf("Fetch(%d)", int(s))
	}
}
