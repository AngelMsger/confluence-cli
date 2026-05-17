package app

import (
	"context"

	"github.com/angelmsger/confluence-cli/internal/apiclient"
	"github.com/spf13/cobra"
)

// enumComplete registers a fixed set of completion values for a flag. Errors
// from registration are ignored: completion is a convenience, never required.
func enumComplete(cmd *cobra.Command, flag string, values ...string) {
	_ = cmd.RegisterFlagCompletionFunc(flag,
		cobra.FixedCompletions(values, cobra.ShellCompDirectiveNoFileComp))
}

// completeSpaceKeys returns a cobra completion function that suggests live
// space keys. It is best-effort: any failure yields no suggestions.
func completeSpaceKeys(s *appState) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		// Completion is invoked without the command's PreRun hooks, so the
		// configuration must be loaded explicitly here.
		if s.resolved == nil {
			if err := s.load(); err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), s.timeout())
		defer cancel()
		client, err := s.newClient(ctx)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		res, err := client.ListSpaces(ctx, apiclient.SpaceListOpts{
			ListOpts: apiclient.ListOpts{Limit: 50},
		})
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		keys := make([]string, 0, len(res.Items))
		for _, sp := range res.Items {
			if sp.Key != "" {
				keys = append(keys, sp.Key+"\t"+sp.Name)
			}
		}
		return keys, cobra.ShellCompDirectiveNoFileComp
	}
}
