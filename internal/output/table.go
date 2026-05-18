package output

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// emitTable renders a generic value as a human-readable text table.
func emitTable(v any, opt Options) error {
	w := opt.Writer
	if items, ok := listEnvelope(v); ok {
		if err := emitListTable(items, opt); err != nil {
			return err
		}
		if next, _ := v.(map[string]any)["next"].(string); next != "" {
			_, err := fmt.Fprintf(w, "\n(more results — re-run with --cursor %s)\n", next)
			return err
		}
		return nil
	}
	switch t := v.(type) {
	case []any:
		return emitListTable(t, opt)
	case map[string]any:
		return emitKVTable(t, opt)
	default:
		_, err := fmt.Fprintln(w, cell(v))
		return err
	}
}

// emitListTable renders a slice. Slices of objects become column tables;
// slices of scalars become a single column.
func emitListTable(list []any, opt Options) error {
	w := opt.Writer
	if len(list) == 0 {
		_, err := fmt.Fprintln(w, "(no results)")
		return err
	}
	cols := columnsFor(list, opt.Fields)
	if len(cols) == 0 {
		for _, item := range list {
			if _, err := fmt.Fprintln(w, cell(item)); err != nil {
				return err
			}
		}
		return nil
	}

	rows := make([][]string, 0, len(list))
	for _, item := range list {
		m, _ := item.(map[string]any)
		row := make([]string, len(cols))
		for i, c := range cols {
			if m != nil {
				if val, ok := lookup(m, strings.Split(c, ".")); ok {
					row[i] = cell(val)
				}
			}
		}
		rows = append(rows, row)
	}
	return writeGrid(w, cols, rows)
}

// emitKVTable renders a single object as a two-column key/value table.
func emitKVTable(m map[string]any, opt Options) error {
	keys := opt.Fields
	if len(keys) == 0 {
		keys = sortedKeys(m)
	}
	rows := make([][]string, 0, len(keys))
	for _, k := range keys {
		val, _ := lookup(m, strings.Split(k, "."))
		rows = append(rows, []string{k, cell(val)})
	}
	return writeGrid(opt.Writer, []string{"FIELD", "VALUE"}, rows)
}

// columnsFor decides table columns: explicit fields, else the sorted union of
// keys across all object rows. Returns nil when the list holds no objects.
func columnsFor(list []any, fields []string) []string {
	if len(fields) > 0 {
		return fields
	}
	seen := map[string]bool{}
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			for k := range m {
				seen[k] = true
			}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	cols := make([]string, 0, len(seen))
	for k := range seen {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	return cols
}

// writeGrid prints a header plus rows with aligned columns.
func writeGrid(w interface{ Write([]byte) (int, error) }, headers []string, rows [][]string) error {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, c := range row {
			if i < len(widths) && len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}
	var b strings.Builder
	writeRow(&b, headers, widths)
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = strings.Repeat("-", widths[i])
	}
	writeRow(&b, seps, widths)
	for _, row := range rows {
		writeRow(&b, row, widths)
	}
	_, err := w.Write([]byte(b.String()))
	return err
}

func writeRow(b *strings.Builder, cells []string, widths []int) {
	for i, w := range widths {
		var c string
		if i < len(cells) {
			c = cells[i]
		}
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(c)
		if i < len(widths)-1 {
			b.WriteString(strings.Repeat(" ", w-len(c)))
		}
	}
	b.WriteString("\n")
}

// cell formats a single value for table display.
func cell(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.ReplaceAll(t, "\n", " ")
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case bool:
		return fmt.Sprintf("%t", t)
	case map[string]any, []any:
		raw, _ := json.Marshal(t)
		return string(raw)
	default:
		return fmt.Sprint(t)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
