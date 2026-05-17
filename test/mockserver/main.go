// Command mockserver is a minimal in-memory Confluence Data Center REST API,
// used by scripts/e2e.sh to exercise confluence-cli end-to-end without a real
// server. It prints its base URL on the first line of stdout, then serves.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
)

func main() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mockserver: listen failed:", err)
		os.Exit(1)
	}
	fmt.Printf("http://%s\n", ln.Addr().String())
	os.Stdout.Sync()

	if err := http.Serve(ln, routes()); err != nil {
		fmt.Fprintln(os.Stderr, "mockserver:", err)
		os.Exit(1)
	}
}

func routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /rest/api/space", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"results": []any{space("ENG", "Engineering")},
			"size":    1, "limit": 25,
		})
	})
	mux.HandleFunc("GET /rest/api/space/{key}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, space(r.PathValue("key"), "Engineering"))
	})

	mux.HandleFunc("GET /rest/api/content/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "404" {
			http.Error(w, `{"message":"No content found"}`, http.StatusNotFound)
			return
		}
		if id == "att1" {
			writeJSON(w, attachment("att1", "spec.txt"))
			return
		}
		writeJSON(w, page(id, "Welcome"))
	})
	mux.HandleFunc("GET /rest/api/content/{id}/child/page", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"results": []any{page("201", "Child One"), page("202", "Child Two")},
			"size":    2, "limit": 25,
		})
	})
	mux.HandleFunc("GET /rest/api/content/{id}/descendant/page", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"results": []any{page("201", "Child One")},
			"size":    1, "limit": 25,
		})
	})
	mux.HandleFunc("GET /rest/api/content/{id}/child/comment", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"results": []any{comment("c1", "<p>First comment</p>")},
			"size":    1, "limit": 25,
		})
	})
	mux.HandleFunc("GET /rest/api/content/{id}/child/attachment", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"results": []any{attachment("att1", "spec.txt")},
			"size":    1, "limit": 25,
		})
	})
	mux.HandleFunc("GET /rest/api/search", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"results": []any{map[string]any{
				"content": page("123", "Welcome"),
				"title":   "Welcome", "excerpt": "a warm welcome",
			}},
			"size": 1, "limit": 25,
		})
	})
	mux.HandleFunc("POST /rest/api/content", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, comment("new-comment", "<p>posted</p>"))
	})
	mux.HandleFunc("GET /download/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("attachment payload\n"))
	})

	return mux
}

func space(key, name string) map[string]any {
	return map[string]any{
		"id": 1, "key": key, "name": name, "type": "global",
		"_links": map[string]any{"webui": "/display/" + key},
	}
}

func page(id, title string) map[string]any {
	return map[string]any{
		"id": id, "type": "page", "status": "current", "title": title,
		"space":   map[string]any{"key": "ENG", "name": "Engineering"},
		"version": map[string]any{"number": 2, "when": "2025-05-01T00:00:00Z"},
		"body": map[string]any{"storage": map[string]any{
			"value": "<h1>Overview</h1><p>Body text for " + title +
				".</p><h2>Details</h2><p>More detail here.</p>",
			"representation": "storage",
		}},
		"_links": map[string]any{"webui": "/display/ENG/" + strings.ReplaceAll(title, " ", "+")},
	}
}

func comment(id, body string) map[string]any {
	return map[string]any{
		"id": id, "type": "comment",
		"version": map[string]any{"number": 1},
		"body":    map[string]any{"storage": map[string]any{"value": body, "representation": "storage"}},
		"_links":  map[string]any{"webui": "/display/ENG/comment"},
	}
}

func attachment(id, title string) map[string]any {
	return map[string]any{
		"id": id, "type": "attachment", "title": title,
		"metadata":   map[string]any{"mediaType": "text/plain"},
		"extensions": map[string]any{"fileSize": 19},
		"_links":     map[string]any{"download": "/download/attachments/123/" + title},
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
