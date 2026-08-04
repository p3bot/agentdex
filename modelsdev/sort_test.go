package modelsdev_test

import (
	"testing"

	"github.com/p3bot/agentdex/modelsdev"
)

func TestSortByReleaseNewestFirst(t *testing.T) {
	models := []modelsdev.Model{
		{ID: "undated"},
		{ID: "old", ReleaseDate: "2024-01-15"},
		{ID: "new-b", ReleaseDate: "2025-06-01"},
		{ID: "new-a", ReleaseDate: "2025-06-01"},
	}
	modelsdev.SortByRelease(models)
	want := []string{"new-a", "new-b", "old", "undated"}
	for i, id := range want {
		if models[i].ID != id {
			t.Fatalf("order[%d] = %q, want %q (full: %v)", i, models[i].ID, id, models)
		}
	}
}
