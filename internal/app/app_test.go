package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/zalando/go-keyring"
)

func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

// mockConfluence is a minimal Data Center REST API server for command tests.
func mockConfluence(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/space", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[{"id":1,"key":"ENG","name":"Engineering","type":"global"}],"size":1,"limit":25}`))
	})
	mux.HandleFunc("/rest/api/space/ENG", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":1,"key":"ENG","name":"Engineering","type":"global"}`))
	})
	mux.HandleFunc("/rest/api/content/123", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"123","type":"page","status":"current","title":"Welcome",
			"space":{"key":"ENG"},"version":{"number":1},
			"body":{"storage":{"value":"<h1>Hi</h1><p>body text</p>","representation":"storage"}},
			"_links":{"webui":"/display/ENG/Welcome"}}`))
	})
	mux.HandleFunc("/rest/api/content/790", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"790","type":"page","status":"current","title":"Macro Page",
			"space":{"key":"ENG"},"version":{"number":1},
			"body":{"storage":{"value":"<p>before</p><ac:structured-macro ac:name=\"view-file\"><ac:parameter ac:name=\"name\"><ri:attachment ri:filename=\"resume.pdf\"/></ac:parameter></ac:structured-macro>","representation":"storage"}},
			"_links":{"webui":"/display/ENG/Macro"}}`))
	})
	mux.HandleFunc("/rest/api/content/404", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"No content found"}`))
	})
	mux.HandleFunc("/rest/api/user/current", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"type":"known","username":"tester","userKey":"ab12","displayName":"Test User"}`))
	})
	mux.HandleFunc("/rest/api/content/c1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPut:
			w.Write([]byte(`{"id":"c1","type":"comment","version":{"number":3},
				"body":{"storage":{"value":"<p>edited</p>","representation":"storage"}}}`))
		default:
			w.Write([]byte(`{"id":"c1","type":"comment","version":{"number":2},
				"body":{"storage":{"value":"<p>original</p>","representation":"storage"}}}`))
		}
	})
	mux.HandleFunc("/rest/api/content/123/version", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[
			{"number":2,"when":"2025-02-01T00:00:00Z","message":"edit","by":{"displayName":"Bob"}},
			{"number":1,"when":"2025-01-01T00:00:00Z","by":{"displayName":"Alice"}}],
			"size":2,"limit":25}`))
	})
	mux.HandleFunc("/rest/api/content/123/version/1", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"number":1,"content":{"id":"123","type":"page","title":"Welcome",
			"body":{"storage":{"value":"<p>v1 body</p>","representation":"storage"}}}}`))
	})
	mux.HandleFunc("/rest/api/user/watch/content/123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"watching":true}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/rest/api/content/att900", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"att900","type":"attachment","title":"notes.txt",
			"container":{"id":"123"},"extensions":{"fileSize":5}}`))
	})
	mux.HandleFunc("/rest/api/content/123/child/attachment", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[{"id":"att900","type":"attachment","title":"notes.txt",
			"extensions":{"fileSize":5,"mediaType":"text/plain"}}]}`))
	})
	mux.HandleFunc("/rest/api/content/123/label", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"results":[{"id":"l1","name":"release-notes","prefix":"global"}],"size":1,"limit":25}`))
			return
		}
		w.Write([]byte(`{"results":[
			{"id":"l1","name":"q3","prefix":"global"},
			{"id":"l2","name":"reviewed","prefix":"global"}],"size":2,"limit":25}`))
	})
	mux.HandleFunc("/rest/api/search", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[{"content":{"id":"123","type":"page","title":"Welcome",
			"space":{"key":"ENG"},"_links":{"webui":"/display/ENG/Welcome"}},
			"title":"Welcome","excerpt":"hello"}],"size":1,"limit":25}`))
	})
	// Stand-in for the GitHub "latest release" API used by the update check.
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"v99.0.0"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// runCLI executes the command tree with args against srv, returning stdout and
// the resulting error.
func runCLI(t *testing.T, srv *httptest.Server, args ...string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CONFLUENCE_SERVER", srv.URL)
	t.Setenv("CONFLUENCE_FLAVOR", "datacenter")
	t.Setenv("CONFLUENCE_PERSONAL_ACCESS_TOKEN", "test-token")
	// Keep the update check off the real GitHub API during tests.
	t.Setenv("CONFLUENCE_RELEASE_API", srv.URL+"/releases/latest")

	full := append([]string{"--config", dir}, args...)
	root := newRootCmd()
	root.SetArgs(full)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	outCh := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		outCh <- string(b)
	}()

	err := root.Execute()
	w.Close()
	os.Stdout = old
	return <-outCh, err
}

// decodeList parses a list command's output envelope ({items,next,has_more})
// and returns the items.
func decodeList(t *testing.T, out string) []map[string]any {
	t.Helper()
	var env struct {
		Items   []map[string]any `json:"items"`
		Next    string           `json:"next"`
		HasMore bool             `json:"has_more"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("output not a list envelope: %v\n%s", err, out)
	}
	return env.Items
}

func TestCmdVersion(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "confluence-cli") {
		t.Errorf("version output = %q", out)
	}
}

func TestVersionFlag(t *testing.T) {
	srv := mockConfluence(t)
	flagOut, err := runCLI(t, srv, "--version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(flagOut, "confluence-cli") {
		t.Errorf("--version output = %q", flagOut)
	}
	// `--version` and the `version` command must agree.
	cmdOut, err := runCLI(t, srv, "version")
	if err != nil {
		t.Fatal(err)
	}
	if flagOut != cmdOut {
		t.Errorf("--version (%q) and `version` (%q) disagree", flagOut, cmdOut)
	}
}

func TestCmdPageGet(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "page", "get", "123")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if got["title"] != "Welcome" {
		t.Errorf("title = %v", got["title"])
	}
	if body, _ := got["body"].(string); !strings.Contains(body, "# Hi") {
		t.Errorf("rendered body missing heading: %v", got["body"])
	}
}

func TestCmdPageGetOutlineScope(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "page", "get", "123", "--scope", "outline")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["scope_applied"] != "outline" {
		t.Errorf("scope_applied = %v", got["scope_applied"])
	}
}

func TestCmdPageGetOutputToFile(t *testing.T) {
	srv := mockConfluence(t)
	dest := filepath.Join(t.TempDir(), "page.md")
	out, err := runCLI(t, srv, "page", "get", "123", "--output", dest)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	// stdout carries metadata, not the body.
	if _, hasBody := got["body"]; hasBody {
		t.Errorf("stdout should omit body when --output is set: %v", got)
	}
	if got["output_path"] != dest {
		t.Errorf("output_path = %v; want %s", got["output_path"], dest)
	}
	// The body landed on disk.
	data, readErr := os.ReadFile(dest)
	if readErr != nil {
		t.Fatalf("body file not written: %v", readErr)
	}
	if !strings.Contains(string(data), "# Hi") {
		t.Errorf("body file missing rendered heading: %q", data)
	}
	if n, _ := got["bytes"].(float64); int(n) != len(data) {
		t.Errorf("bytes = %v; want %d", got["bytes"], len(data))
	}
}

func TestCmdPageGetOutputNoBodyConflicts(t *testing.T) {
	srv := mockConfluence(t)
	dest := filepath.Join(t.TempDir(), "x.md")
	_, err := runCLI(t, srv, "page", "get", "123", "--output", dest, "--no-body")
	if err == nil {
		t.Fatal("expected --output with --no-body to fail")
	}
	if !strings.Contains(err.Error(), "has nothing to write") {
		t.Errorf("error = %v; want the --no-body conflict message", err)
	}
}

func TestCmdPageGetFromURL(t *testing.T) {
	srv := mockConfluence(t)
	url := srv.URL + "/pages/viewpage.action?pageId=123"
	out, err := runCLI(t, srv, "page", "get", url, "--no-body")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Welcome") {
		t.Errorf("page get from URL failed: %q", out)
	}
}

func TestCmdSearch(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "search", "--text", "welcome")
	if err != nil {
		t.Fatal(err)
	}
	got := decodeList(t, out)
	if len(got) != 1 || got[0]["id"] != "123" {
		t.Errorf("search results = %v", got)
	}
}

func TestCmdSpaceList(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "space", "list", "--format", "table")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ENG") {
		t.Errorf("space list table missing ENG:\n%s", out)
	}
}

func TestCmdDoctor(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "doctor")
	if err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	json.Unmarshal([]byte(out), &report)
	if report["healthy"] != true {
		t.Errorf("doctor not healthy:\n%s", out)
	}
	// doctor folds in a release-update check; the mock advertises v99.0.0.
	// The test binary reports version "dev", so the comparison is skipped —
	// what we assert here is the wiring: the block is present and the latest
	// release was fetched from the (mocked) release API.
	upd, ok := report["update"].(map[string]any)
	if !ok {
		t.Fatalf("doctor report has no update block:\n%s", out)
	}
	if upd["latest"] != "99.0.0" {
		t.Errorf("update.latest = %v, want 99.0.0", upd["latest"])
	}
}

func TestCmdDoctorNoUpdateCheck(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "doctor", "--no-update-check")
	if err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	json.Unmarshal([]byte(out), &report)
	if report["healthy"] != true {
		t.Errorf("doctor not healthy:\n%s", out)
	}
	if _, present := report["update"]; present {
		t.Errorf("--no-update-check should omit the update block:\n%s", out)
	}
}

// TestCmdDoctorUpdateNonFatal proves that a failed update lookup is purely
// informational: doctor stays healthy and exits 0 even when the release API
// is unreachable.
func TestCmdDoctorUpdateNonFatal(t *testing.T) {
	srv := mockConfluence(t)
	dir := t.TempDir()
	t.Setenv("CONFLUENCE_SERVER", srv.URL)
	t.Setenv("CONFLUENCE_FLAVOR", "datacenter")
	t.Setenv("CONFLUENCE_PERSONAL_ACCESS_TOKEN", "test-token")
	// Point the update check at a closed port.
	t.Setenv("CONFLUENCE_RELEASE_API", "http://127.0.0.1:0/releases/latest")

	root := newRootCmd()
	root.SetArgs([]string{"--config", dir, "doctor"})

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	outCh := make(chan string)
	go func() { b, _ := io.ReadAll(r); outCh <- string(b) }()
	err := root.Execute()
	w.Close()
	os.Stdout = old
	out := <-outCh

	if err != nil {
		t.Fatalf("doctor failed despite update check being non-fatal: %v", err)
	}
	var report map[string]any
	json.Unmarshal([]byte(out), &report)
	if report["healthy"] != true {
		t.Errorf("doctor not healthy:\n%s", out)
	}
	upd, ok := report["update"].(map[string]any)
	if !ok {
		t.Fatalf("missing update block:\n%s", out)
	}
	if upd["available"] != false {
		t.Errorf("update.available = %v, want false on lookup failure", upd["available"])
	}
}

func TestCmdPageGetNotFound(t *testing.T) {
	srv := mockConfluence(t)
	_, err := runCLI(t, srv, "page", "get", "404")
	if err == nil {
		t.Fatal("expected error for missing page")
	}
	if cerrors.ExitCode(err) != cerrors.ExitNotFound {
		t.Errorf("exit code = %d, want %d", cerrors.ExitCode(err), cerrors.ExitNotFound)
	}
}

func TestCmdBadFlag(t *testing.T) {
	srv := mockConfluence(t)
	_, err := runCLI(t, srv, "page", "get", "123", "--nonsense")
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestCompletionEnumFlag(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "__complete", "page", "get", "x", "--scope", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"full", "outline", "section", "keyword"} {
		if !strings.Contains(out, want) {
			t.Errorf("--scope completion missing %q:\n%s", want, out)
		}
	}
}

func TestCompletionGlobalFlag(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "__complete", "--format", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"json", "table", "ndjson"} {
		if !strings.Contains(out, want) {
			t.Errorf("--format completion missing %q:\n%s", want, out)
		}
	}
}

func TestCompletionSpaceKeys(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "__complete", "space", "get", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ENG") {
		t.Errorf("space key completion missing ENG:\n%s", out)
	}
}

func TestCmdSkillInstall(t *testing.T) {
	srv := mockConfluence(t)
	dir := t.TempDir()
	out, err := runCLI(t, srv, "skill", "install", "--dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"status": "installed"`) {
		t.Errorf("unexpected output: %q", out)
	}
	for _, f := range []string{
		"confluence/SKILL.md",
		"confluence/references/reading-pages.md",
	} {
		if _, statErr := os.Stat(filepath.Join(dir, f)); statErr != nil {
			t.Errorf("expected installed file %s: %v", f, statErr)
		}
	}
}

func TestCmdSkillInstallAgent(t *testing.T) {
	srv := mockConfluence(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	out, err := runCLI(t, srv, "skill", "install", "--agent", "codex")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"agent": "codex"`) {
		t.Errorf("output missing codex agent: %q", out)
	}
	want := filepath.Join(home, ".codex", "skills", "confluence", "SKILL.md")
	if _, statErr := os.Stat(want); statErr != nil {
		t.Errorf("expected installed file %s: %v", want, statErr)
	}
}

func TestCmdSkillInstallDetect(t *testing.T) {
	srv := mockConfluence(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, m := range []string{".claude", ".codex"} {
		if err := os.Mkdir(filepath.Join(home, m), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runCLI(t, srv, "skill", "install")
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{`"agent": "claude-code"`, `"agent": "codex"`} {
		if !strings.Contains(out, tag) {
			t.Errorf("output missing %s: %q", tag, out)
		}
	}
	for _, p := range []string{
		filepath.Join(home, ".claude", "skills", "confluence", "SKILL.md"),
		filepath.Join(home, ".codex", "skills", "confluence", "SKILL.md"),
	} {
		if _, statErr := os.Stat(p); statErr != nil {
			t.Errorf("expected installed file %s: %v", p, statErr)
		}
	}
}

func TestCmdSkillInstallNoAgent(t *testing.T) {
	srv := mockConfluence(t)
	t.Setenv("HOME", t.TempDir())
	_, err := runCLI(t, srv, "skill", "install")
	if err == nil {
		t.Fatal("expected an error when no agent is detected")
	}
	if cerrors.ExitCode(err) != cerrors.ExitUsage {
		t.Errorf("exit code = %d, want %d", cerrors.ExitCode(err), cerrors.ExitUsage)
	}
}

func TestCmdSkillUninstall(t *testing.T) {
	srv := mockConfluence(t)
	dir := t.TempDir()
	if _, err := runCLI(t, srv, "skill", "install", "--dir", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, srv, "skill", "uninstall", "--dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"status": "removed"`) {
		t.Errorf("unexpected output: %q", out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "confluence")); !os.IsNotExist(statErr) {
		t.Errorf("Skill directory was not removed: %v", statErr)
	}
	// A second uninstall is a no-op, not an error.
	out, err = runCLI(t, srv, "skill", "uninstall", "--dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"status": "not_installed"`) {
		t.Errorf("expected not_installed on repeat uninstall: %q", out)
	}
}

func TestCmdSkillShow(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "skill", "show")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "name: confluence") {
		t.Errorf("skill show did not emit SKILL.md:\n%.120s", out)
	}
}

func TestCompletionScriptGenerates(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "completion", "bash")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "confluence-cli") {
		t.Errorf("bash completion script looks empty:\n%.200s", out)
	}
}
