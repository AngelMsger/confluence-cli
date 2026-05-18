package update

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// doerFunc adapts a function to transport.Doer.
type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

// jsonResponder returns a Doer that replies with status and body.
func jsonResponder(status int, body string) doerFunc {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": {"application/json"}},
		}, nil
	}
}

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want [3]int
		ok   bool
	}{
		{"1.2.3", [3]int{1, 2, 3}, true},
		{"v0.0.2", [3]int{0, 0, 2}, true},
		{"0.0.2-rc1", [3]int{0, 0, 2}, true},
		{"1.4", [3]int{1, 4, 0}, true},
		{" 2 ", [3]int{2, 0, 0}, true},
		{"dev", [3]int{}, false},
		{"none", [3]int{}, false},
		{"1.2.3.4", [3]int{}, false},
		{"1.x.0", [3]int{}, false},
		{"", [3]int{}, false},
	}
	for _, c := range cases {
		got, ok := parse(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parse(%q) = %v,%v; want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestLess(t *testing.T) {
	cases := []struct {
		a, b [3]int
		want bool
	}{
		{[3]int{0, 0, 1}, [3]int{0, 0, 2}, true},
		{[3]int{0, 9, 9}, [3]int{1, 0, 0}, true},
		{[3]int{1, 0, 0}, [3]int{1, 0, 0}, false},
		{[3]int{2, 0, 0}, [3]int{1, 9, 9}, false},
		{[3]int{1, 2, 3}, [3]int{1, 2, 2}, false},
	}
	for _, c := range cases {
		if got := less(c.a, c.b); got != c.want {
			t.Errorf("less(%v,%v) = %v; want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestCheckUpdateAvailable(t *testing.T) {
	st := Check(context.Background(), jsonResponder(200, `{"tag_name":"v1.2.0"}`), "1.0.0")
	if !st.Available {
		t.Fatalf("expected an update to be available: %+v", st)
	}
	if st.Current != "1.0.0" || st.Latest != "1.2.0" {
		t.Errorf("current/latest = %q/%q", st.Current, st.Latest)
	}
	if !strings.Contains(st.Detail, "newer release") {
		t.Errorf("detail = %q", st.Detail)
	}
}

func TestCheckUpToDate(t *testing.T) {
	st := Check(context.Background(), jsonResponder(200, `{"tag_name":"v1.2.0"}`), "1.2.0")
	if st.Available {
		t.Errorf("did not expect an update: %+v", st)
	}
	if !strings.Contains(st.Detail, "up to date") {
		t.Errorf("detail = %q", st.Detail)
	}
}

func TestCheckNewerThanLatest(t *testing.T) {
	// A locally built version ahead of the published release.
	st := Check(context.Background(), jsonResponder(200, `{"tag_name":"v1.0.0"}`), "1.1.0")
	if st.Available {
		t.Errorf("local build ahead of release should not report an update: %+v", st)
	}
}

func TestCheckDevBuild(t *testing.T) {
	st := Check(context.Background(), jsonResponder(200, `{"tag_name":"v1.2.0"}`), "dev")
	if st.Available {
		t.Errorf("dev build must not report an update: %+v", st)
	}
	if !strings.Contains(st.Detail, "comparison skipped") {
		t.Errorf("detail = %q", st.Detail)
	}
}

func TestCheckHTTPError(t *testing.T) {
	st := Check(context.Background(), jsonResponder(503, `service unavailable`), "1.0.0")
	if st.Available {
		t.Errorf("a failed lookup must not report an update: %+v", st)
	}
	if !strings.Contains(st.Detail, "could not check") {
		t.Errorf("detail = %q", st.Detail)
	}
}

func TestCheckMalformedBody(t *testing.T) {
	st := Check(context.Background(), jsonResponder(200, `not json`), "1.0.0")
	if st.Available || !strings.Contains(st.Detail, "could not check") {
		t.Errorf("malformed body should degrade gracefully: %+v", st)
	}
}

func TestCheckEmptyTag(t *testing.T) {
	st := Check(context.Background(), jsonResponder(200, `{"tag_name":""}`), "1.0.0")
	if st.Available || !strings.Contains(st.Detail, "could not check") {
		t.Errorf("empty tag should degrade gracefully: %+v", st)
	}
}

func TestEndpointEnvOverride(t *testing.T) {
	if got := endpoint(); got != DefaultEndpoint {
		t.Errorf("endpoint() = %q; want default", got)
	}
	t.Setenv(EndpointEnv, "https://example.test/latest")
	if got := endpoint(); got != "https://example.test/latest" {
		t.Errorf("endpoint() with override = %q", got)
	}
}
