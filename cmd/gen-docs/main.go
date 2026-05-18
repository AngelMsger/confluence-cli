// Command gen-docs renders the confluence-cli command tree into Markdown
// reference docs under docs/cli/. Because it walks the very same cobra command
// tree the CLI runs, the generated reference can never drift from --help.
//
// Run it with `make docs`; CI regenerates and fails when the committed output
// is stale (see .github/workflows/ci.yml).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angelmsger/confluence-cli/internal/app"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// outDir is relative to the repository root, where `make docs` and CI run.
const outDir = "docs/cli"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen-docs:", err)
		os.Exit(1)
	}
}

func run() error {
	root := app.NewRootCmd()
	// Drop cobra's "Auto generated ... on <date>" footer so regenerating the
	// docs yields byte-identical output (the CI drift check depends on this).
	disableAutoGenTag(root)

	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := doc.GenMarkdownTree(root, outDir); err != nil {
		return err
	}
	if err := writeIndex(root); err != nil {
		return err
	}
	fmt.Printf("generated %s/\n", outDir)
	return nil
}

func disableAutoGenTag(cmd *cobra.Command) {
	cmd.DisableAutoGenTag = true
	for _, c := range cmd.Commands() {
		disableAutoGenTag(c)
	}
}

// docFile returns the Markdown filename cobra/doc uses for a command.
func docFile(cmd *cobra.Command) string {
	return strings.ReplaceAll(cmd.CommandPath(), " ", "_") + ".md"
}

// writeIndex writes docs/cli/README.md — the entry point GitHub renders when
// browsing the folder — listing every command as a nested tree.
func writeIndex(root *cobra.Command) error {
	var b strings.Builder
	b.WriteString("# confluence-cli command reference\n\n")
	b.WriteString("This reference is generated from the CLI command tree, so it always\n")
	b.WriteString("matches `--help`. Do not edit these files by hand — run `make docs`.\n\n")

	var walk func(cmd *cobra.Command, depth int)
	walk = func(cmd *cobra.Command, depth int) {
		if !cmd.IsAvailableCommand() || cmd.IsAdditionalHelpTopicCommand() {
			return
		}
		fmt.Fprintf(&b, "%s- [`%s`](%s) — %s\n",
			strings.Repeat("  ", depth), cmd.CommandPath(), docFile(cmd), cmd.Short)
		for _, c := range cmd.Commands() {
			walk(c, depth+1)
		}
	}
	walk(root, 0)

	return os.WriteFile(filepath.Join(outDir, "README.md"), []byte(b.String()), 0o644)
}
