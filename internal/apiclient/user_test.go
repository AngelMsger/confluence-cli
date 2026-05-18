package apiclient

import (
	"context"
	"net/http"
	"testing"
)

func TestCurrentUserDataCenter(t *testing.T) {
	t.Parallel()
	var gotPath string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"type":"known","username":"jdoe","userKey":"ff01",
			"displayName":"Jane Doe"}`))
	}))

	u, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/rest/api/user/current" {
		t.Errorf("path = %q", gotPath)
	}
	if u.Username != "jdoe" || u.DisplayName != "Jane Doe" {
		t.Errorf("user = %+v", u)
	}
}

func TestCurrentUserCloud(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"type":"known","accountId":"5b10","displayName":"Sam Cloud"}`))
	}))

	u, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.AccountID != "5b10" || u.DisplayName != "Sam Cloud" {
		t.Errorf("user = %+v", u)
	}
}

// TestCurrentUserUserKeyFallback proves the username falls back to userKey when
// the server omits a username.
func TestCurrentUserUserKeyFallback(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"type":"known","userKey":"ff01","displayName":"Key Only"}`))
	}))

	u, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "ff01" {
		t.Errorf("username fallback = %q, want ff01", u.Username)
	}
}
