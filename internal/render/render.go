package render

import (
	"strconv"
	"strings"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
)

// Scope values control how much of a page body is returned.
const (
	ScopeFull    = "full"
	ScopeOutline = "outline"
	ScopeSection = "section"
	ScopeKeyword = "keyword"
)

// Detail values control per-block verbosity.
const (
	DetailSimple  = "simple"
	DetailWithIDs = "with-ids"
	DetailFull    = "full"
)

// As values control the output syntax.
const (
	AsMarkdown = "markdown"
	AsText     = "text"
)

// Options configures Render.
type Options struct {
	Scope     string // full | outline | section | keyword
	Detail    string // simple | with-ids | full
	As        string // markdown | text
	SectionID string // required when Scope == section
	Keyword   string // required when Scope == keyword
}

// withDefaults fills empty fields with sensible defaults.
func (o Options) withDefaults() Options {
	if o.Scope == "" {
		o.Scope = ScopeFull
	}
	if o.Detail == "" {
		o.Detail = DetailSimple
	}
	if o.As == "" {
		o.As = AsMarkdown
	}
	return o
}

// OutlineEntry is a single heading in a page's outline.
type OutlineEntry struct {
	SectionID string `json:"section_id"`
	Level     int    `json:"level"`
	Title     string `json:"title"`
}

// Rendered is the result of rendering a page body.
type Rendered struct {
	Outline      []OutlineEntry `json:"outline,omitempty"`
	Body         string         `json:"body"`
	ScopeApplied string         `json:"scope_applied"`
	Truncated    bool           `json:"truncated"`
	// Notes lists content the renderer could not represent (macros without a
	// native rendering, images shown as placeholders). It is empty when the
	// markdown/text output is a faithful representation of the source.
	Notes []string `json:"notes,omitempty"`
}

// Render parses storage-format XHTML and renders it according to opt.
func Render(storage string, opt Options) (Rendered, error) {
	opt = opt.withDefaults()
	blocks, notes := parse(storage)
	assignSections(blocks)
	outline := buildOutline(blocks)

	result := Rendered{Outline: outline, ScopeApplied: opt.Scope, Notes: notes}

	switch opt.Scope {
	case ScopeFull:
		result.Body = renderBlocks(blocks, opt)
	case ScopeOutline:
		result.Body = renderOutline(outline)
		result.Truncated = len(blocks) > len(outline)
	case ScopeSection:
		if opt.SectionID == "" {
			return Rendered{}, cerrors.New(cerrors.CategoryUsage, "SCOPE_NO_SECTION",
				"--scope section requires --section <id>").
				WithHint("Run with --scope outline first to list section IDs.")
		}
		sec := sectionBlocks(blocks, opt.SectionID)
		if sec == nil {
			return Rendered{}, cerrors.Newf(cerrors.CategoryNotFound, "SECTION_NOT_FOUND",
				"section %q not found", opt.SectionID).
				WithHint("Run with --scope outline to list valid section IDs.")
		}
		result.Body = renderBlocks(sec, opt)
		result.Truncated = len(sec) < len(blocks)
	case ScopeKeyword:
		if opt.Keyword == "" {
			return Rendered{}, cerrors.New(cerrors.CategoryUsage, "SCOPE_NO_KEYWORD",
				"--scope keyword requires --keyword <text>")
		}
		hits := keywordBlocks(blocks, opt.Keyword)
		result.Body = renderBlocks(hits, opt)
		result.Truncated = len(hits) < len(blocks)
	default:
		return Rendered{}, cerrors.Newf(cerrors.CategoryUsage, "SCOPE_BAD",
			"unknown scope %q", opt.Scope)
	}
	return result, nil
}

// assignSections gives every heading a stable sequential ID (sec-1, sec-2, ...).
func assignSections(blocks []Block) {
	n := 0
	for i := range blocks {
		if blocks[i].Kind == KindHeading {
			n++
			blocks[i].SectionID = "sec-" + strconv.Itoa(n)
		}
	}
}

func buildOutline(blocks []Block) []OutlineEntry {
	var out []OutlineEntry
	for _, b := range blocks {
		if b.Kind == KindHeading {
			out = append(out, OutlineEntry{
				SectionID: b.SectionID, Level: b.Level, Title: b.Text,
			})
		}
	}
	return out
}

// sectionBlocks returns the heading with id plus every block until the next
// heading at the same or higher level. Returns nil when id is not found.
func sectionBlocks(blocks []Block, id string) []Block {
	start := -1
	var level int
	for i, b := range blocks {
		if b.Kind == KindHeading && b.SectionID == id {
			start, level = i, b.Level
			break
		}
	}
	if start < 0 {
		return nil
	}
	out := []Block{blocks[start]}
	for i := start + 1; i < len(blocks); i++ {
		if blocks[i].Kind == KindHeading && blocks[i].Level <= level {
			break
		}
		out = append(out, blocks[i])
	}
	return out
}

// keywordBlocks returns blocks containing kw, each preceded by its nearest
// heading for context.
func keywordBlocks(blocks []Block, kw string) []Block {
	lkw := strings.ToLower(kw)
	included := make([]bool, len(blocks))
	lastHeading := -1
	for i, b := range blocks {
		if b.Kind == KindHeading {
			lastHeading = i
		}
		if strings.Contains(strings.ToLower(b.Text), lkw) {
			if lastHeading >= 0 {
				included[lastHeading] = true
			}
			included[i] = true
		}
	}
	var out []Block
	for i, in := range included {
		if in {
			out = append(out, blocks[i])
		}
	}
	return out
}
