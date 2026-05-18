package render

import (
	"strings"
	"testing"
)

func TestMarkdownToStorageElements(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		md   string
		want string
	}{
		{"heading", "# Title", "<h1>Title</h1>"},
		{"heading level 6", "###### Deep", "<h6>Deep</h6>"},
		{"paragraph", "hello world", "<p>hello world</p>"},
		{"bold", "**strong**", "<strong>strong</strong>"},
		{"italic", "*soft*", "<em>soft</em>"},
		{"inline code", "use `x`", "<code>x</code>"},
		{"link", "[text](http://e.com)", `<a href="http://e.com">text</a>`},
		{"blockquote", "> quoted", "<blockquote><p>quoted</p></blockquote>"},
		{"bullet list", "- one\n- two", "<ul><li>one</li><li>two</li></ul>"},
		{"ordered list", "1. one\n2. two", "<ol><li>one</li><li>two</li></ol>"},
		{"thematic break", "a\n\n---\n\nb", "<hr/>"},
		{"image", "![alt](http://e.com/p.png)", `<ri:url ri:value="http://e.com/p.png"/>`},
		{"strikethrough", "~~gone~~", "text-decoration: line-through"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := MarkdownToStorage(tc.md, nil)
			if !strings.Contains(got, tc.want) {
				t.Errorf("MarkdownToStorage(%q) = %q, want substring %q", tc.md, got, tc.want)
			}
		})
	}
}

func TestMarkdownToStorageFencedCode(t *testing.T) {
	t.Parallel()
	got := MarkdownToStorage("```go\nfmt.Println(\"hi\")\n```", nil)
	for _, want := range []string{
		`<ac:structured-macro ac:name="code">`,
		`<ac:parameter ac:name="language">go</ac:parameter>`,
		`<![CDATA[fmt.Println("hi")`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("fenced code output = %q, want substring %q", got, want)
		}
	}
}

func TestMarkdownToStorageTable(t *testing.T) {
	t.Parallel()
	md := "| A | B |\n|---|---|\n| 1 | 2 |"
	got := MarkdownToStorage(md, nil)
	for _, want := range []string{"<table><tbody>", "<th>A</th>", "<td>1</td>", "</tbody></table>"} {
		if !strings.Contains(got, want) {
			t.Errorf("table output = %q, want substring %q", got, want)
		}
	}
}

func TestMarkdownToStorageEscaping(t *testing.T) {
	t.Parallel()
	got := MarkdownToStorage("a < b & c > d", nil)
	if strings.Contains(got, "< b") || !strings.Contains(got, "&lt;") || !strings.Contains(got, "&amp;") {
		t.Errorf("unescaped output: %q", got)
	}
}

func TestMarkdownToStorageCDATAEscape(t *testing.T) {
	t.Parallel()
	got := MarkdownToStorage("```\nclosing ]]> here\n```", nil)
	if strings.Contains(got, "]]> here") {
		t.Errorf("raw CDATA terminator leaked: %q", got)
	}
	if !strings.Contains(got, "]]]]><![CDATA[>") {
		t.Errorf("CDATA terminator not split: %q", got)
	}
}

func TestMarkdownToStorageDegradesWithNote(t *testing.T) {
	t.Parallel()
	var warn strings.Builder
	got := MarkdownToStorage("- [x] done\n- [ ] todo", &warn)
	if !strings.Contains(warn.String(), "not supported") {
		t.Errorf("expected a degradation note, got %q", warn.String())
	}
	if !strings.Contains(got, "[x]") {
		t.Errorf("task marker not rendered as text: %q", got)
	}
}
