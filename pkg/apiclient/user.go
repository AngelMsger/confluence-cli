package apiclient

import "context"

// user.go holds the current-user lookup. The v1 user/current endpoint returns
// the user behind the credentials — no user parameter is needed.

// rawUser is the v1 user object. Data Center populates username/userKey,
// Cloud populates accountId; displayName is set by both.
type rawUser struct {
	Type        string `json:"type"`
	AccountID   string `json:"accountId"`
	Username    string `json:"username"`
	Name        string `json:"name"`
	UserKey     string `json:"userKey"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	EmailAddr   string `json:"emailAddress"`
}

func mapUser(r rawUser) *User {
	u := &User{
		AccountID:   r.AccountID,
		Username:    firstNonEmpty(r.Username, r.Name),
		UserKey:     r.UserKey,
		DisplayName: r.DisplayName,
		Email:       firstNonEmpty(r.Email, r.EmailAddr),
		Type:        r.Type,
	}
	if u.Username == "" {
		u.Username = r.UserKey
	}
	return u
}

// CurrentUser returns the user the configured credentials authenticate as.
func (c *apiClient) CurrentUser(ctx context.Context) (*User, error) {
	var raw rawUser
	if err := c.getJSON(ctx, c.v1Base()+"/user/current", nil, &raw); err != nil {
		return nil, err
	}
	return mapUser(raw), nil
}
