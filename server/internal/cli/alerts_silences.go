package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// alertSilence mirrors the server's silence JSON and doubles as the create
// request body.
type alertSilence struct {
	ID        string           `json:"id,omitempty"`
	Comment   string           `json:"comment,omitempty"`
	RuleID    string           `json:"ruleId,omitempty"`
	Host      string           `json:"host,omitempty"`
	Container string           `json:"container,omitempty"`
	Project   string           `json:"project,omitempty"`
	StartsAt  string           `json:"startsAt,omitempty"`
	EndsAt    string           `json:"endsAt,omitempty"`
	Schedule  *silenceSchedule `json:"schedule,omitempty"`
	Active    bool             `json:"active,omitempty"`
	Until     string           `json:"until,omitempty"`
}

type silenceSchedule struct {
	Days            []string `json:"days,omitempty"`
	Start           string   `json:"start"`
	DurationMinutes int      `json:"durationMinutes"`
	Timezone        string   `json:"timezone,omitempty"`
}

func newAlertSilencesCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "silences",
		Short: "List silences and maintenance windows",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			var resp struct {
				Silences json.RawMessage `json:"silences"`
			}
			if err := a.client.get(cmd.Context(), "/alerts/silences", nil, &resp); err != nil {
				return err
			}

			if a.jsonOutput() {
				var silences any
				if len(resp.Silences) > 0 {
					if err := json.Unmarshal(resp.Silences, &silences); err != nil {
						return err
					}
				}
				if silences == nil {
					silences = []any{}
				}
				return a.printJSON(map[string]any{"silences": silences})
			}

			var silences []alertSilence
			if len(resp.Silences) > 0 {
				if err := json.Unmarshal(resp.Silences, &silences); err != nil {
					return err
				}
			}
			rows := make([][]string, 0, len(silences))
			for _, s := range silences {
				state := "scheduled"
				if s.Active {
					state = "muting"
					if s.Until != "" {
						state += " until " + s.Until
					}
				}
				rows = append(rows, []string{s.ID, silenceTargets(s), silenceWhen(s), state, s.Comment})
			}
			renderTable(os.Stdout, []string{"ID", "MUTES", "WHEN", "STATE", "COMMENT"}, rows)
			return nil
		}),
	}
	cmd.AddCommand(newAlertSilenceAddCmd(a), newAlertSilenceDeleteCmd(a))
	return cmd
}

// silenceTargets summarizes what a silence mutes ("all" when unrestricted).
func silenceTargets(s alertSilence) string {
	var parts []string
	for _, p := range []struct{ key, value string }{
		{"rule", s.RuleID}, {"host", s.Host}, {"container", s.Container}, {"project", s.Project},
	} {
		if p.value != "" {
			parts = append(parts, p.key+"="+p.value)
		}
	}
	if len(parts) == 0 {
		return "all"
	}
	return strings.Join(parts, " ")
}

// silenceWhen summarizes a silence's time ("until deleted", "from X until Y",
// "sun 02:00 for 120m Europe/Berlin").
func silenceWhen(s alertSilence) string {
	if sch := s.Schedule; sch != nil {
		days := "daily"
		if len(sch.Days) > 0 {
			days = strings.Join(sch.Days, ",")
		}
		zone := sch.Timezone
		if zone == "" {
			zone = "UTC"
		}
		return fmt.Sprintf("%s %s for %dm %s", days, sch.Start, sch.DurationMinutes, zone)
	}
	when := "until deleted"
	if s.EndsAt != "" {
		when = "until " + s.EndsAt
	}
	if s.StartsAt != "" {
		when = "from " + s.StartsAt + " " + when
	}
	return when
}

func newAlertSilenceAddCmd(a *app) *cobra.Command {
	var (
		rule, host, container, project, comment string
		forDuration, from, until                string
		every                                   []string
		at, duration, timezone                  string
		req                                     alertSilence
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Mute alerts now, for a while, or in a recurring maintenance window",
		Long: `Mute the alerts that match every given --rule, --host, --container and
--project (none mutes everything). Muted alerts are kept in history as muted
and sent nowhere.

With no time flags the silence lasts until it is deleted. --for or --until
end it, and --from delays its start. --every, --at and --duration make it a
weekly maintenance window instead.`,
		Args: cobra.NoArgs,
		// Flag validation runs in PreRunE so bad combinations are usage
		// errors (exit 2) and never reach the server.
		PreRunE: func(cmd *cobra.Command, args []string) error {
			req = alertSilence{RuleID: rule, Host: host, Container: container, Project: project, Comment: comment}
			now := time.Now()

			if len(every) > 0 || at != "" || duration != "" {
				if forDuration != "" || from != "" || until != "" {
					return fmt.Errorf("--every/--at/--duration can't be combined with --for, --from or --until")
				}
				if at == "" || duration == "" {
					return fmt.Errorf("a maintenance window needs --at and --duration")
				}
				d, err := parseDuration(duration)
				if err != nil || d < time.Minute {
					return fmt.Errorf("invalid --duration %q (examples: 30m, 2h)", duration)
				}
				days := every
				if len(days) == 1 && days[0] == "daily" {
					days = nil
				}
				req.Schedule = &silenceSchedule{Days: days, Start: at, DurationMinutes: int(d / time.Minute), Timezone: timezone}
				return nil
			}
			if timezone != "" {
				return fmt.Errorf("--tz only applies to a maintenance window (--every)")
			}

			if forDuration != "" && until != "" {
				return fmt.Errorf("use either --for or --until")
			}
			start := now
			if from != "" {
				t, err := time.Parse(time.RFC3339, from)
				if err != nil {
					return fmt.Errorf("invalid --from %q (an RFC 3339 time, e.g. 2026-07-14T22:00:00Z)", from)
				}
				start = t
				req.StartsAt = from
			}
			if forDuration != "" {
				d, err := parseDuration(forDuration)
				if err != nil || d <= 0 {
					return fmt.Errorf("invalid --for %q (examples: 30m, 2h, 1d)", forDuration)
				}
				req.EndsAt = start.Add(d).UTC().Format(time.RFC3339)
			}
			if until != "" {
				if _, err := time.Parse(time.RFC3339, until); err != nil {
					return fmt.Errorf("invalid --until %q (an RFC 3339 time, e.g. 2026-07-14T23:00:00Z)", until)
				}
				req.EndsAt = until
			}
			return nil
		},
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			var created alertSilence
			var raw json.RawMessage
			if err := a.client.post(cmd.Context(), "/alerts/silences", nil, req, &raw); err != nil {
				return err
			}
			if a.jsonOutput() {
				return a.printJSON(raw)
			}
			if err := json.Unmarshal(raw, &created); err != nil {
				return err
			}
			fmt.Printf("added silence %s (%s, %s)\n", created.ID, silenceTargets(created), silenceWhen(created))
			return nil
		}),
	}

	cmd.Flags().StringVar(&rule, "rule", "", "mute only this rule id")
	cmd.Flags().StringVar(&host, "host", "", "mute only this host")
	cmd.Flags().StringVar(&container, "container", "", "mute only this container name")
	cmd.Flags().StringVar(&project, "project", "", "mute only this compose project")
	cmd.Flags().StringVar(&comment, "comment", "", "why, shown next to the silence")
	cmd.Flags().StringVar(&forDuration, "for", "", "how long to mute (e.g. 30m, 2h, 1d)")
	cmd.Flags().StringVar(&from, "from", "", "start muting at this RFC 3339 time instead of now")
	cmd.Flags().StringVar(&until, "until", "", "stop muting at this RFC 3339 time")
	cmd.Flags().StringSliceVar(&every, "every", nil, "maintenance window days: mon,tue,...,sun or daily")
	cmd.Flags().StringVar(&at, "at", "", "maintenance window start time of day (e.g. 02:00)")
	cmd.Flags().StringVar(&duration, "duration", "", "maintenance window length, at most 24h (e.g. 2h)")
	cmd.Flags().StringVar(&timezone, "tz", "", "maintenance window time zone (IANA name, e.g. Europe/Berlin; default UTC)")
	return cmd
}

func newAlertSilenceDeleteCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"unmute"},
		Short:   "Delete a silence, unmuting what it matched",
		Args:    cobra.ExactArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			if err := a.client.do(cmd.Context(), http.MethodDelete, "/alerts/silences/"+args[0], nil, nil, nil); err != nil {
				return err
			}
			if a.jsonOutput() {
				return a.printJSON(map[string]string{"message": "silence deleted", "id": args[0]})
			}
			fmt.Printf("deleted silence %q\n", args[0])
			return nil
		}),
	}
}
