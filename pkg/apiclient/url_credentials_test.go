package apiclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/angelmsger/confluence-cli/pkg/transport"
)

func TestPageURLKeepsCredentialsOutOfOutput(t *testing.T) {
	for _, tc := range []struct {
		flavor Flavor
		path   string
	}{
		{FlavorCloud, "/wiki/rest/api/content/123"},
		{FlavorDataCenter, "/rest/api/content/123"},
	} {
		t.Run(string(tc.flavor), func(t *testing.T) {
			for _, absolute := range []bool{false, true} {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != tc.path {
						t.Errorf("path = %q, want %q", r.URL.Path, tc.path)
					}
					if user, password, ok := r.BasicAuth(); !ok || user != "reader" || password != "url-secret" {
						t.Error("request lost its URL credentials")
					}
					link := "/display/ENG/Welcome"
					if absolute {
						link = "https://reader:link-secret@pages.example.test" + link
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]any{
						"id": "123", "type": "page", "title": "Welcome",
						"_links": map[string]string{"webui": link},
					})
				}))
				t.Cleanup(server.Close)
				client := New(Config{
					Flavor:    tc.flavor,
					BaseURL:   strings.Replace(server.URL, "http://", "http://reader:url-secret@", 1),
					Transport: transport.New(transport.Options{}),
				})
				page, err := client.GetPage(context.Background(), "123", GetPageOpts{})
				if err != nil {
					t.Fatal(err)
				}
				want := server.URL + "/display/ENG/Welcome"
				if absolute {
					want = "https://pages.example.test/display/ENG/Welcome"
				}
				if page.URL != want {
					t.Errorf("page URL = %q, want %q", page.URL, want)
				}
			}
		})
	}
}

func TestAttachmentURLIsSafeWithoutLosingDownloadAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, password, ok := r.BasicAuth(); !ok || user != "reader" || password != "url-secret" {
			t.Error("same-origin request lost its URL credentials")
		}
		if r.URL.Path == "/download/file.pdf" {
			io.WriteString(w, "PDFDATA")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"att1","type":"attachment","title":"file.pdf","_links":{"download":"/download/file.pdf"}}`)
	}))
	defer server.Close()
	client := New(Config{
		Flavor:    FlavorDataCenter,
		BaseURL:   strings.Replace(server.URL, "http://", "http://reader:url-secret@", 1),
		Transport: transport.New(transport.Options{}),
	})
	attachment, err := client.GetAttachment(context.Background(), "att1")
	if err != nil {
		t.Fatal(err)
	}
	if want := server.URL + "/download/file.pdf"; attachment.DownloadURL != want {
		t.Fatalf("download URL = %q, want %q", attachment.DownloadURL, want)
	}
	var content strings.Builder
	if _, err := client.DownloadAttachment(context.Background(), *attachment, &content); err != nil {
		t.Fatal(err)
	}
	if content.String() != "PDFDATA" {
		t.Fatalf("download = %q", content.String())
	}

	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("URL credentials were forwarded to another origin")
		}
		io.WriteString(w, "EXTERNAL")
	}))
	defer external.Close()
	attachment.DownloadURL = external.URL + "/file.pdf"
	if _, err := client.DownloadAttachment(context.Background(), *attachment, io.Discard); err != nil {
		t.Fatal(err)
	}
}
