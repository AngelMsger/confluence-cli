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
<style>
  :root {
    --bg: #0d1117; --bg-soft: #161b22; --card: #1c2333; --border: #2a3350;
    --text: #e6e9f0; --muted: #9aa4bf; --accent: #4d7cff; --radius: 14px;
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  html { scroll-behavior: smooth; }
  body {
    background: var(--bg); color: var(--text);
    font: 16px/1.65 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    -webkit-font-smoothing: antialiased;
  }
  a { color: var(--accent); text-decoration: none; }
  a:hover { text-decoration: underline; }
  code, pre { font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace; }
  nav {
    position: sticky; top: 0; z-index: 10; background: rgba(13,17,23,0.85);
    backdrop-filter: blur(8px); border-bottom: 1px solid var(--border);
  }
  nav .row { display: flex; align-items: center; gap: 22px; height: 56px; max-width: 1180px; margin: 0 auto; padding: 0 24px; }
  nav .brand { font-weight: 700; color: var(--text); }
  nav .links { margin-left: auto; display: flex; gap: 20px; }
  nav .links a { color: var(--muted); font-size: 0.92rem; }
  nav .links a:hover { color: var(--text); text-decoration: none; }
  .layout {
    display: grid; grid-template-columns: 240px 1fr; gap: 40px;
    max-width: 1180px; margin: 0 auto; padding: 32px 24px 96px; align-items: start;
  }
  .side { position: sticky; top: 76px; max-height: calc(100vh - 100px); overflow-y: auto; font-size: 0.9rem; }
  .side-group { margin-bottom: 18px; }
  .side-title { color: var(--muted); text-transform: uppercase; font-size: 0.72rem; letter-spacing: 0.09em; margin-bottom: 6px; }
  .side a { display: block; color: var(--text); padding: 3px 0; }
  .side a:hover { color: var(--accent); text-decoration: none; }
  main { min-width: 0; }
  main > h1 { font-size: 1.9rem; letter-spacing: -0.01em; }
  main > .lead { color: var(--muted); margin: 8px 0 8px; }
  .cmd { border-top: 1px solid var(--border); padding-top: 30px; margin-top: 30px; }
  .cmd h2 { font-size: 1.25rem; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
  .cmd .short { color: var(--muted); margin: 5px 0 14px; }
  .cmd .long { white-space: pre-wrap; margin: 12px 0; }
  .cmd h3 { font-size: 0.95rem; color: var(--muted); text-transform: uppercase;
            letter-spacing: 0.07em; margin: 20px 0 8px; }
  .group-tag { font-size: 0.72rem; color: var(--muted); border: 1px solid var(--border);
               border-radius: 999px; padding: 2px 9px; margin-left: 8px; vertical-align: 3px; }
  pre {
    background: var(--bg-soft); border: 1px solid var(--border); border-radius: var(--radius);
    padding: 16px 18px; overflow-x: auto; font-size: 0.88rem; line-height: 1.7;
  }
  pre .c { color: #6f7896; }
  table { width: 100%; border-collapse: collapse; font-size: 0.9rem; }
  th, td { text-align: left; padding: 9px 12px; border-bottom: 1px solid var(--border); vertical-align: top; }
  th { color: var(--muted); font-weight: 600; }
  td code { color: var(--accent); }
  footer { border-top: 1px solid var(--border); padding: 28px 24px; color: var(--muted);
           font-size: 0.9rem; text-align: center; }
  @media (max-width: 820px) {
    .layout { grid-template-columns: 1fr; }
    .side { position: static; max-height: none; }
  }
</style>
</head>
<body>
<nav>
  <div class="row">
    <a class="brand" href="../">confluence-cli</a>
    <div class="links">
      <a href="../">Home</a>
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
  <main>
    <h1>CLI reference</h1>
    <p class="lead">{{.Intro}}</p>
    <p class="lead">This page is generated from the command tree, so it always matches <code>--help</code>.</p>

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
<footer>confluence-cli — released under the MIT License.</footer>
</body>
</html>
`
