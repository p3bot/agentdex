package agentdex

import (
	"sort"

	"github.com/p3bot/agentdex/modelsdev"
)

// Newest release first via modelsdev.Newer so library and CLI share one rule.
func sortModels(models []Model) {
	sort.SliceStable(models, func(i, j int) bool {
		return modelsdev.Newer(models[i].Model, models[j].Model)
	})
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// First-seen order so a repeated provider id cannot double candidates or probes.
func dedupeIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
