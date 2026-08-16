package cli

import (
	"strings"
	"testing"
)

func TestListHelpSurface(t *testing.T) {
	root := NewRootCommand()
	tests := []struct {
		name    string
		path    []string
		longHas []string
		longNot []string
		orderBy string
		set     fieldSet
		flagHas map[string][]string
	}{
		{
			name: "agents",
			path: []string{"agents", "list"},
			set:  agentFieldSet,
			longHas: []string{
				"BIN column",
				"case-insensitive",
				"empty listing",
				"exits 0",
			},
			orderBy: "detected agents first",
			flagHas: map[string][]string{
				"order-by": {`drops that grouping`},
				"provider": {`show "-" for providers and models`},
			},
		},
		{
			name: "providers",
			path: []string{"providers", "list"},
			longHas: []string{
				"The set column shows set",
				"--fields present",
				"case-insensitive",
				"empty listing",
				"exits 0",
			},
			longNot: []string{"API-key environment variables and whether they are set"},
			orderBy: "default: id",
			set:     providerFieldSet,
		},
		{
			name: "models",
			path: []string{"models", "list"},
			longHas: []string{
				"case-insensitive",
				"empty listing",
				"exits 0",
			},
			longNot: []string{"scope"},
			orderBy: "default: released, newest first",
			set:     modelFieldSet,
			flagHas: map[string][]string{
				"agent":    {"provider-agnostic agent requires --provider", "home-provider agent accepts --provider only as a subset"},
				"provider": {"provider-agnostic --agent", "home-provider --agent"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, _, err := root.Find(tt.path)
			if err != nil {
				t.Fatalf("find %s: %v", strings.Join(tt.path, " "), err)
			}
			for _, want := range tt.longHas {
				if !strings.Contains(cmd.Long, want) {
					t.Errorf("Long missing %q:\n%s", want, cmd.Long)
				}
			}
			for _, omit := range tt.longNot {
				if strings.Contains(cmd.Long, omit) {
					t.Errorf("Long must not carry %q:\n%s", omit, cmd.Long)
				}
			}
			for _, flag := range []string{"--order-by", "--reverse"} {
				if strings.Contains(cmd.Long, flag) {
					t.Errorf("Long must not carry %s (row order lives on the flag):\n%s", flag, cmd.Long)
				}
			}
			order := cmd.Flags().Lookup("order-by")
			if order == nil {
				t.Fatal("missing --order-by")
			}
			if !strings.Contains(order.Usage, tt.orderBy) {
				t.Errorf("--order-by usage missing %q: %s", tt.orderBy, order.Usage)
			}
			if tt.set.defaultKey == "" {
				t.Fatal("test case missing field set")
			}
			if !strings.Contains(order.Usage, tt.set.defaultKey) {
				t.Errorf("--order-by usage missing default key %q: %s", tt.set.defaultKey, order.Usage)
			}
			for name, phrases := range tt.flagHas {
				f := cmd.Flags().Lookup(name)
				if f == nil {
					t.Errorf("missing --%s", name)
					continue
				}
				for _, want := range phrases {
					if !strings.Contains(f.Usage, want) {
						t.Errorf("--%s usage missing %q: %s", name, want, f.Usage)
					}
				}
			}
		})
	}
}
