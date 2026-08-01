package cli

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/start-cli/agentdex/modelsdev"
)

func (fs fieldSet) ordered(defaultKey string, descend ...string) fieldSet {
	fs.defaultKey = defaultKey
	fs.descend = make(map[string]bool, len(descend))
	for _, k := range descend {
		fs.descend[k] = true
	}
	return fs
}

// Sorting compares typed JSON values, not formatted text, so numbers and dates order correctly.
func (r *record) value(key string) any {
	if f, ok := r.present[key]; ok {
		return f.val
	}
	return nil
}

// Unknown keys fail at apply time so help cannot drift from acceptance.
func registerOrderFlags(cmd *cobra.Command, orderBy *string, reverse *bool) {
	cmd.Flags().StringVar(orderBy, "order-by", "", "Sort rows by this field")
	cmd.Flags().BoolVar(reverse, "reverse", false, "Reverse the sort direction")
}

// Returns the effective key so the caller can pull its column leftmost.
// --reverse flips the field's natural direction.
func applyOrder(recs []*record, set fieldSet, orderBy string, reverse bool) (string, error) {
	key := orderBy
	if key == "" {
		key = set.defaultKey
	}
	if !set.index[key] {
		return "", fmt.Errorf("unknown field %q (valid: %s)", key, strings.Join(set.all, ", "))
	}
	// XOR: --reverse flips the field's natural direction.
	descending := set.descend[key] != reverse
	orderRecords(recs, key, descending)
	return key, nil
}

// Missing values sink last regardless of direction (undated models, unknown prices).
func orderRecords(recs []*record, key string, descending bool) {
	sort.SliceStable(recs, func(i, j int) bool {
		if less, tie := lessByKey(recs[i], recs[j], key, descending); !tie {
			return less
		}
		return recordLess(recs[i], recs[j])
	})
}

const (
	orderMissing = iota // absent or empty: always sorts last
	orderNum
	orderStr
)

// Missing always last; present values compare typed and invert when descending.
func lessByKey(a, b *record, key string, descending bool) (less, tie bool) {
	ak, an, as := orderKey(a.value(key))
	bk, bn, bs := orderKey(b.value(key))
	if ak == orderMissing || bk == orderMissing {
		if ak == bk {
			return false, true
		}
		return bk == orderMissing, false // non-missing sorts first
	}
	switch {
	case ak != bk:
		less = ak == orderNum // numbers before strings; mixed kinds should not occur
	case ak == orderNum:
		if an == bn {
			return false, true
		}
		less = an < bn
	default:
		if as == bs {
			return false, true
		}
		less = as < bs
	}
	if descending {
		less = !less
	}
	return less, false
}

// Nil or empty string is missing. Slice/map values order by length so --order-by models ranks by count.
func orderKey(v any) (kind int, num float64, str string) {
	switch t := v.(type) {
	case nil:
		return orderMissing, 0, ""
	case string:
		if t == "" {
			return orderMissing, 0, ""
		}
		return orderStr, 0, t
	case bool:
		if t {
			return orderNum, 1, ""
		}
		return orderNum, 0, ""
	case int:
		return orderNum, float64(t), ""
	case int64:
		return orderNum, float64(t), ""
	case float64:
		return orderNum, t, ""
	case []string:
		return orderNum, float64(len(t)), ""
	case []modelsdev.Model:
		return orderNum, float64(len(t)), ""
	case map[string]bool:
		return orderNum, float64(len(t)), ""
	default:
		return orderMissing, 0, ""
	}
}

func recordLess(a, b *record) bool {
	return recordID(a) < recordID(b)
}

func recordID(r *record) string {
	if id, ok := r.value("id").(string); ok {
		return id
	}
	return ""
}

// Without mutating the shared default slice.
func insertAfter(cols []string, anchor, add string) []string {
	if slices.Contains(cols, add) {
		return cols
	}
	out := make([]string, 0, len(cols)+1)
	for _, c := range cols {
		out = append(out, c)
		if c == anchor {
			out = append(out, add)
		}
	}
	return out
}

// Sort column leftmost so ordering is legible. Default/verbose only; never --fields.
func orderColumns(cols []string, sortKey string) []string {
	out := make([]string, 0, len(cols)+1)
	out = append(out, sortKey)
	for _, c := range cols {
		if c != sortKey {
			out = append(out, c)
		}
	}
	return out
}
