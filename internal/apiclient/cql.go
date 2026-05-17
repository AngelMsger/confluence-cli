package apiclient

import (
	"strings"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
)

// CQLParams describes a search expressed as discrete filters. BuildCQL turns it
// into a Confluence Query Language string.
type CQLParams struct {
	Text        string // free-text match (text ~ "...")
	Author      string // original creator (creator = "...")
	Contributor string // any contributor (contributor = "...")
	Space       string // space key (space = "...")
	Label       string // label (label = "...")
	Type        string // content type: page, blogpost, comment, attachment
	After       string // modified on/after this date (lastmodified >= "...")
	Before      string // modified on/before this date (lastmodified <= "...")
}

// IsEmpty reports whether no filter was supplied.
func (p CQLParams) IsEmpty() bool {
	return p == CQLParams{}
}

// validTypes are the content types accepted for the type filter.
var validTypes = map[string]bool{
	"page": true, "blogpost": true, "comment": true, "attachment": true,
}

// BuildCQL assembles a CQL string from the filters, AND-joining each clause.
func BuildCQL(p CQLParams) (string, error) {
	var clauses []string
	if p.Text != "" {
		clauses = append(clauses, `text ~ `+quote(p.Text))
	}
	if p.Author != "" {
		clauses = append(clauses, `creator = `+quote(p.Author))
	}
	if p.Contributor != "" {
		clauses = append(clauses, `contributor = `+quote(p.Contributor))
	}
	if p.Space != "" {
		clauses = append(clauses, `space = `+quote(p.Space))
	}
	if p.Label != "" {
		clauses = append(clauses, `label = `+quote(p.Label))
	}
	if p.Type != "" {
		t := strings.ToLower(p.Type)
		if !validTypes[t] {
			return "", cerrors.Newf(cerrors.CategoryUsage, "CQL_BAD_TYPE",
				"unknown content type %q (want page, blogpost, comment or attachment)", p.Type)
		}
		clauses = append(clauses, "type = "+t)
	}
	if p.After != "" {
		clauses = append(clauses, `lastmodified >= `+quote(p.After))
	}
	if p.Before != "" {
		clauses = append(clauses, `lastmodified <= `+quote(p.Before))
	}
	if len(clauses) == 0 {
		return "", cerrors.New(cerrors.CategoryUsage, "CQL_EMPTY",
			"no search filters were provided").
			WithNextSteps("Pass a raw CQL string, or use --text/--author/--space/--label/--type.")
	}
	return strings.Join(clauses, " AND "), nil
}

// quote wraps a value in double quotes, escaping any embedded quotes.
func quote(v string) string {
	return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
}
