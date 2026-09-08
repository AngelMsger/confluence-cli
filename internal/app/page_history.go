package app

import (
	"os"
	"sort"
	"time"

	"github.com/angelmsger/confluence-cli/internal/output"
	"github.com/angelmsger/confluence-cli/pkg/apiclient"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/spf13/cobra"
)

func newPageHistoryCmd(s *appState) *cobra.Command {
	var (
		actor, since, from, to string
		limit                  int
		all                    bool
		cursor                 string
	)
	cmd := &cobra.Command{
		Use:   "history <id|url>...",
		Short: "List and filter the version history of one or more pages",
		Long: "List page version history. Pass several page references, or a single '-' to\n" +
			"read newline-separated references from stdin. Batch mode requires --since or\n" +
			"--from so automation cannot accidentally scan unbounded history. Inaccessible\n" +
			"pages are reported individually while the remaining pages are still read.",
		Example: "  confluence-cli page history 123456\n" +
			"  confluence-cli page history 123456 --all --format table\n" +
			"  confluence-cli page history 123456 --actor me --since 24h\n" +
			"  confluence-cli search --type page --contributor me --after 2026-09-03 --all --fields id | jq -r '.items[].id' | confluence-cli page history - --actor me --from 2026-09-03T00:00:00+08:00 --to 2026-09-04T00:00:00+08:00",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stdinMode := len(args) == 1 && args[0] == "-"
			inputs, err := collectBatchArgs(args, cmd.InOrStdin())
			if err != nil {
				return err
			}
			batchMode := stdinMode || len(inputs) > 1
			if batchMode && since == "" && from == "" {
				return cerrors.New(cerrors.CategoryUsage, "HISTORY_TIME_REQUIRED",
					"batch history queries require --since or --from").
					WithHint("Bound the version window so the command does not scan every page's full history.")
			}
			window, err := resolveHistoryWindow(since, from, to, time.Time{})
			if err != nil {
				return cerrors.Wrap(err, cerrors.CategoryUsage, "BAD_TIME_RANGE",
					"invalid history time range")
			}
			ids, err := resolvePageHistoryIDs(inputs)
			if err != nil {
				return err
			}
			if batchMode && cursor != "" {
				return cerrors.New(cerrors.CategoryUsage, "HISTORY_BATCH_CURSOR",
					"--cursor cannot be used with batch or stdin page references")
			}

			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			var targetActor *apiclient.User
			if actor != "" {
				targetActor, err = resolveUserSelector(ctx, client, actor)
				if err != nil {
					return err
				}
			}

			filteredMode := batchMode || window.set || targetActor != nil
			var versions []apiclient.PageVersion
			missingActors := 0
			failedPages := 0
			var lastFailure *cerrors.CLIError
			for _, id := range ids {
				fetch := func(c string) (apiclient.ListResult[apiclient.PageVersion], error) {
					return client.ListPageVersions(ctx, id, apiclient.ListOpts{Limit: limit, Cursor: c})
				}
				var (
					items []apiclient.PageVersion
					info  pageInfo
				)
				if window.set {
					items, err = collectPageVersionsInWindow(fetch, cursor, window)
				} else {
					items, info, err = collectPage(fetch, cursor, all || filteredMode)
				}
				if err != nil {
					if !batchMode {
						return err
					}
					failure := cerrors.AsCLIError(err)
					output.EmitNotice(os.Stderr, map[string]any{"_notice": map[string]any{
						"code":    "HISTORY_SOURCE_FAILED",
						"message": "could not read one page's version history; remaining pages will still be processed",
						"data": map[string]any{
							"page_id": id,
							"error":   failure.Payload().Error,
						},
					}})
					failedPages++
					lastFailure = failure
					continue
				}
				if !filteredMode {
					return s.emitList(items, info)
				}
				items, missing := filterPageVersionsByActor(items, targetActor, client.Flavor())
				missingActors += missing
				versions = append(versions, items...)
			}
			if missingActors > 0 {
				output.EmitNotice(os.Stderr, map[string]any{"_notice": map[string]any{
					"code":                          "HISTORY_ACTOR_COVERAGE",
					"message":                       "some candidate versions lacked a stable actor identifier and were excluded",
					"versions_without_stable_actor": missingActors,
				}})
			}
			sortPageVersionsNewestFirst(versions)
			if err := s.emitList(versions, pageInfo{}); err != nil {
				return err
			}
			if failedPages > 0 {
				return cerrors.Newf(lastFailure.Category, "BATCH_PARTIAL_FAILURE",
					"%d of %d page history queries failed; successful versions are on stdout", failedPages, len(ids)).
					WithHint("Inspect the HISTORY_SOURCE_FAILED notices for each inaccessible page.")
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&actor, "actor", "", "only versions by this user selector; use 'me' for the authenticated user")
	f.StringVar(&since, "since", "", "only versions within this recent duration, such as 24h or 7d")
	f.StringVar(&from, "from", "", "versions at or after this RFC3339 timestamp or UTC date")
	f.StringVar(&to, "to", "", "versions before this RFC3339 timestamp or UTC date (defaults to now)")
	addListFlags(cmd, &limit, &all, &cursor)
	return cmd
}

func resolvePageHistoryIDs(inputs []string) ([]string, error) {
	seen := make(map[string]struct{}, len(inputs))
	ids := make([]string, 0, len(inputs))
	for _, input := range inputs {
		id, err := resolvePageID(input)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func sortPageVersionsNewestFirst(items []apiclient.PageVersion) {
	sort.SliceStable(items, func(i, j int) bool {
		left, leftErr := parseHistoryInstant(items[i].When)
		right, rightErr := parseHistoryInstant(items[j].When)
		if leftErr != nil || rightErr != nil {
			return items[i].When > items[j].When
		}
		return left.After(right)
	})
}

func newPageRestoreCmd(s *appState) *cobra.Command {
	var (
		version int
		message string
		dryRun  bool
	)
	cmd := &cobra.Command{
		Use:   "restore <id|url> --version <N>",
		Short: "Restore a page to an earlier version",
		Long: "Republish an earlier version's body as a new version. The restore is\n" +
			"non-destructive: the version history is left intact. Run `page history`\n" +
			"first to find the version number to restore.",
		Example: "  confluence-cli page restore 123456 --version 3\n" +
			"  confluence-cli page restore 123456 --version 3 --message \"roll back bad edit\"",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolvePageID(args[0])
			if err != nil {
				return err
			}
			if version <= 0 {
				return cerrors.New(cerrors.CategoryUsage, "RESTORE_NO_VERSION",
					"--version must be a positive version number to restore")
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			req := apiclient.RestorePageReq{ID: id, Version: version, Message: message}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			page, err := client.RestorePage(ctx, req)
			if err != nil {
				return err
			}
			return s.emit(pageMetaOutput(page))
		},
	}
	f := cmd.Flags()
	f.IntVar(&version, "version", 0, "the version number to restore")
	f.StringVar(&message, "message", "", "version comment for the restore")
	f.BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	return cmd
}
