package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// tostorage.go converts Markdown into Confluence storage-format XHTML. It is
// the inverse of the storage->markdown rendering in render.go, used by the
// page write commands when --format markdown is given.
//
// Conversion is best-effort: constructs that have no clean storage-format
// equivalent are degraded to escaped plain text and a one-time note is written
// to the warn writer. Content never causes the conversion to fail.

// MarkdownToStorage converts Markdown into Confluence storage-format XHTML.
// Notes about degraded constructs are written to warn (pass nil to discard).
func MarkdownToStorage(md string, warn io.Writer) string {
	src := []byte(md)
	doc := goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser().
		Parse(text.NewReader(src))
	w := &storageWriter{src: src, warn: warn, noted: map[string]bool{}}
	var sb strings.Builder
	w.renderChildren(&sb, doc)
	return strings.TrimSpace(sb.String())
}

type storageWriter struct {
	src   []byte
	warn  io.Writer
	noted map[string]bool
}

// note reports an unsupported construct once per kind.
func (w *storageWriter) note(kind string) {
	if w.warn == nil || w.noted[kind] {
		return
	}
	w.noted[kind] = true
	fmt.Fprintf(w.warn, "note: markdown %s is not supported, rendered as plain text\n", kind)
}

func (w *storageWriter) renderChildren(sb *strings.Builder, n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		w.render(sb, c)
	}
}

func (w *storageWriter) render(sb *strings.Builder, n ast.Node) {
	if n.Type() == ast.TypeInline {
		w.renderInline(sb, n)
		return
	}
	w.renderBlock(sb, n)
}

func (w *storageWriter) renderBlock(sb *strings.Builder, n ast.Node) {
	switch node := n.(type) {
	case *ast.Heading:
		level := node.Level
		if level < 1 {
			level = 1
		} else if level > 6 {
			level = 6
		}
		tag := "h" + strconv.Itoa(level)
		sb.WriteString("<" + tag + ">")
		w.renderChildren(sb, node)
		sb.WriteString("</" + tag + ">")
	case *ast.Paragraph:
		sb.WriteString("<p>")
		w.renderChildren(sb, node)
		sb.WriteString("</p>")
	case *ast.TextBlock:
		w.renderChildren(sb, node)
	case *ast.Blockquote:
		sb.WriteString("<blockquote>")
		w.renderChildren(sb, node)
		sb.WriteString("</blockquote>")
	case *ast.FencedCodeBlock:
		w.writeCodeMacro(sb, string(node.Language(w.src)), w.linesText(node))
	case *ast.CodeBlock:
		w.writeCodeMacro(sb, "", w.linesText(node))
	case *ast.List:
		tag := "ul"
		if node.IsOrdered() {
			tag = "ol"
		}
		sb.WriteString("<" + tag + ">")
		w.renderChildren(sb, node)
		sb.WriteString("</" + tag + ">")
	case *ast.ListItem:
		sb.WriteString("<li>")
		w.renderChildren(sb, node)
		sb.WriteString("</li>")
	case *ast.ThematicBreak:
		sb.WriteString("<hr/>")
	case *ast.HTMLBlock:
		w.note("raw HTML")
		sb.WriteString("<p>")
		sb.WriteString(escapeXML(strings.TrimRight(w.linesText(node), "\n")))
		sb.WriteString("</p>")
	case *extast.Table:
		w.renderTable(sb, node)
	default:
		w.renderChildren(sb, n)
	}
}

func (w *storageWriter) renderInline(sb *strings.Builder, n ast.Node) {
	switch node := n.(type) {
	case *ast.Text:
		sb.WriteString(escapeXML(string(node.Segment.Value(w.src))))
		if node.HardLineBreak() {
			sb.WriteString("<br/>")
		} else if node.SoftLineBreak() {
			sb.WriteString("\n")
		}
	case *ast.String:
		sb.WriteString(escapeXML(string(node.Value)))
	case *ast.Emphasis:
		tag := "em"
		if node.Level == 2 {
			tag = "strong"
		}
		sb.WriteString("<" + tag + ">")
		w.renderChildren(sb, node)
		sb.WriteString("</" + tag + ">")
	case *ast.CodeSpan:
		sb.WriteString("<code>")
		sb.WriteString(escapeXML(w.textOf(node)))
		sb.WriteString("</code>")
	case *ast.Link:
		sb.WriteString(`<a href="` + escapeXML(string(node.Destination)) + `">`)
		w.renderChildren(sb, node)
		sb.WriteString("</a>")
	case *ast.AutoLink:
		u := string(node.URL(w.src))
		sb.WriteString(`<a href="` + escapeXML(u) + `">` + escapeXML(u) + "</a>")
	case *ast.Image:
		alt := w.textOf(node)
		sb.WriteString(`<ac:image`)
		if alt != "" {
			sb.WriteString(` ac:alt="` + escapeXML(alt) + `"`)
		}
		sb.WriteString(`><ri:url ri:value="` + escapeXML(string(node.Destination)) + `"/></ac:image>`)
	case *ast.RawHTML:
		w.note("raw HTML")
		for i := 0; i < node.Segments.Len(); i++ {
			seg := node.Segments.At(i)
			sb.WriteString(escapeXML(string(seg.Value(w.src))))
		}
	case *extast.Strikethrough:
		sb.WriteString(`<span style="text-decoration: line-through;">`)
		w.renderChildren(sb, node)
		sb.WriteString("</span>")
	case *extast.TaskCheckBox:
		w.note("task list")
		if node.IsChecked {
			sb.WriteString("[x] ")
		} else {
			sb.WriteString("[ ] ")
		}
	default:
		w.renderChildren(sb, n)
	}
}

func (w *storageWriter) renderTable(sb *strings.Builder, n ast.Node) {
	sb.WriteString("<table><tbody>")
	for row := n.FirstChild(); row != nil; row = row.NextSibling() {
		cellTag := "td"
		if _, ok := row.(*extast.TableHeader); ok {
			cellTag = "th"
		}
		sb.WriteString("<tr>")
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			sb.WriteString("<" + cellTag + ">")
			w.renderChildren(sb, cell)
			sb.WriteString("</" + cellTag + ">")
		}
		sb.WriteString("</tr>")
	}
	sb.WriteString("</tbody></table>")
}

// writeCodeMacro emits a Confluence code macro wrapping code in a CDATA block.
func (w *storageWriter) writeCodeMacro(sb *strings.Builder, lang, code string) {
	sb.WriteString(`<ac:structured-macro ac:name="code">`)
	if lang != "" {
		sb.WriteString(`<ac:parameter ac:name="language">` + escapeXML(lang) + `</ac:parameter>`)
	}
	// Escape any CDATA terminator inside the code so the block stays well-formed.
	safe := strings.ReplaceAll(code, "]]>", "]]]]><![CDATA[>")
	sb.WriteString(`<ac:plain-text-body><![CDATA[` + safe + `]]></ac:plain-text-body>`)
	sb.WriteString(`</ac:structured-macro>`)
}

// linesText concatenates the raw source lines of a block node.
func (w *storageWriter) linesText(n ast.Node) string {
	var b strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(w.src))
	}
	return b.String()
}

// textOf returns the concatenated raw text of a node's descendants.
func (w *storageWriter) textOf(n ast.Node) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch t := c.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(w.src))
		case *ast.String:
			b.Write(t.Value)
		default:
			b.WriteString(w.textOf(c))
		}
	}
	return b.String()
}

var xmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&#39;",
)

// escapeXML escapes a string for use as XHTML text or attribute content.
func escapeXML(s string) string { return xmlEscaper.Replace(s) }
