package cli

import (
	"encoding/json"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// auditEntry mirrors one entry of GET /api/v1/settings/audit.
type auditEntry struct {
	Time   time.Time `json:"time"`
	Actor  string    `json:"actor"`
	Action string    `json:"action"`
	Host   string    `json:"host"`
	Target string    `json:"target"`
	Status int       `json:"status"`
}

func newAuditCmd(a *app) *cobra.Command {
	var (
		actor, action, target, host, since string
		failed                             bool
		limit                              int
	)

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "List recorded changes (who did what, where, and whether it worked), newest first",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			query := url.Values{"limit": {strconv.Itoa(limit)}}
			for key, value := range map[string]string{"actor": actor, "action": action, "target": target, "host": host} {
				if value != "" {
					query.Set(key, value)
				}
			}
			if failed {
				query.Set("result", "failure")
			}
			now := time.Now()
			sinceArg, err := parseTimeArg(since, now)
			if err != nil {
				return err
			}
			if sinceArg != "" {
				query.Set("since", sinceArg)
			}

			var resp struct {
				Entries json.RawMessage `json:"entries"`
				Total   int             `json:"total"`
			}
			if err := a.client.get(cmd.Context(), "/settings/audit", query, &resp); err != nil {
				return err
			}

			if a.jsonOutput() {
				var entries any
				if len(resp.Entries) > 0 {
					if err := json.Unmarshal(resp.Entries, &entries); err != nil {
						return err
					}
				}
				if entries == nil {
					entries = []any{}
				}
				return a.printJSON(map[string]any{"entries": entries, "total": resp.Total})
			}

			var entries []auditEntry
			if len(resp.Entries) > 0 {
				if err := json.Unmarshal(resp.Entries, &entries); err != nil {
					return err
				}
			}
			rows := make([][]string, 0, len(entries))
			for _, e := range entries {
				who := e.Actor
				if who == "" {
					who = "-"
				}
				where := e.Target
				if e.Host != "" {
					where += "@" + e.Host
				}
				rows = append(rows, []string{
					humanAge(e.Time, now),
					who,
					e.Action,
					where,
					strconv.Itoa(e.Status),
				})
			}
			renderTable(os.Stdout, []string{"TIME", "WHO", "ACTION", "TARGET", "STATUS"}, rows)
			return nil
		}),
	}

	cmd.Flags().StringVar(&actor, "actor", "", "only changes by this user or token (token:<name>); matches part of the name")
	cmd.Flags().StringVar(&action, "action", "", "only these actions, e.g. container.stop or container; matches part of the name")
	cmd.Flags().StringVar(&target, "target", "", "only changes to this container, project, rule or token; matches part of it")
	cmd.Flags().StringVar(&host, "host", "", "only changes on this host")
	cmd.Flags().StringVar(&since, "since", "", "only changes after this time (RFC3339 or relative: 30m, 2h, 1d)")
	cmd.Flags().BoolVar(&failed, "failed", false, "only changes that failed or were refused")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum number of entries to return (at most 500)")
	return cmd
}
