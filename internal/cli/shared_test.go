package cli

import (
	"slices"
	"testing"
)

func TestCSVTokensTrimsAndDropsEmpties(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil", nil, []string{}},
		{"empty", []string{}, []string{}},
		{"already clean", []string{"id", "name"}, []string{"id", "name"}},
		{"spaces after comma", []string{"id", " name"}, []string{"id", "name"}},
		{"spaces both sides", []string{" id ", " name "}, []string{"id", "name"}},
		{"empty tokens", []string{"id", "", "name"}, []string{"id", "name"}},
		{"whitespace tokens", []string{"id", "  ", "name"}, []string{"id", "name"}},
		{"only empties", []string{"", "  "}, []string{}},
		{"keeps duplicates", []string{"id", " id"}, []string{"id", "id"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := csvTokens(tc.in)
			if !slices.Equal(got, tc.want) {
				t.Errorf("csvTokens(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFlattenProvidersDedupsAfterTrim(t *testing.T) {
	got := flattenProviders([]string{"anthropic", " openai", "anthropic", "", " openai"})
	want := []string{"anthropic", "openai"}
	if !slices.Equal(got, want) {
		t.Errorf("flattenProviders = %q, want %q", got, want)
	}
}

func TestFieldsValueTrimsOnSet(t *testing.T) {
	var fields []string
	v := newFieldsValue(&fields)
	if err := v.Set("id, name"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !slices.Equal(fields, []string{"id", "name"}) {
		t.Errorf("Set(\"id, name\") = %q, want [id name]", fields)
	}
	if err := v.Set("found,"); err != nil {
		t.Fatalf("second Set: %v", err)
	}
	if !slices.Equal(fields, []string{"id", "name", "found"}) {
		t.Errorf("repeat Set append = %q, want [id name found]", fields)
	}
}

func TestFieldsValueEmptyTokens(t *testing.T) {
	var fields []string
	v := newFieldsValue(&fields)
	if err := v.Set(","); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("Set(\",\") = %q, want empty", fields)
	}
	if got := v.String(); got != "" {
		t.Errorf("empty String() = %q, want \"\" so help omits a default", got)
	}
}
