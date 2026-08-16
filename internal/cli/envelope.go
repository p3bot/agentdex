package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/p3bot/agentdex/internal/tui"
)

// envelope is the shared JSON output contract for every command.
type envelope struct {
	Status   string   `json:"status"`          // "ok" or "error"
	Data     any      `json:"data,omitempty"`  // command payload on success
	Error    string   `json:"error,omitempty"` // message on failure
	Warnings []string `json:"warnings"`        // always an array; never omitted or null
}

// field is one selectable output field: JSON value and text rendering kept together
// so --fields selects the same field for both surfaces.
type field struct {
	key  string
	val  any
	text string
}

// fieldSet is the declared field authority for a record type. Validity and order
// come from the declaration, never from which fields a record instance carries.
// defaultKey is the sort field when --order-by is absent; descend keys sort
// newest/most first and --reverse flips that.
type fieldSet struct {
	all        []string
	defaults   []string
	index      map[string]bool
	defaultKey string
	descend    map[string]bool
}

func newFieldSet(all, defaults []string) fieldSet {
	index := make(map[string]bool, len(all))
	for _, k := range all {
		index[k] = true
	}
	return fieldSet{all: all, defaults: defaults, index: index}
}

func (fs fieldSet) validate(fields []string) error {
	for _, k := range fields {
		if !fs.index[k] {
			return fmt.Errorf("unknown field %q (valid: %s)", k, strings.Join(fs.all, ", "))
		}
	}
	return nil
}

// record is one output row as an ordered, selectable field set. A valid but absent
// key (empty canonical id, say) resolves to empty JSON and text "-" rather than
// an unknown-field error.
type record struct {
	set     fieldSet
	order   []string
	present map[string]field
}

func newRecord(set fieldSet) *record {
	return &record{set: set, present: map[string]field{}}
}

func (r *record) add(key string, val any, text string) {
	if _, dup := r.present[key]; !dup {
		r.order = append(r.order, key)
	}
	r.present[key] = field{key: key, val: val, text: text}
}

// Empty selection is every field this record carries; selected-but-absent resolve
// to empty JSON val and text "-" so table cells never look accidentally blank.
func (r *record) resolve(fields []string) ([]field, error) {
	if err := r.set.validate(fields); err != nil {
		return nil, err
	}
	keys := fields
	if len(keys) == 0 {
		keys = r.order
	}
	out := make([]field, 0, len(keys))
	for _, k := range keys {
		if f, ok := r.present[k]; ok {
			out = append(out, f)
			continue
		}
		out = append(out, field{key: k, val: "", text: "-"})
	}
	return out, nil
}

func jsonObject(fields []field) map[string]any {
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		m[f.key] = f.val
	}
	return m
}

func writeJSON(w io.Writer, env envelope) {
	if env.Warnings == nil {
		env.Warnings = []string{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(env)
}

func emitWarnings(w io.Writer, warnings []string) {
	for _, msg := range warnings {
		fmt.Fprintln(w, tui.Warn.Sprint("warning:")+" "+msg)
	}
}
