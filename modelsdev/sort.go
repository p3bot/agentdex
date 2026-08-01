package modelsdev

import "sort"

// Newer reports whether a sorts before b in a newest-first listing: later
// release date first (ISO dates compare lexically), undated last, ties broken
// by id. Shared presentation order; Client.Models remains sorted by id.
func Newer(a, b Model) bool {
	if a.ReleaseDate != b.ReleaseDate {
		if a.ReleaseDate == "" {
			return false
		}
		if b.ReleaseDate == "" {
			return true
		}
		return a.ReleaseDate > b.ReleaseDate
	}
	return a.ID < b.ID
}

// SortByRelease orders models newest release first, in place, via Newer.
func SortByRelease(models []Model) {
	sort.SliceStable(models, func(i, j int) bool { return Newer(models[i], models[j]) })
}
