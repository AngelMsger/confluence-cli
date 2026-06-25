// Package output renders command results for either agents or humans. JSON is
// the default (machine-readable); table is human-friendly; ndjson streams one
// record per line for large result sets.
package output

import (
	"encoding/json"
	"fmt"
	"io"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/neilotoole/jsoncolor"
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
	// Pretty enables ANSI-colored JSON when Writer is a TTY. It has no effect
	// on table output and is silently downgraded to plain JSON when Writer is
	// not a terminal (so e.g. `--pretty | jq` continues to work).
	Pretty bool
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
		generic = project(generic, opt.Fields)
	}

	switch opt.Format {
	case FormatTable:
		return emitTable(generic, opt)
	case FormatNDJSON:
		return emitNDJSON(generic, opt.Writer, opt.Pretty)
	case FormatJSON, "":
		return emitJSON(generic, opt.Writer, opt.Pretty)
	default:
		return badFormat(opt.Format)
	}
}

// EmitList renders a paginated list result as a {items, next, has_more}
// envelope. Unlike Emit it is told explicitly that the value is a list, so the
// envelope shape never has to be guessed from the data. json emits the
// envelope; table renders the items as a grid with a cursor footer; ndjson
// streams the items, one per line.
func EmitList(items any, next string, hasMore bool, opt Options) error {
	if opt.Writer == nil {
		return cerrors.New(cerrors.CategoryInternal, "NO_WRITER", "no output writer configured")
	}
	generic, err := toGeneric(items)
	if err != nil {
		return err
	}
	list, _ := generic.([]any)
	if list == nil {
		list = []any{}
	}
	if len(opt.Fields) > 0 {
		if projected, ok := project(list, opt.Fields).([]any); ok {
			list = projected
		}
	}

	switch opt.Format {
	case FormatTable:
		if err := emitListTable(list, opt); err != nil {
			return err
		}
		if hasMore {
			_, err := fmt.Fprintf(opt.Writer, "\n(more results — re-run with --cursor %s)\n", next)
			return err
		}
		return nil
	case FormatNDJSON:
		return emitNDJSON(list, opt.Writer, opt.Pretty)
	case FormatJSON, "":
		env := map[string]any{"items": list, "has_more": hasMore}
		if next != "" {
			env["next"] = next
		}
		return emitJSON(env, opt.Writer, opt.Pretty)
	default:
		return badFormat(opt.Format)
	}
}

// EmitNotice writes a single compact JSON line carrying out-of-band notices
// (e.g. a newer-release notice, or input corrections) to w — typically stderr.
// It deliberately never touches stdout, so the command's data contract on
// stdout stays byte-identical; agents still see the notice via the shell.
// Failures are swallowed: a notice must never turn a successful command into
// a failure.
func EmitNotice(w io.Writer, notice any) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(notice)
}

func badFormat(format string) error {
	return cerrors.Newf(cerrors.CategoryUsage, "BAD_FORMAT",
		"unknown output format %q (want json, table or ndjson)", format)
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

func emitJSON(v any, w io.Writer, pretty bool) error {
	if pretty && jsoncolor.IsColorTerminal(w) {
		enc := jsoncolor.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		enc.SetColors(jsoncolor.DefaultColors())
		return enc.Encode(v)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func emitNDJSON(v any, w io.Writer, pretty bool) error {
	encode := func(item any) error {
		if pretty && jsoncolor.IsColorTerminal(w) {
			enc := jsoncolor.NewEncoder(w)
			enc.SetEscapeHTML(false)
			enc.SetColors(jsoncolor.DefaultColors())
			return enc.Encode(item)
		}
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		return enc.Encode(item)
	}
	if list, ok := v.([]any); ok {
		for _, item := range list {
			if err := encode(item); err != nil {
				return err
			}
		}
		return nil
	}
	return encode(v)
}

// EmitError writes a structured error as JSON to w (typically stderr). When
// the process was invoked with --pretty AND w is a TTY, the error JSON is
// also colorized; otherwise it falls back to plain JSON so log scrapers and
// non-TTY consumers see byte-identical output.
func EmitError(err error, w io.Writer) {
	emitErrorWith(err, w, prettyEnabled())
}

// emitErrorWith is the testable form of EmitError; production code uses
// EmitError which reads the package-level pretty switch.
func emitErrorWith(err error, w io.Writer, pretty bool) {
	ce := cerrors.AsCLIError(err)
	if pretty && jsoncolor.IsColorTerminal(w) {
		enc := jsoncolor.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		enc.SetColors(jsoncolor.DefaultColors())
		if encErr := enc.Encode(ce.Payload()); encErr == nil {
			return
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if encErr := enc.Encode(ce.Payload()); encErr != nil {
		fmt.Fprintf(w, "%s\n", ce.Error())
	}
}

// errorPretty is a process-global hint that --pretty was passed. EmitError is
// the only path that needs it (it's called from cobra's outermost handler
// where we no longer have an *appState in scope). Other emit paths receive
// Pretty via Options.
var errorPretty = false

// SetErrorPretty toggles the process-global pretty flag for EmitError. It is
// called by the root command after parsing --pretty so error output styling
// matches success output styling.
func SetErrorPretty(v bool) { errorPretty = v }

func prettyEnabled() bool { return errorPretty }
