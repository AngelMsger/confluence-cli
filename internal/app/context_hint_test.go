package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angelmsger/confluence-cli/internal/config"
	"github.com/spf13/cobra"
)

// writeCtxConfig writes a config.yaml with the given body into a temp dir and
// returns the dir, for driving contextReminderBlock through a real load.
func writeCtxConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(config.ConfigFilePath(dir), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

const twoContextConfig = `current_context: alpha
contexts:
  - name: alpha
    server: https://alpha.example.com
    flavor: datacenter
    auth: {scheme: pat}
  - name: beta
    server: https://beta.example.com
    flavor: cloud
    auth: {scheme: basic, username: bob}
defaults:
  format: json
`

const oneContextConfig = `current_context: solo
contexts:
  - name: solo
    server: https://solo.example.com
    flavor: cloud
    auth: {scheme: pat}
`

func TestContextReminderBlock(t *testing.T) {
	t.Setenv("CONFLUENCE_CONTEXT", "")

	t.Run("multi context shows active and available", func(t *testing.T) {
		s := &appState{}
		s.gflags.configPath = writeCtxConfig(t, twoContextConfig)
		block := contextReminderBlock(s)
		for _, want := range []string{"Active context: alpha", "alpha, beta", "current_context", "--use-context"} {
			if !strings.Contains(block, want) {
				t.Errorf("block missing %q:\n%s", want, block)
			}
		}
	})

	t.Run("single context is silent", func(t *testing.T) {
		s := &appState{}
		s.gflags.configPath = writeCtxConfig(t, oneContextConfig)
		if block := contextReminderBlock(s); block != "" {
			t.Errorf("want empty block for single context, got:\n%s", block)
		}
	})

	t.Run("missing config is silent", func(t *testing.T) {
		s := &appState{}
		s.gflags.configPath = filepath.Join(t.TempDir(), "nope")
		if block := contextReminderBlock(s); block != "" {
			t.Errorf("want empty block when no config, got:\n%s", block)
		}
	})

	t.Run("flag selection labelled explicit", func(t *testing.T) {
		s := &appState{}
		s.gflags.configPath = writeCtxConfig(t, twoContextConfig)
		s.gflags.useContext = "beta"
		block := contextReminderBlock(s)
		if !strings.Contains(block, "Active context: beta") || !strings.Contains(block, "--use-context") {
			t.Errorf("block should name beta via flag:\n%s", block)
		}
	})
}

func TestContextHintApplies(t *testing.T) {
	root := newRootCmd()
	realCmd := findCmd(t, root, "page", "get")
	metaCmd := findCmd(t, root, "config", "init")

	multi := func(source string) *config.Resolved {
		return &config.Resolved{
			ActiveContext: "alpha",
			ContextSource: source,
			ContextNames:  []string{"alpha", "beta"},
		}
	}

	t.Run("implicit multi-context on a real command applies", func(t *testing.T) {
		s := &appState{resolved: multi(config.ContextSourceCurrent)}
		if !contextHintApplies(realCmd, s) {
			t.Error("want hint to apply")
		}
	})
	t.Run("explicit selection is silent", func(t *testing.T) {
		s := &appState{resolved: multi(config.ContextSourceFlag)}
		if contextHintApplies(realCmd, s) {
			t.Error("explicit flag selection should silence the hint")
		}
	})
	t.Run("single context is silent", func(t *testing.T) {
		s := &appState{resolved: &config.Resolved{ActiveContext: "solo", ContextSource: config.ContextSourceSingle, ContextNames: []string{"solo"}}}
		if contextHintApplies(realCmd, s) {
			t.Error("single context should silence the hint")
		}
	})
	t.Run("meta command is silent", func(t *testing.T) {
		s := &appState{resolved: multi(config.ContextSourceCurrent)}
		if contextHintApplies(metaCmd, s) {
			t.Error("config command should silence the hint")
		}
	})
	t.Run("opt-out env is silent", func(t *testing.T) {
		t.Setenv(envNoContextHint, "1")
		s := &appState{resolved: multi(config.ContextSourceCurrent)}
		if contextHintApplies(realCmd, s) {
			t.Error("opt-out env should silence the hint")
		}
	})
}

// findCmd walks the command tree to the leaf named by path.
func findCmd(t *testing.T, root *cobra.Command, path ...string) *cobra.Command {
	t.Helper()
	cur := root
	for _, name := range path {
		var match *cobra.Command
		for _, c := range cur.Commands() {
			if c.Name() == name {
				match = c
				break
			}
		}
		if match == nil {
			t.Fatalf("command %q not found under %q", name, cur.Name())
		}
		cur = match
	}
	return cur
}
