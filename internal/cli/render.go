package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/p3bot/agentdex/internal/tui"
)

// tabulate projects records onto JSON and a text table independently. Empty
// jsonFields means each record's full field listing (JSON is never truncated to
// table columns). tableCols is always explicit. jsonFields is validated up front
// so an unknown key fails whether the result set is empty or not.
func tabulate(recs []*record, jsonFields, tableCols []string, set fieldSet) (data []map[string]any, headers []string, rows [][]string, err error) {
	if err := set.validate(jsonFields); err != nil {
		return nil, nil, nil, err
	}
	if len(recs) == 0 {
		return []map[string]any{}, tableCols, nil, nil
	}

	first, err := recs[0].resolve(tableCols)
	if err != nil {
		return nil, nil, nil, err
	}
	headers = make([]string, len(first))
	for i, f := range first {
		headers[i] = f.key
	}

	data = make([]map[string]any, 0, len(recs))
	rows = make([][]string, 0, len(recs))
	for _, r := range recs {
		jf, err := r.resolve(jsonFields)
		if err != nil {
			return nil, nil, nil, err
		}
		data = append(data, jsonObject(jf))

		cells, err := r.resolve(tableCols)
		if err != nil {
			return nil, nil, nil, err
		}
		row := make([]string, len(cells))
		for i, f := range cells {
			row[i] = f.text
		}
		rows = append(rows, row)
	}
	return data, headers, rows, nil
}

func renderTable(w io.Writer, headers []string, rows [][]string, empty string) {
	if len(rows) == 0 {
		fmt.Fprintln(w, empty)
		return
	}
	t := tui.NewTable(upper(headers)...)
	for _, row := range rows {
		t.Append(row...)
	}
	t.Render(w)
}

const priceUnitNote = "Prices in USD per 1M tokens (models.dev)"

// Text only: unit note when columns include a price.
func renderPriceFooter(w io.Writer, cols []string) {
	for _, c := range cols {
		if c == "input" || c == "output" || c == "total" {
			fmt.Fprintln(w, tui.Muted.Sprint(priceUnitNote))
			return
		}
	}
}

// One field is a bare value (pipe-friendly); several print "key: value" lines.
func renderFields(w io.Writer, fs []field) {
	if len(fs) == 1 {
		fmt.Fprintln(w, fs[0].text)
		return
	}
	for _, f := range fs {
		fmt.Fprintf(w, "%s: %s\n", f.key, f.text)
	}
}

func renderDetail(w io.Writer, fs []field) {
	width := 0
	for _, f := range fs {
		if n := len(f.key); n > width {
			width = n
		}
	}
	for _, f := range fs {
		label := tui.Label.Sprint(padRight(f.key, width))
		fmt.Fprintf(w, "%s  %s\n", label, f.text)
	}
}

func upper(keys []string) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = strings.ToUpper(k)
	}
	return out
}

func padRight(s string, width int) string {
	if pad := width - len(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
