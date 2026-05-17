package render

import (
	"strings"
	"testing"
)

const sample = `
<h1>Introduction</h1>
<p>Welcome to the <strong>project</strong>.</p>
<h2>Setup</h2>
<p>Install the tool first.</p>
<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">bash</ac:parameter><ac:plain-text-body><![CDATA[make build]]></ac:plain-text-body></ac:structured-macro>
<h2>Usage</h2>
<p>Run the command with a flag.</p>
<ul><li>first item</li><li>second item</li></ul>
`

func TestRenderFull(t *testing.T) {
	t.Parallel()
	got, err := Render(sample, Options{Scope: ScopeFull})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Body, "# Introduction") {
		t.Errorf("missing h1 markdown:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "**project**") {
		t.Errorf("inline bold not rendered:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "```bash\nmake build\n```") {
		t.Errorf("code macro not rendered:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "- first item") {
		t.Errorf("list not rendered:\n%s", got.Body)
	}
}

func TestRenderOutline(t *testing.T) {
	t.Parallel()
	got, err := Render(sample, Options{Scope: ScopeOutline})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Outline) != 3 {
		t.Fatalf("outline entries = %d, want 3", len(got.Outline))
	}
	if got.Outline[0].SectionID != "sec-1" || got.Outline[0].Title != "Introduction" {
		t.Errorf("first outline entry = %+v", got.Outline[0])
	}
	if !got.Truncated {
		t.Error("outline scope should mark result truncated")
	}
	if !strings.Contains(got.Body, "Setup [sec-2]") {
		t.Errorf("outline body missing entry:\n%s", got.Body)
	}
}

func TestRenderSection(t *testing.T) {
	t.Parallel()
	got, err := Render(sample, Options{Scope: ScopeSection, SectionID: "sec-2"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Body, "Setup") || !strings.Contains(got.Body, "make build") {
		t.Errorf("section body wrong:\n%s", got.Body)
	}
	if strings.Contains(got.Body, "Usage") {
		t.Errorf("section should stop before next h2:\n%s", got.Body)
	}
	if !got.Truncated {
		t.Error("section scope should mark result truncated")
	}
}

func TestRenderSectionNotFound(t *testing.T) {
	t.Parallel()
	if _, err := Render(sample, Options{Scope: ScopeSection, SectionID: "sec-99"}); err == nil {
		t.Error("expected error for unknown section")
	}
}

func TestRenderSectionRequiresID(t *testing.T) {
	t.Parallel()
	if _, err := Render(sample, Options{Scope: ScopeSection}); err == nil {
		t.Error("expected error when --section omitted")
	}
}

func TestRenderKeyword(t *testing.T) {
	t.Parallel()
	got, err := Render(sample, Options{Scope: ScopeKeyword, Keyword: "command"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Body, "Run the command") {
		t.Errorf("keyword hit missing:\n%s", got.Body)
	}
	// The nearest preceding heading provides context.
	if !strings.Contains(got.Body, "Usage") {
		t.Errorf("keyword scope should include context heading:\n%s", got.Body)
	}
	if strings.Contains(got.Body, "Welcome to the") {
		t.Errorf("keyword scope should exclude non-matching blocks:\n%s", got.Body)
	}
}

func TestRenderDetailWithIDs(t *testing.T) {
	t.Parallel()
	got, err := Render(sample, Options{Scope: ScopeFull, Detail: DetailWithIDs})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Body, "# Introduction [sec-1]") {
		t.Errorf("with-ids detail should annotate headings:\n%s", got.Body)
	}
}

func TestRenderAsText(t *testing.T) {
	t.Parallel()
	got, err := Render(sample, Options{Scope: ScopeFull, As: AsText})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Body, "# Introduction") {
		t.Errorf("text output should not contain markdown headings:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "INTRODUCTION") {
		t.Errorf("text heading should be upper-cased:\n%s", got.Body)
	}
}

func TestRenderTableAndQuote(t *testing.T) {
	t.Parallel()
	doc := `<blockquote>note this</blockquote>
		<table><tbody><tr><th>A</th><th>B</th></tr><tr><td>1</td><td>2</td></tr></tbody></table>`
	got, err := Render(doc, Options{Scope: ScopeFull})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Body, "> note this") {
		t.Errorf("blockquote not rendered:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "| A | B |") || !strings.Contains(got.Body, "| 1 | 2 |") {
		t.Errorf("table not rendered:\n%s", got.Body)
	}
}

func TestRenderEmpty(t *testing.T) {
	t.Parallel()
	got, err := Render("", Options{Scope: ScopeFull})
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "" || len(got.Outline) != 0 {
		t.Errorf("empty input should yield empty render: %+v", got)
	}
}

func TestRenderLink(t *testing.T) {
	t.Parallel()
	got, err := Render(`<p>see <a href="https://x.test">here</a></p>`, Options{Scope: ScopeFull})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Body, "[here](https://x.test)") {
		t.Errorf("link not rendered:\n%s", got.Body)
	}
}
