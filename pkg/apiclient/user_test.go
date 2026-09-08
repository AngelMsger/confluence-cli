package apiclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/angelmsger/confluence-cli/pkg/transport"
)

func TestSearchUsersCloudKeepsDedicatedEndpoint(t *testing.T) {
	t.Parallel()
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"user":{"accountId":"cloud-1","displayName":"Cloud User"}}],"start":25,"limit":25,"size":1}`))
	}))
	t.Cleanup(srv.Close)
	c := New(Config{Flavor: FlavorCloud, BaseURL: srv.URL, Transport: transport.New(transport.Options{})})

	res, err := c.SearchUsers(context.Background(), UserSearchOpts{ListOpts: ListOpts{Cursor: "25"}, Query: "Cloud"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/wiki/rest/api/search/user" || gotQuery.Get("start") != "25" {
		t.Errorf("request = %s?%s", gotPath, gotQuery.Encode())
	}
	if len(res.Items) != 1 || res.Items[0].AccountID != "cloud-1" {
		t.Fatalf("users = %+v", res.Items)
	}
}

func TestSearchUsersDataCenterUsesCQLUserSearch(t *testing.T) {
	t.Parallel()
	var gotPath string
	var gotQuery url.Values
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[
			{"user":{"username":"kodomo","userKey":"k1","displayName":"Kodomo.D"}}],
			"start":0,"limit":25,"size":2}`))
	}))

	res, err := c.SearchUsers(context.Background(), UserSearchOpts{ListOpts: ListOpts{Limit: 1}, Query: "Kodomo.D"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/rest/api/search" {
		t.Errorf("path = %q", gotPath)
	}
	if gotQuery.Get("start") != "0" || gotQuery.Get("limit") != "1" || gotQuery.Get("cql") != `user.fullname ~ "Kodomo.D"` {
		t.Errorf("query = %v", gotQuery)
	}
	if len(res.Items) != 1 || res.Items[0].Username != "kodomo" {
		t.Fatalf("users = %+v", res.Items)
	}
	if res.Next != "1" {
		t.Fatalf("next = %q", res.Next)
	}
}

func TestSearchUsersDataCenterGlobalListFallsBackToCQL(t *testing.T) {
	t.Parallel()
	var paths []string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/rest/api/user/list" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"results":[{"user":{"username":"alice","displayName":"Alice"}}],"size":1,"limit":25}`))
	}))

	res, err := c.SearchUsers(context.Background(), UserSearchOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].Username != "alice" {
		t.Fatalf("users = %+v", res.Items)
	}
	if len(paths) != 2 || paths[1] != "/rest/api/search?cql=type+%3D+user&limit=25&start=0" {
		t.Fatalf("requests = %v", paths)
	}
}

func TestGetUserDataCenterUsesStableUserEndpoint(t *testing.T) {
	t.Parallel()
	var gotPath string
	var gotQuery url.Values
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"username":"kodomo","userKey":"k1","displayName":"Kodomo.D"}`))
	}))

	u, err := c.GetUser(context.Background(), "kodomo")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/rest/api/user" || gotQuery.Get("username") != "kodomo" {
		t.Errorf("request = %s?%s", gotPath, gotQuery.Encode())
	}
	if u.Username != "kodomo" || u.UserKey != "k1" {
		t.Fatalf("user = %+v", u)
	}
}

func TestGetUserDataCenterFallsBackFromUsernameToUserKey(t *testing.T) {
	t.Parallel()
	var queries []string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("username") != "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"userKey":"k1","displayName":"Key User"}`))
	}))

	u, err := c.GetUser(context.Background(), "k1")
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) != 2 || queries[0] != "username=k1" || queries[1] != "key=k1" {
		t.Fatalf("queries = %v", queries)
	}
	if u.UserKey != "k1" {
		t.Fatalf("user = %+v", u)
	}
}

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
	if u.Username != "jdoe" || u.UserKey != "ff01" || u.DisplayName != "Jane Doe" {
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
	if u.Username != "ff01" || u.UserKey != "ff01" {
		t.Errorf("user key fallback = %+v", u)
	}
}
