// Command gen-docs renders the confluence-cli command tree into reference
// documentation under docs/cli/:
//
//   - index.html — a single styled page served by GitHub Pages;
//   - README.md  — a module-grouped table index for browsing on GitHub.
//
// Both are generated from the live cobra command tree (via app.NewRootCmd), so
// the reference can never drift from --help. Run it with `make docs`; CI
// regenerates and fails when the committed output is stale.
package main

import (
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/angelmsger/confluence-cli/internal/app"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// outDir is relative to the repository root, where `make docs` and CI run.
const outDir = "docs/cli"

// pagesURL is where the generated index.html is published; README.md links
// command rows to anchors there.
const pagesURL = "https://angelmsger.github.io/confluence-cli/cli/"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen-docs:", err)
		os.Exit(1)
	}
}

type flagInfo struct {
	Name    string
	Default string
	Usage   string
}

type command struct {
	Path    string
	Anchor  string
	Short   string
	Long    string
	Usage   string
	Example string
	Flags   []flagInfo
	IsGroup bool
}

type module struct {
	Name     string
	Commands []command
}

func run() error {
	root := app.NewRootCmd()
	mods := collect(root)

	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := writeHTML(root, mods); err != nil {
		return err
	}
	if err := writeReadme(root, mods); err != nil {
		return err
	}
	fmt.Printf("generated %s/index.html and %s/README.md\n", outDir, outDir)
	return nil
}

// collect groups the command tree into modules — one per top-level command.
func collect(root *cobra.Command) []module {
	var mods []module
	for _, top := range root.Commands() {
		if !documented(top) {
			continue
		}
		m := module{Name: top.Name()}
		var walk func(c *cobra.Command)
		walk = func(c *cobra.Command) {
			if !documented(c) {
				return
			}
			m.Commands = append(m.Commands, describe(c))
			for _, sub := range c.Commands() {
				walk(sub)
			}
		}
		walk(top)
		mods = append(mods, m)
	}
	return mods
}

// documented reports whether a command should appear in the reference.
func documented(c *cobra.Command) bool {
	if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
		return false
	}
	switch c.Name() {
	case "help", "completion": // cobra built-ins, not part of the product surface
		return false
	}
	return true
}

func describe(c *cobra.Command) command {
	cmd := command{
		Path:    c.CommandPath(),
		Anchor:  strings.ReplaceAll(c.CommandPath(), " ", "-"),
		Short:   c.Short,
		Long:    c.Long,
		Usage:   c.UseLine(),
		Example: c.Example,
		IsGroup: !c.Runnable(),
	}
	c.NonInheritedFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		cmd.Flags = append(cmd.Flags, flagInfo{
			Name:    flagName(f),
			Default: f.DefValue,
			Usage:   f.Usage,
		})
	})
	return cmd
}

func flagName(f *pflag.Flag) string {
	if f.Shorthand != "" {
		return fmt.Sprintf("--%s, -%s", f.Name, f.Shorthand)
	}
	return "--" + f.Name
}

func globalFlags(root *cobra.Command) []flagInfo {
	var out []flagInfo
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		out = append(out, flagInfo{Name: flagName(f), Default: f.DefValue, Usage: f.Usage})
	})
	return out
}

// --- HTML output ---

type htmlData struct {
	Intro       string
	GlobalFlags []flagInfo
	Modules     []module
}

func writeHTML(root *cobra.Command, mods []module) error {
	tpl := template.Must(template.New("cli").Funcs(template.FuncMap{
		"example": renderExample,
	}).Parse(htmlTemplate))

	intro := root.Long
	if intro == "" {
		intro = root.Short
	}
	data := htmlData{Intro: intro, GlobalFlags: globalFlags(root), Modules: mods}

	f, err := os.Create(filepath.Join(outDir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()
	return tpl.Execute(f, data)
}

// renderExample turns an Example block into HTML, dimming comment lines to
// match the landing page's code styling.
func renderExample(s string) template.HTML {
	var b strings.Builder
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			b.WriteString(`<span class="c">` + html.EscapeString(line) + `</span>`)
		} else {
			b.WriteString(html.EscapeString(line))
		}
	}
	return template.HTML(b.String())
}

// --- Markdown index ---

func writeReadme(root *cobra.Command, mods []module) error {
	var b strings.Builder
	b.WriteString("# confluence-cli command reference\n\n")
	b.WriteString("This index is generated from the CLI command tree — do not edit it by\n")
	b.WriteString("hand; run `make docs`. The full reference, with every flag and example,\n")
	fmt.Fprintf(&b, "is published at <%s>.\n\n", pagesURL)

	for _, m := range mods {
		fmt.Fprintf(&b, "## %s\n\n", m.Name)
		b.WriteString("| Command | Description |\n| --- | --- |\n")
		for _, c := range m.Commands {
			fmt.Fprintf(&b, "| [`%s`](%s#%s) | %s |\n", c.Path, pagesURL, c.Anchor, c.Short)
		}
		b.WriteString("\n")
	}
	return os.WriteFile(filepath.Join(outDir, "README.md"), []byte(b.String()), 0o644)
}

const htmlTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>confluence-cli — CLI reference</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500;600&family=IBM+Plex+Sans:wght@400;500;600;700&display=swap" rel="stylesheet">
<link rel="stylesheet" href="../style.css">
<link rel="canonical" href="https://angelmsger.github.io/confluence-cli/cli/">
</head>
<body>
<nav class="nav">
  <div class="nav-inner">
    <a class="brand" href="../">confluence-cli</a>
    <div class="nav-links">
      <a href="../">Home</a>
      <a class="is-cli" href="../#commands">Commands</a>
      <a href="https://github.com/angelmsger/confluence-cli">GitHub</a>
    </div>
  </div>
</nav>
<div class="layout">
  <aside class="side">
    {{range .Modules}}<div class="side-group">
      <div class="side-title">{{.Name}}</div>
      {{range .Commands}}<a href="#{{.Anchor}}">{{.Path}}</a>
      {{end}}</div>
    {{end}}
  </aside>
  <main class="cli-main">
    <span class="eyebrow">Reference</span>
    <h1>CLI reference</h1>
    <p class="lead">{{.Intro}}</p>
    <p class="lead">Generated from the command tree, so it always matches <code>--help</code>.</p>

    <section class="cmd">
      <h2>Global flags</h2>
      <p class="short">Persistent flags accepted by every command.</p>
      <table>
        <thead><tr><th>Flag</th><th>Default</th><th>Description</th></tr></thead>
        <tbody>
        {{range .GlobalFlags}}<tr><td><code>{{.Name}}</code></td><td>{{if .Default}}<code>{{.Default}}</code>{{end}}</td><td>{{.Usage}}</td></tr>
        {{end}}</tbody>
      </table>
    </section>

    {{range .Modules}}{{range .Commands}}
    <section class="cmd" id="{{.Anchor}}">
      <h2>{{.Path}}{{if .IsGroup}}<span class="group-tag">command group</span>{{end}}</h2>
      <p class="short">{{.Short}}</p>
      <pre>{{.Usage}}</pre>
      {{if .Long}}<p class="long">{{.Long}}</p>{{end}}
      {{if .Flags}}<h3>Options</h3>
      <table>
        <thead><tr><th>Flag</th><th>Default</th><th>Description</th></tr></thead>
        <tbody>
        {{range .Flags}}<tr><td><code>{{.Name}}</code></td><td>{{if .Default}}<code>{{.Default}}</code>{{end}}</td><td>{{.Usage}}</td></tr>
        {{end}}</tbody>
      </table>{{end}}
      {{if .Example}}<h3>Examples</h3><pre>{{example .Example}}</pre>{{end}}
    </section>
    {{end}}{{end}}
  </main>
</div>
<footer class="footer">
  <div class="wrap">
    <span class="brand-foot">confluence-cli — MIT License</span>
    <span><a href="https://github.com/angelmsger/confluence-cli">github.com/angelmsger/confluence-cli</a></span>
  </div>
</footer>
</body>
</html>
`
