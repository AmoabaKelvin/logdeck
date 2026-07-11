package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// API response shapes for /api/v1/alerts. Field names mirror the server's
// JSON exactly.

// alertRule doubles as the create request body: omitempty keeps fields the
// user didn't set out of the payload, so the server applies its defaults.
type alertRule struct {
	ID              string   `json:"id,omitempty"`
	Name            string   `json:"name"`
	Enabled         bool     `json:"enabled"`
	Type            string   `json:"type"`
	Hosts           []string `json:"hosts,omitempty"`
	Containers      []string `json:"containers,omitempty"`
	Projects        []string `json:"projects,omitempty"`
	Events          []string `json:"events,omitempty"`
	MinLevel        string   `json:"minLevel,omitempty"`
	Pattern         string   `json:"pattern,omitempty"`
	Threshold       int      `json:"threshold,omitempty"`
	WindowSeconds   int      `json:"windowSeconds,omitempty"`
	CooldownSeconds int      `json:"cooldownSeconds,omitempty"`
}

type alertDelivery struct {
	Status     string `json:"status"`
	HTTPStatus int    `json:"httpStatus"`
	Error      string `json:"error"`
}

type alertInfo struct {
	ID            string        `json:"id"`
	RuleID        string        `json:"ruleId"`
	RuleName      string        `json:"ruleName"`
	Type          string        `json:"type"`
	Host          string        `json:"host"`
	ContainerID   string        `json:"containerId"`
	ContainerName string        `json:"containerName"`
	Reason        string        `json:"reason"`
	Sample        string        `json:"sample"`
	Count         int           `json:"count"`
	Suppressed    bool          `json:"suppressed"`
	FiredAt       time.Time     `json:"firedAt"`
	Delivery      alertDelivery `json:"delivery"`
}

func newAlertsCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alerts",
		Short: "Manage alert rules, the webhook, and fired-alert history",
	}
	cmd.AddCommand(
		newAlertRulesCmd(a),
		newAlertHistoryCmd(a),
		newAlertWebhookCmd(a),
		newAlertTestCmd(a),
	)
	return cmd
}

func newAlertRulesCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "List alert rules",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			var resp struct {
				Rules json.RawMessage `json:"rules"`
			}
			if err := a.client.get(cmd.Context(), "/alerts/rules", nil, &resp); err != nil {
				return err
			}

			if a.jsonOutput() {
				var rules any
				if len(resp.Rules) > 0 {
					if err := json.Unmarshal(resp.Rules, &rules); err != nil {
						return err
					}
				}
				if rules == nil {
					rules = []any{}
				}
				return a.printJSON(map[string]any{"rules": rules})
			}

			var rules []alertRule
			if len(resp.Rules) > 0 {
				if err := json.Unmarshal(resp.Rules, &rules); err != nil {
					return err
				}
			}
			rows := make([][]string, 0, len(rules))
			for _, r := range rules {
				rows = append(rows, []string{
					r.ID,
					r.Name,
					r.Type,
					strconv.FormatBool(r.Enabled),
					ruleTargets(r),
					ruleTrigger(r),
				})
			}
			renderTable(os.Stdout, []string{"ID", "NAME", "TYPE", "ENABLED", "TARGETS", "TRIGGER"}, rows)
			return nil
		}),
	}
	cmd.AddCommand(
		newAlertRuleCreateCmd(a),
		newAlertRuleDeleteCmd(a),
		newAlertRuleToggleCmd(a, "enable", "Enable an alert rule", true),
		newAlertRuleToggleCmd(a, "disable", "Disable an alert rule", false),
	)
	return cmd
}

// ruleTargets summarizes what a rule applies to ("all" when unrestricted).
func ruleTargets(r alertRule) string {
	var parts []string
	if len(r.Hosts) > 0 {
		parts = append(parts, "hosts="+strings.Join(r.Hosts, ","))
	}
	if len(r.Containers) > 0 {
		parts = append(parts, "containers="+strings.Join(r.Containers, ","))
	}
	if len(r.Projects) > 0 {
		parts = append(parts, "projects="+strings.Join(r.Projects, ","))
	}
	if len(parts) == 0 {
		return "all"
	}
	return strings.Join(parts, " ")
}

// ruleTrigger summarizes what fires a rule ("die,oom 3x/120s",
// "level>=ERROR pattern=timeout").
func ruleTrigger(r alertRule) string {
	var parts []string
	if r.Type == "event" {
		if len(r.Events) > 0 {
			parts = append(parts, strings.Join(r.Events, ","))
		}
	} else {
		if r.MinLevel != "" {
			parts = append(parts, "level>="+r.MinLevel)
		}
		if r.Pattern != "" {
			parts = append(parts, "pattern="+r.Pattern)
		}
	}
	if r.Threshold > 0 {
		parts = append(parts, fmt.Sprintf("%dx/%ds", r.Threshold, r.WindowSeconds))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

// parseSecondsFlag converts a duration flag value ("60s", "5m", or bare
// seconds like "90") to whole seconds. Empty means unset (0).
func parseSecondsFlag(flag, value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	if n, err := strconv.Atoi(value); err == nil {
		if n < 0 {
			return 0, fmt.Errorf("--%s must be >= 0", flag)
		}
		return n, nil
	}
	d, err := parseDuration(value)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("invalid --%s duration %q (examples: 60s, 5m, 90)", flag, value)
	}
	return int(d / time.Second), nil
}

func newAlertRuleCreateCmd(a *app) *cobra.Command {
	var (
		ruleType, name, minLevel, pattern, window, cooldown string
		hosts, containers, projects, events                 []string
		threshold                                           int
		disabled                                            bool
		req                                                 alertRule
	)

	cmd := &cobra.Command{
		Use:   "create --type <log|event> --name <name>",
		Short: "Create an alert rule",
		Args:  cobra.NoArgs,
		// Flag validation runs in PreRunE so bad combinations are usage
		// errors (exit 2) and never reach the server.
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			switch ruleType {
			case "log":
				if len(events) > 0 {
					return fmt.Errorf("--events only applies to --type event")
				}
			case "event":
				if len(events) == 0 {
					return fmt.Errorf("--events is required for --type event (die, oom)")
				}
				for _, e := range events {
					if e != "die" && e != "oom" {
						return fmt.Errorf("invalid event %q (must be die or oom)", e)
					}
				}
				if minLevel != "" {
					return fmt.Errorf("--min-level only applies to --type log")
				}
				if pattern != "" {
					return fmt.Errorf("--pattern only applies to --type log")
				}
			case "":
				return fmt.Errorf("--type is required (log or event)")
			default:
				return fmt.Errorf("invalid --type %q (must be log or event)", ruleType)
			}

			req = alertRule{
				Name:       name,
				Enabled:    !disabled,
				Type:       ruleType,
				Hosts:      hosts,
				Containers: containers,
				Projects:   projects,
				Events:     events,
				MinLevel:   minLevel,
				Pattern:    pattern,
			}
			if cmd.Flags().Changed("threshold") {
				if threshold < 1 {
					return fmt.Errorf("--threshold must be >= 1")
				}
				req.Threshold = threshold
			}
			var err error
			if req.WindowSeconds, err = parseSecondsFlag("window", window); err != nil {
				return err
			}
			if req.CooldownSeconds, err = parseSecondsFlag("cooldown", cooldown); err != nil {
				return err
			}
			return nil
		},
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			var created map[string]any
			if err := a.client.post(cmd.Context(), "/alerts/rules", nil, req, &created); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(created)
			}
			id, _ := created["id"].(string)
			fmt.Printf("created rule %q (%s)\n", name, id)
			return nil
		}),
	}

	cmd.Flags().StringVar(&ruleType, "type", "", "rule type: log or event")
	cmd.Flags().StringVar(&name, "name", "", "rule name")
	cmd.Flags().StringSliceVar(&hosts, "host", nil, "limit to these hosts (repeatable)")
	cmd.Flags().StringSliceVar(&containers, "container", nil, "limit to these container names (repeatable)")
	cmd.Flags().StringSliceVar(&projects, "project", nil, "limit to these compose projects (repeatable)")
	cmd.Flags().StringSliceVar(&events, "events", nil, "events to match: die, oom (only with --type event)")
	cmd.Flags().StringVar(&minLevel, "min-level", "", "minimum log level to match, e.g. ERROR (only with --type log)")
	cmd.Flags().StringVar(&pattern, "pattern", "", "regex the log message must match (only with --type log)")
	cmd.Flags().IntVar(&threshold, "threshold", 0, "fire only after this many matches within --window")
	cmd.Flags().StringVar(&window, "window", "", "threshold window (e.g. 60s, 5m, or bare seconds)")
	cmd.Flags().StringVar(&cooldown, "cooldown", "", "minimum time between deliveries (e.g. 5m)")
	cmd.Flags().BoolVar(&disabled, "disabled", false, "create the rule disabled")
	return cmd
}

func newAlertRuleDeleteCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an alert rule",
		Args:  cobra.ExactArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			if err := a.client.do(cmd.Context(), http.MethodDelete, "/alerts/rules/"+args[0], nil, nil, nil); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]string{"message": "rule deleted", "id": args[0]})
			}
			fmt.Printf("deleted rule %q\n", args[0])
			return nil
		}),
	}
}

func newAlertRuleToggleCmd(a *app, verb, short string, enabled bool) *cobra.Command {
	return &cobra.Command{
		Use:   verb + " <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// The API has no single-rule GET: fetch the list, flip the flag,
			// and PUT the full rule back. Maps keep fields the CLI doesn't
			// model from being dropped.
			var resp struct {
				Rules []map[string]any `json:"rules"`
			}
			if err := a.client.get(ctx, "/alerts/rules", nil, &resp); err != nil {
				return err
			}
			for _, rule := range resp.Rules {
				id, _ := rule["id"].(string)
				if id != args[0] {
					continue
				}
				rule["enabled"] = enabled
				if err := a.client.put(ctx, "/alerts/rules/"+id, nil, rule, nil); err != nil {
					return err
				}

				if a.jsonOutput() {
					return a.printJSON(map[string]string{"message": "rule " + verb + "d", "id": id})
				}
				fmt.Printf("%sd rule %q\n", verb, id)
				return nil
			}
			return fmt.Errorf("no rule with id %q (see: logdeck alerts rules)", args[0])
		}),
	}
}

func newAlertHistoryCmd(a *app) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "history",
		Short: "List recently fired alerts, newest first",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			var resp struct {
				Alerts json.RawMessage `json:"alerts"`
				Count  int             `json:"count"`
			}
			query := url.Values{"limit": {strconv.Itoa(limit)}}
			if err := a.client.get(cmd.Context(), "/alerts/history", query, &resp); err != nil {
				return err
			}

			if a.jsonOutput() {
				var alerts any
				if len(resp.Alerts) > 0 {
					if err := json.Unmarshal(resp.Alerts, &alerts); err != nil {
						return err
					}
				}
				if alerts == nil {
					alerts = []any{}
				}
				return a.printJSON(map[string]any{"alerts": alerts, "count": resp.Count})
			}

			var alerts []alertInfo
			if len(resp.Alerts) > 0 {
				if err := json.Unmarshal(resp.Alerts, &alerts); err != nil {
					return err
				}
			}
			sort.SliceStable(alerts, func(i, j int) bool { return alerts[i].FiredAt.After(alerts[j].FiredAt) })
			now := time.Now()
			rows := make([][]string, 0, len(alerts))
			for _, al := range alerts {
				rows = append(rows, []string{
					humanAge(al.FiredAt, now),
					al.RuleName,
					al.ContainerName + "@" + al.Host,
					al.Reason,
					strconv.FormatBool(al.Suppressed),
					deliverySummary(al.Delivery),
				})
			}
			renderTable(os.Stdout, []string{"TIME", "RULE", "CONTAINER@HOST", "REASON", "SUPPRESSED", "DELIVERY"}, rows)
			return nil
		}),
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "maximum number of alerts to return")
	cmd.AddCommand(newAlertHistoryClearCmd(a))
	return cmd
}

// deliverySummary renders a delivery result on one line ("ok (HTTP 200)",
// "failed (HTTP 500): timeout").
func deliverySummary(d alertDelivery) string {
	s := d.Status
	if s == "" {
		s = "-"
	}
	if d.HTTPStatus > 0 {
		s = fmt.Sprintf("%s (HTTP %d)", s, d.HTTPStatus)
	}
	if d.Error != "" {
		s += ": " + d.Error
	}
	return s
}

func newAlertHistoryClearCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Delete all fired-alert history",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			if err := a.client.do(cmd.Context(), http.MethodDelete, "/alerts/history", nil, nil, nil); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]string{"message": "alert history cleared"})
			}
			fmt.Println("cleared alert history")
			return nil
		}),
	}
}

func newAlertWebhookCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Show the alert webhook URL",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			var resp struct {
				URL string `json:"url"`
			}
			if err := a.client.get(cmd.Context(), "/alerts/webhook", nil, &resp); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]string{"url": resp.URL})
			}
			if resp.URL == "" {
				fmt.Println("(not set)")
			} else {
				fmt.Println(resp.URL)
			}
			return nil
		}),
	}
	cmd.AddCommand(newAlertWebhookSetCmd(a), newAlertWebhookClearCmd(a))
	return cmd
}

func newAlertWebhookSetCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "set <url>",
		Short: "Set the alert webhook URL",
		Args:  cobra.ExactArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			body := map[string]string{"url": args[0]}
			if err := a.client.put(cmd.Context(), "/alerts/webhook", nil, body, nil); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]string{"message": "webhook updated", "url": args[0]})
			}
			fmt.Printf("webhook set to %s\n", args[0])
			return nil
		}),
	}
}

func newAlertWebhookClearCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear the alert webhook URL",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			body := map[string]string{"url": ""}
			if err := a.client.put(cmd.Context(), "/alerts/webhook", nil, body, nil); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]string{"message": "webhook cleared"})
			}
			fmt.Println("webhook cleared")
			return nil
		}),
	}
}

func newAlertTestCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Send a test alert to the webhook",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			var result alertDelivery
			if err := a.client.post(cmd.Context(), "/alerts/test", nil, nil, &result); err != nil {
				return err
			}

			if a.jsonOutput() {
				if err := a.printJSON(result); err != nil {
					return err
				}
			} else {
				fmt.Println("delivery " + deliverySummary(result))
			}
			if result.Status != "ok" {
				msg := "webhook test failed"
				if result.Error != "" {
					msg += ": " + result.Error
				}
				return fmt.Errorf("%s", msg)
			}
			return nil
		}),
	}
}
