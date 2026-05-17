package output

import "strings"

// project filters a generic value down to the requested dot-path fields.
// A list projects each element; a map projects itself; scalars pass through.
func project(v any, fields []string) any {
	switch t := v.(type) {
	case []any:
		out := make([]any, 0, len(t))
		for _, item := range t {
			out = append(out, project(item, fields))
		}
		return out
	case map[string]any:
		return projectMap(t, fields)
	default:
		return v
	}
}

// projectMap builds a new map containing only the requested dot paths. The
// output key is the last path segment; nested paths are flattened.
func projectMap(m map[string]any, fields []string) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		val, ok := lookup(m, strings.Split(f, "."))
		if !ok {
			continue
		}
		out[f] = val
	}
	return out
}

// lookup walks a dot path through nested maps.
func lookup(v any, path []string) (any, bool) {
	cur := v
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, exists := m[key]
		if !exists {
			return nil, false
		}
		cur = next
	}
	return cur, true
}
