package app

import "github.com/spf13/cobra"

func newWhoamiCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:     "whoami",
		Short:   "Print the user the configured credentials authenticate as",
		Example: "  confluence-cli whoami",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
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
		},
	}
}
