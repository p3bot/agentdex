package agentdex

import (
	"fmt"

	"github.com/p3bot/agentdex/internal/catalog"
	"github.com/p3bot/agentdex/modelsdev"
)

// Fetch is the Open-time policy for when the agent catalog or models.dev may
// use the network. It is orthogonal to Enrich (what data). Default is FetchCached.
// Catalog and models.dev each take their own Fetch via WithCatalogFetch and
// WithModelsFetch. Not a field on Get/List queries.
type Fetch int

const (
	// FetchCached is the default: use the cache if present and network only on
	// a cold miss. No freshness check and no TTL re-resolve.
	FetchCached Fetch = iota
	// FetchCacheOnly is last disk cache only. Catalog: no ModuleVersions; the
	// pinned module@version is still fetched (CUE content cache, network if that
	// cache is cold). models.dev: no HTTP. A miss is ErrCatalogUnavailable /
	// ErrModelsUnavailable.
	FetchCacheOnly
	// FetchTTL: within TTL use cache; after TTL re-resolve or refetch. A failed
	// re-resolve may serve a stale catalog or degrade models.dev.
	FetchTTL
	// FetchLatest requires the network. Failure does not silently serve stale.
	// Same honesty as Index.Refresh.
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

func (s Fetch) asCatalog() (catalog.Fetch, bool) {
	switch s {
	case FetchTTL:
		return catalog.FetchTTL, true
	case FetchCacheOnly:
		return catalog.FetchCacheOnly, true
	case FetchCached:
		return catalog.FetchCached, true
	case FetchLatest:
		return catalog.FetchLatest, true
	default:
		return 0, false
	}
}

func (s Fetch) asModels() (modelsdev.Fetch, bool) {
	switch s {
	case FetchTTL:
		return modelsdev.FetchTTL, true
	case FetchCacheOnly:
		return modelsdev.FetchCacheOnly, true
	case FetchCached:
		return modelsdev.FetchCached, true
	case FetchLatest:
		return modelsdev.FetchLatest, true
	default:
		return 0, false
	}
}
