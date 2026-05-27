package apiclient

import (
	"context"
	"net/url"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
)

// SearchUsers enumerates users matching a query — the discovery path for the
// `search --author` / `search --contributor` flags.
//
//	Cloud: GET /wiki/rest/api/search/user?cql=user.fullname~"..."  (CQL-driven;
//	       Query is required because Cloud has no global user-list endpoint)
//	DC:    GET /rest/api/1.0/users?filter=...                       (DC-wide
//	       user catalog under the /rest/api/1.0 namespace; Query is optional)
func (c *apiClient) SearchUsers(ctx context.Context, opt UserSearchOpts) (ListResult[User], error) {
	limit := c.limitOf(opt.ListOpts)
	if c.flavor == FlavorCloud {
		if opt.Query == "" {
			return ListResult[User]{}, cerrors.New(cerrors.CategoryUsage, "USER_NO_QUERY",
				"Confluence Cloud user search requires --query (Cloud has no global user list)").
				WithHint(`Pass --query "<substring>" to search by display name.`)
		}
		q := url.Values{}
		q.Set("cql", `user.fullname ~ "`+escapeQuotes(opt.Query)+`"`)
		q.Set("limit", itoaUser(limit))
		var raw struct {
			Results []struct {
				User struct {
					AccountID   string `json:"accountId"`
					Username    string `json:"username"`
					DisplayName string `json:"displayName"`
					Email       string `json:"email"`
					Type        string `json:"type"`
				} `json:"user"`
			} `json:"results"`
			Start int `json:"start"`
			Limit int `json:"limit"`
			Size  int `json:"size"`
		}
		if err := c.getJSON(ctx, "/wiki/rest/api/search/user", q, &raw); err != nil {
			return ListResult[User]{}, err
		}
		res := ListResult[User]{}
		for _, r := range raw.Results {
			res.Items = append(res.Items, User{
				AccountID:   r.User.AccountID,
				Username:    r.User.Username,
				DisplayName: r.User.DisplayName,
				Email:       r.User.Email,
				Type:        r.User.Type,
			})
		}
		return res, nil
	}
	q := offsetQuery(opt.Cursor, limit)
	if opt.Query != "" {
		q.Set("filter", opt.Query)
	}
	var raw struct {
		Values []struct {
			Name         string `json:"name"`
			Slug         string `json:"slug"`
			EmailAddress string `json:"emailAddress"`
			DisplayName  string `json:"displayName"`
			Active       bool   `json:"active"`
			Type         string `json:"type"`
		} `json:"values"`
		Size       int  `json:"size"`
		Limit      int  `json:"limit"`
		Start      int  `json:"start"`
		IsLastPage bool `json:"isLastPage"`
	}
	if err := c.getJSON(ctx, "/rest/api/1.0/users", q, &raw); err != nil {
		return ListResult[User]{}, err
	}
	res := ListResult[User]{}
	if !raw.IsLastPage && len(raw.Values) == limit {
		res.Next = itoaUser(raw.Start + raw.Limit)
	}
	for _, u := range raw.Values {
		res.Items = append(res.Items, User{
			Username:    u.Name,
			DisplayName: u.DisplayName,
			Email:       u.EmailAddress,
			Type:        u.Type,
		})
	}
	return res, nil
}

// GetUser fetches a single user by selector.
//
//	Cloud: GET /wiki/rest/api/user?accountId={selector}
//	DC:    GET /rest/api/1.0/users/{slug}
func (c *apiClient) GetUser(ctx context.Context, selector string) (*User, error) {
	if selector == "" {
		return nil, cerrors.New(cerrors.CategoryUsage, "USER_NO_SELECTOR",
			"a user selector is required (accountId on Cloud, username/slug on DC)")
	}
	if c.flavor == FlavorCloud {
		q := url.Values{}
		q.Set("accountId", selector)
		var raw struct {
			AccountID   string `json:"accountId"`
			DisplayName string `json:"displayName"`
			Email       string `json:"email"`
			Type        string `json:"type"`
		}
		if err := c.getJSON(ctx, "/wiki/rest/api/user", q, &raw); err != nil {
			return nil, err
		}
		return &User{
			AccountID:   raw.AccountID,
			DisplayName: raw.DisplayName,
			Email:       raw.Email,
			Type:        raw.Type,
		}, nil
	}
	var raw struct {
		Name         string `json:"name"`
		Slug         string `json:"slug"`
		EmailAddress string `json:"emailAddress"`
		DisplayName  string `json:"displayName"`
		Type         string `json:"type"`
	}
	if err := c.getJSON(ctx, "/rest/api/1.0/users/"+url.PathEscape(selector), nil, &raw); err != nil {
		return nil, err
	}
	return &User{
		Username:    raw.Name,
		DisplayName: raw.DisplayName,
		Email:       raw.EmailAddress,
		Type:        raw.Type,
	}, nil
}

func itoaUser(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func escapeQuotes(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			out = append(out, '\\', '"')
			continue
		}
		out = append(out, s[i])
	}
	return string(out)
}
