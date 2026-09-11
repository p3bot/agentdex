package catalog

import "fmt"

// Fetch is when version resolution may use the network. Names and semantics
// match agentdex.Fetch and modelsdev.Fetch; the loader stays separate.
type Fetch int

const (
	// FetchCached is the default: use a cached version if present and resolve
	// only on a cold miss. No freshness check and no TTL re-resolve.
	FetchCached Fetch = iota
	// FetchCacheOnly is the last resolved version only. No ModuleVersions.
	// Fetch of the pin still uses CUE's content cache (network if that cache is
	// cold). A miss is ErrUnavailable.
	FetchCacheOnly
	// FetchTTL: within TTL use the resolution cache; after TTL re-resolve.
	// A failed re-resolve may serve the last version as stale.
	FetchTTL
	// FetchLatest always resolves. Failure does not serve stale.
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
