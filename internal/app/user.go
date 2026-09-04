package app

import (
	"context"
	"strings"

	"github.com/angelmsger/confluence-cli/pkg/apiclient"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/spf13/cobra"
)

// newWhoamiCmd is the top-level convenience alias for `user me`. The
// stand-alone `whoami` is the universal Unix idiom and predates the `user`
// subtree, so the CLI keeps it.
func newWhoamiCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:     "whoami",
		Short:   "Print the user the configured credentials authenticate as",
		Example: "  confluence-cli whoami",
		Args:    cobra.NoArgs,
		RunE:    runWhoami(s),
	}
}

func runWhoami(s *appState) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		ctx, cancel := cmdContext(s)
		defer cancel()
		client, err := s.newClient(ctx)
		if err != nil {
			return err
		}
		user, err := client.CurrentUser(ctx)
		if err != nil {
			return err
		}
		return s.emit(user)
	}
}

func resolveUserSelector(ctx context.Context, client apiclient.Client, selector string) (*apiclient.User, error) {
	var (
		user *apiclient.User
		err  error
	)
	if strings.EqualFold(selector, "me") {
		user, err = client.CurrentUser(ctx)
	} else {
		user, err = client.GetUser(ctx, selector)
	}
	if err != nil {
		return nil, err
	}
	if !hasStableUserID(client.Flavor(), user) {
		return nil, cerrors.New(cerrors.CategoryAuth, "AUTH_IDENTITY_UNAVAILABLE",
			"Confluence did not return a stable identifier for the selected user").
			WithHint("Cloud requires account_id; Data Center requires username or user_key.")
	}
	return user, nil
}

func stableUserID(flavor apiclient.Flavor, user *apiclient.User) string {
	if user == nil {
		return ""
	}
	if flavor == apiclient.FlavorCloud {
		return user.AccountID
	}
	if user.Username == "" {
		return user.UserKey
	}
	return user.Username
}

func hasStableUserID(flavor apiclient.Flavor, user *apiclient.User) bool {
	if user == nil {
		return false
	}
	if flavor == apiclient.FlavorCloud {
		return user.AccountID != ""
	}
	return user.Username != "" || user.UserKey != ""
}

// newUserCmd is the discovery entry point for the user identifiers that the
// CQL filter flags `--author` / `--contributor` on `search` accept (Cloud
// accountId or DC username).
//
// Cloud requires --query because there is no global user-list endpoint —
// only the CQL-driven /wiki/rest/api/search/user.
// Data Center uses /rest/api/1.0/users and treats --query as optional.
func newUserCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "user",
		Short:   "Discover Confluence users — the values --author / --contributor accept",
		Aliases: []string{"users"},
	}
	cmd.AddCommand(newUserSearchCmd(s), newUserGetCmd(s), newUserMeCmd(s))
	return cmd
}

func newUserSearchCmd(s *appState) *cobra.Command {
	var (
		query  string
		limit  int
		all    bool
		cursor string
	)
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search users by display-name substring",
		Long: "Search Confluence users.\n\n" +
			"Cloud: --query is required (the CQL `user.fullname ~ \"...\"` search).\n" +
			"DC:    --query is optional (omit for a paginated walk of every user).",
		Aliases: []string{"list"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			fetch := func(c string) (apiclient.ListResult[apiclient.User], error) {
				return client.SearchUsers(ctx, apiclient.UserSearchOpts{
					ListOpts: apiclient.ListOpts{Limit: limit, Cursor: c},
					Query:    query,
				})
			}
			items, info, err := collectPage(fetch, cursor, all)
			if err != nil {
				return err
			}
			return s.emitList(items, info)
		},
	}
	f := cmd.Flags()
	f.StringVar(&query, "query", "", "display-name substring (required on Cloud)")
	addListFlags(cmd, &limit, &all, &cursor)
	return cmd
}

func newUserGetCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <selector>",
		Short: "Show details of a single user (accountId on Cloud; username on DC)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			u, err := client.GetUser(ctx, args[0])
			if err != nil {
				return err
			}
			return s.emit(u)
		},
	}
	return cmd
}

func newUserMeCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:     "me",
		Short:   "Print the user the configured credentials authenticate as (alias for whoami)",
		Aliases: []string{"current"},
		Args:    cobra.NoArgs,
		RunE:    runWhoami(s),
	}
}
