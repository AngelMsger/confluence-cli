package apiclient

import (
	"context"
	"net/http"
	"testing"
)

func TestWatchStatus(t *testing.T) {
	t.Parallel()
	var gotPath string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"watching":true}`))
	}))

	watching, err := c.WatchStatus(context.Background(), "123")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/rest/api/user/watch/content/123" {
		t.Errorf("path = %q", gotPath)
	}
	if !watching {
		t.Error("expected watching = true")
	}
}

func TestSetWatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		watching bool
		method   string
	}{
		{"watch", true, http.MethodPost},
		{"unwatch", false, http.MethodDelete},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got string
			c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Method + " " + r.URL.Path
				w.WriteHeader(http.StatusNoContent)
			}))
			err := c.SetWatch(context.Background(), WatchReq{PageID: "123", Watching: tc.watching})
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.method+" /rest/api/user/watch/content/123" {
				t.Errorf("request = %q", got)
			}
		})
	}
}

func TestWatchValidation(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if _, err := c.WatchStatus(context.Background(), ""); err == nil {
		t.Error("expected error for missing page ID")
	}
	if err := c.SetWatch(context.Background(), WatchReq{Watching: true}); err == nil {
		t.Error("expected error for missing page ID")
	}
}

func TestDescribeWriteWatch(t *testing.T) {
	t.Parallel()
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("dry-run must not send a request: %s %s", r.Method, r.URL.Path)
	}))

	plan, err := c.DescribeWrite(context.Background(), WatchReq{PageID: "123", Watching: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != http.MethodPost || plan.URL != srv.URL+"/rest/api/user/watch/content/123" {
		t.Errorf("watch plan = %s %s", plan.Method, plan.URL)
	}

	rm, err := c.DescribeWrite(context.Background(), WatchReq{PageID: "123", Watching: false})
	if err != nil {
		t.Fatal(err)
	}
	if rm.Method != http.MethodDelete {
		t.Errorf("unwatch plan method = %s", rm.Method)
	}
}
