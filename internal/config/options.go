package config

import (
	"maps"

	"github.com/start-cli/agentdex"
)

// Flags carries the global flag values that feed into option mapping.
// SearchDirs is the repeated --search-dir values; BinPaths the parsed id=path
// map from --bin-path. Both merge with config.cue counterparts; flags win on collision.
type Flags struct {
	SearchDirs []string
	BinPaths   map[string]string
}

// Options builds the agentdex.Open options from the resolved configuration and
// the global flags. When catalog.dir is set it is the sole catalog source
// (module omitted); otherwise catalog.module is used — the two are mutually
// exclusive in the library. Force-refresh is owned by Index.Refresh.
func (c *Config) Options(f Flags) []agentdex.Option {
	opts := []agentdex.Option{
		agentdex.WithCatalogTTL(c.CatalogTTL),
		agentdex.WithModelsTTL(c.ModelsTTL),
	}
	if c.CatalogDir != "" {
		opts = append(opts, agentdex.WithCatalogDir(c.CatalogDir))
	} else if c.CatalogModule != "" {
		opts = append(opts, agentdex.WithCatalogModule(c.CatalogModule))
	}
	if c.ModelsURL != "" {
		opts = append(opts, agentdex.WithModelsURL(c.ModelsURL))
	}
	if dirs := mergeSlices(c.SearchDirs, f.SearchDirs); len(dirs) > 0 {
		opts = append(opts, agentdex.WithSearchDirs(dirs...))
	}
	if bin := mergeBinPaths(c.BinPaths, f.BinPaths); len(bin) > 0 {
		opts = append(opts, agentdex.WithBinPaths(bin))
	}
	return opts
}

// mergeSlices concatenates config then flags, dropping exact duplicates so a
// value given in both places is not searched twice.
func mergeSlices(cfg, flags []string) []string {
	out := make([]string, 0, len(cfg)+len(flags))
	seen := make(map[string]struct{}, len(cfg)+len(flags))
	for _, group := range [][]string{cfg, flags} {
		for _, v := range group {
			if _, dup := seen[v]; dup {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

// mergeBinPaths overlays flag overrides onto the config map so command-line
// paths win on id collision.
func mergeBinPaths(cfg, flags map[string]string) map[string]string {
	if len(cfg) == 0 && len(flags) == 0 {
		return nil
	}
	out := make(map[string]string, len(cfg)+len(flags))
	maps.Copy(out, cfg)
	maps.Copy(out, flags)
	return out
}
