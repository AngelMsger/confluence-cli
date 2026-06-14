package cliflags

import (
	"reflect"
	"testing"
)

func testInfo() FlagInfo {
	return FlagInfo{
		Known:   map[string]bool{"user-id": true, "user-name": true, "limit": true, "format": true, "max-items": true, "dry-run": true},
		Numeric: map[string]bool{"limit": true, "max-items": true},
	}
}

func TestKebab(t *testing.T) {
	cases := map[string]string{
		"userId":    "user-id",
		"user_name": "user-name",
		"UserName":  "user-name",
		"user-id":   "user-id",
		"format":    "format",
		"maxItems":  "max-items",
	}
	for in, want := range cases {
		if got := kebab(in); got != want {
			t.Errorf("kebab(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestNormalize(t *testing.T) {
	info := testInfo()
	cases := []struct {
		name     string
		in       []string
		wantOut  []string
		wantKind []string
	}{
		{"flag-name camel", []string{"--userId", "7"}, []string{"--user-id", "7"}, []string{"flag-name"}},
		{"flag-name snake", []string{"--user_name=bob"}, []string{"--user-name=bob"}, []string{"flag-name"}},
		{"flag-name with eq", []string{"--userId=7"}, []string{"--user-id=7"}, []string{"flag-name"}},
		{"sticky int", []string{"--limit100"}, []string{"--limit", "100"}, []string{"sticky-value"}},
		{"sticky camel int", []string{"--maxItems50"}, []string{"--max-items", "50"}, []string{"sticky-value"}},
		{"sticky on non-numeric left alone", []string{"--format2"}, []string{"--format2"}, nil},
		{"already canonical", []string{"--limit", "5"}, []string{"--limit", "5"}, nil},
		{"unknown flag untouched", []string{"--bogus"}, []string{"--bogus"}, nil},
		{"short flag untouched", []string{"-v"}, []string{"-v"}, nil},
		{"after double dash untouched", []string{"--", "--userId"}, []string{"--", "--userId"}, nil},
		{"completion bypass", []string{"__complete", "--userId"}, []string{"__complete", "--userId"}, nil},
		{"positional untouched", []string{"page", "get", "123"}, []string{"page", "get", "123"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, corr := Normalize(tc.in, info)
			if !reflect.DeepEqual(out, tc.wantOut) {
				t.Errorf("out = %v; want %v", out, tc.wantOut)
			}
			var kinds []string
			for _, c := range corr {
				kinds = append(kinds, c.Kind)
			}
			if !reflect.DeepEqual(kinds, tc.wantKind) {
				t.Errorf("correction kinds = %v; want %v", kinds, tc.wantKind)
			}
		})
	}
}

// TestNormalizeNoFalsePositive guards the headline safety property: a token
// that does not normalize to a known flag is never rewritten.
func TestNormalizeNoFalsePositive(t *testing.T) {
	out, corr := Normalize([]string{"--statusRocket"}, testInfo())
	if len(corr) != 0 || !reflect.DeepEqual(out, []string{"--statusRocket"}) {
		t.Errorf("unknown camel flag should pass through: out=%v corr=%v", out, corr)
	}
}
