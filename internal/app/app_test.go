package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
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
	mux.HandleFunc("/rest/api/content/404", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"No content found"}`))
	})
	mux.HandleFunc("/rest/api/search", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[{"content":{"id":"123","type":"page","title":"Welcome",
			"space":{"key":"ENG"},"_links":{"webui":"/display/ENG/Welcome"}},
			"title":"Welcome","excerpt":"hello"}],"size":1,"limit":25}`))
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
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not a JSON array: %v\n%s", err, out)
	}
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
