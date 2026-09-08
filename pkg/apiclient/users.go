package apiclient

import (
	"context"
	"net/url"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// SearchUsers enumerates users matching a query — the discovery path for the
// `search --author` / `search --contributor` flags.
//
//	Cloud: GET /wiki/rest/api/search/user?cql=user.fullname~"..."  (CQL-driven;
//	       Query is required because Cloud has no global user-list endpoint)
//	DC:    GET /rest/api/search?cql=user.fullname~"..."            (CQL user
//	       search), or GET /rest/api/user/list without a query
func (c *apiClient) SearchUsers(ctx context.Context, opt UserSearchOpts) (ListResult[User], error) {
	limit := c.limitOf(opt.ListOpts)
	if c.flavor == FlavorCloud {
		if opt.Query == "" {
			return ListResult[User]{}, cerrors.New(cerrors.CategoryUsage, "USER_NO_QUERY",
				"Confluence Cloud user search requires --query (Cloud has no global user list)").
				WithHint(`Pass --query "<substring>" to search by display name.`)
		}
		q := offsetQuery(opt.Cursor, limit)
		q.Set("cql", `user.fullname ~ "`+escapeQuotes(opt.Query)+`"`)
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
		// A full page implies there may be more; advance the offset cursor so
		// callers can page past the first batch (matching the DC branch below).
		if len(raw.Results) == limit {
			next := raw.Start + raw.Limit
			if raw.Limit == 0 {
				next = raw.Start + limit
			}
			res.Next = itoaUser(next)
		}
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
	if opt.Query != "" {
		return c.searchDataCenterUsers(ctx, opt.ListOpts, `user.fullname ~ "`+escapeQuotes(opt.Query)+`"`)
	}
	return c.listDataCenterUsers(ctx, opt.ListOpts)
}

func (c *apiClient) searchDataCenterUsers(ctx context.Context, opt ListOpts, cql string) (ListResult[User], error) {
	limit := c.limitOf(opt)
	q := offsetQuery(opt.Cursor, limit)
	q.Set("cql", cql)
	var raw struct {
		Results []struct {
			User rawUser `json:"user"`
		} `json:"results"`
		Start int `json:"start"`
		Limit int `json:"limit"`
		Size  int `json:"size"`
	}
	if err := c.getJSON(ctx, "/rest/api/search", q, &raw); err != nil {
		return ListResult[User]{}, err
	}
	res := ListResult[User]{Next: nextOffsetToken(opt.Cursor, limit, len(raw.Results))}
	for _, result := range raw.Results {
		res.Items = append(res.Items, *mapUser(result.User))
	}
	return res, nil
}

func (c *apiClient) listDataCenterUsers(ctx context.Context, opt ListOpts) (ListResult[User], error) {
	limit := c.limitOf(opt)
	q := offsetQuery(opt.Cursor, limit)
	var raw struct {
		Results []rawUser `json:"results"`
		Size    int       `json:"size"`
		Limit   int       `json:"limit"`
		Start   int       `json:"start"`
		Links   rawLinks  `json:"_links"`
	}
	if err := c.getJSON(ctx, "/rest/api/user/list", q, &raw); err != nil {
		if isHTTPNotFound(err) {
			return c.searchDataCenterUsers(ctx, opt, "type = user")
		}
		return ListResult[User]{}, err
	}
	res := ListResult[User]{Next: nextOffsetToken(opt.Cursor, limit, len(raw.Results))}
	for _, user := range raw.Results {
		res.Items = append(res.Items, *mapUser(user))
	}
	return res, nil
}

// GetUser fetches a single user by selector.
//
//	Cloud: GET /wiki/rest/api/user?accountId={selector}
//	DC:    GET /rest/api/user?username={selector}, then ?key= as a fallback
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
	q := url.Values{}
	q.Set("username", selector)
	var raw rawUser
	if err := c.getJSON(ctx, "/rest/api/user", q, &raw); err != nil {
		if !isHTTPNotFound(err) {
			return nil, err
		}
		q.Del("username")
		q.Set("key", selector)
		if err := c.getJSON(ctx, "/rest/api/user", q, &raw); err != nil {
			return nil, err
		}
	}
	return mapUser(raw), nil
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
