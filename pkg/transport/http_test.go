package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/angelmsger/confluence-cli/pkg/constants"
)

// The client is tested in agent-cli-kit; these check confluence-cli's defaults.
func TestNewSendsConfluenceUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := New(Options{}).Do(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got != constants.UserAgent() {
		t.Fatalf("User-Agent = %q, want %q", got, constants.UserAgent())
	}
}

// Attachment and media downloads can redirect to another origin, so New must
// not opt into the kit's same-origin redirect guard.
func TestNewFollowsCrossOriginRedirects(t *testing.T) {
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer dest.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, dest.URL+"/download", http.StatusFound)
	}))
	defer src.Close()

	req, _ := http.NewRequest(http.MethodGet, src.URL, nil)
	resp, err := New(Options{}).Do(context.Background(), req)
	if err != nil {
		t.Fatalf("cross-origin redirect should be followed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the redirect target", resp.StatusCode)
	}
}
