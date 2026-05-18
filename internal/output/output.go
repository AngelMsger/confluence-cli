// Package output renders command results for either agents or humans. JSON is
// the default (machine-readable); table is human-friendly; ndjson streams one
// record per line for large result sets.
package output

import (
	"encoding/json"
	"fmt"
	"io"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
)

// Format identifies an output format.
const (
	FormatJSON   = "json"
	FormatTable  = "table"
	FormatNDJSON = "ndjson"
)

// Options configures rendering.
type Options struct {
	Format string
	// Fields, when non-empty, projects each record to these dot-path keys.
	Fields []string
	Writer io.Writer
}

// Emit renders v to the configured writer in the configured format.
func Emit(v any, opt Options) error {
	if opt.Writer == nil {
		return cerrors.New(cerrors.CategoryInternal, "NO_WRITER", "no output writer configured")
	}
	// Normalize through JSON so every result type is handled uniformly.
	generic, err := toGeneric(v)
	if err != nil {
		return err
	}
	if len(opt.Fields) > 0 {
		// For a list envelope, --fields projects the items, not the
		// {items,next,has_more} wrapper itself.
		if items, ok := listEnvelope(generic); ok {
			env := generic.(map[string]any)
			env["items"] = project(items, opt.Fields)
			generic = env
		} else {
			generic = project(generic, opt.Fields)
		}
	}

	switch opt.Format {
	case FormatTable:
		return emitTable(generic, opt)
	case FormatNDJSON:
		return emitNDJSON(generic, opt.Writer)
	case FormatJSON, "":
		return emitJSON(generic, opt.Writer)
	default:
		return cerrors.Newf(cerrors.CategoryUsage, "BAD_FORMAT",
			"unknown output format %q (want json, table or ndjson)", opt.Format)
	}
}

// toGeneric converts any value into map[string]any / []any / scalar form.
func toGeneric(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, cerrors.Wrap(err, cerrors.CategoryInternal, "ENCODE",
			"failed to encode result")
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, cerrors.Wrap(err, cerrors.CategoryInternal, "DECODE",
			"failed to normalize result")
	}
	return out, nil
}

func emitJSON(v any, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// listEnvelope reports whether v is a list envelope ({items,next,has_more})
// and, if so, returns its items slice.
func listEnvelope(v any) ([]any, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	if _, ok := m["has_more"]; !ok {
		return nil, false
	}
	items, ok := m["items"].([]any)
	if !ok {
		return nil, false
	}
	return items, true
}

func emitNDJSON(v any, w io.Writer) error {
	// ndjson streams the records themselves; unwrap a list envelope to its items.
	if items, ok := listEnvelope(v); ok {
		v = items
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if list, ok := v.([]any); ok {
		for _, item := range list {
			if err := enc.Encode(item); err != nil {
				return err
			}
		}
		return nil
	}
	return enc.Encode(v)
}

// EmitError writes a structured error as JSON to w (typically stderr).
func EmitError(err error, w io.Writer) {
	ce := cerrors.AsCLIError(err)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if encErr := enc.Encode(ce.Payload()); encErr != nil {
		fmt.Fprintf(w, "%s\n", ce.Error())
	}
}
