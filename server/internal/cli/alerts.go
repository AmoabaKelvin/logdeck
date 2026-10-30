package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
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
	Metric          string   `json:"metric,omitempty"`
	AbovePercent    float64  `json:"abovePercent,omitempty"`
	AboveBytes      int64    `json:"aboveBytes,omitempty"`
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
	Suppressed    int           `json:"suppressed"`
	FiredAt       time.Time     `json:"firedAt"`
	Delivery      alertDelivery `json:"delivery"`
}

// alertChannel doubles as the create request body: omitempty keeps fields the
// user didn't set out of the payload, so the server applies its defaults.
type alertChannel struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Enabled  bool   `json:"enabled"`
	URL      string `json:"url,omitempty"`
	Token    string `json:"token,omitempty"`
	Target   string `json:"target,omitempty"`
	Username string `json:"username,omitempty"`
	From     string `json:"from,omitempty"`
}

func newAlertsCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alerts",
		Short: "Manage alert rules, notification channels, silences, and fired-alert history",
	}
	cmd.AddCommand(
		newAlertRulesCmd(a),
		newAlertHistoryCmd(a),
		newAlertChannelsCmd(a),
		newAlertSilencesCmd(a),
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

// ruleTrigger summarizes what fires a rule and how often it may deliver
// ("die,oom 3x/120s cooldown 60s", "level>=ERROR cooldown 300s (default)").
func ruleTrigger(r alertRule) string {
	var parts []string
	switch r.Type {
	case "event":
		if len(r.Events) > 0 {
			parts = append(parts, strings.Join(r.Events, ","))
		}
	case "collection":
		parts = append(parts, fmt.Sprintf("failing %ds", r.WindowSeconds))
	case "resource":
		above := fmt.Sprintf("%g%%", r.AbovePercent)
		if r.AboveBytes > 0 {
			above = humanBytes(uint64(r.AboveBytes))
		}
		parts = append(parts, fmt.Sprintf("%s>%s for %ds", r.Metric, above, r.WindowSeconds))
	default:
		if r.MinLevel != "" {
			parts = append(parts, "level>="+r.MinLevel)
		}
		if r.Pattern != "" {
			parts = append(parts, "pattern="+r.Pattern)
		}
	}
	if r.Threshold > 0 && r.Type != "collection" && r.Type != "resource" {
		parts = append(parts, fmt.Sprintf("%dx/%ds", r.Threshold, r.WindowSeconds))
	}
	if r.CooldownSeconds > 0 {
		parts = append(parts, fmt.Sprintf("cooldown %ds", r.CooldownSeconds))
	} else {
		// The server treats 0/absent as its 300s default.
		parts = append(parts, "cooldown 300s (default)")
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
		ruleType, name, minLevel, pattern, window, cooldown, metric, above string
		hosts, containers, projects, events                                []string
		threshold                                                          int
		disabled                                                           bool
		req                                                                alertRule
	)

	cmd := &cobra.Command{
		Use:   "create --type <log|event|collection|resource> --name <name>",
		Short: "Create an alert rule",
		Args:  cobra.NoArgs,
		// Flag validation runs in PreRunE so bad combinations are usage
		// errors (exit 2) and never reach the server.
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if ruleType != "resource" && (metric != "" || above != "") {
				return fmt.Errorf("--metric and --above only apply to --type resource")
			}
			switch ruleType {
			case "log":
				if len(events) > 0 {
					return fmt.Errorf("--events only applies to --type event")
				}
			case "event":
				if len(events) == 0 {
					return fmt.Errorf("--events is required for --type event (die, oom, unhealthy, restart)")
				}
				for _, e := range events {
					if e != "die" && e != "oom" && e != "unhealthy" && e != "restart" {
						return fmt.Errorf("invalid event %q (must be die, oom, unhealthy, or restart)", e)
					}
				}
				if len(events) > 1 && slices.Contains(events, "restart") {
					return fmt.Errorf("--events restart can't be combined with other events")
				}
				if minLevel != "" {
					return fmt.Errorf("--min-level only applies to --type log")
				}
				if pattern != "" {
					return fmt.Errorf("--pattern only applies to --type log")
				}
			case "collection":
				// --window is how long a failure lasts before it alerts.
				if len(events) > 0 || minLevel != "" || pattern != "" || len(containers) > 0 || len(projects) > 0 {
					return fmt.Errorf("--type collection takes only --host, --window and --cooldown")
				}
			case "resource":
				// --window is how long usage stays above --above before it alerts.
				if len(events) > 0 || minLevel != "" || pattern != "" {
					return fmt.Errorf("--type resource takes --metric and --above, not --events, --min-level or --pattern")
				}
				if metric != "cpu" && metric != "memory" {
					return fmt.Errorf("--metric is required for --type resource (cpu or memory)")
				}
				if above == "" {
					return fmt.Errorf("--above is required for --type resource (e.g. 90%%, 1.5g)")
				}
				if strings.HasSuffix(above, "%") {
					percent, err := strconv.ParseFloat(strings.TrimSuffix(above, "%"), 64)
					if err != nil || percent <= 0 {
						return fmt.Errorf("invalid --above %q (examples: 90%%, 512m, 1.5g)", above)
					}
					req.AbovePercent = percent
				} else {
					if metric == "cpu" {
						return fmt.Errorf("--above for cpu is a percentage (100%% is one core)")
					}
					bytes, err := parseMemory(above)
					if err != nil || bytes <= 0 {
						return fmt.Errorf("invalid --above %q (examples: 90%%, 512m, 1.5g)", above)
					}
					req.AboveBytes = bytes
				}
			case "":
				return fmt.Errorf("--type is required (log, event, collection, or resource)")
			default:
				return fmt.Errorf("invalid --type %q (must be log, event, collection, or resource)", ruleType)
			}

			req.Name = name
			req.Enabled = !disabled
			req.Type = ruleType
			req.Hosts = hosts
			req.Containers = containers
			req.Projects = projects
			req.Events = events
			req.MinLevel = minLevel
			req.Pattern = pattern
			req.Metric = metric
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

	cmd.Flags().StringVar(&ruleType, "type", "", "rule type: log, event, collection, or resource")
	cmd.Flags().StringVar(&name, "name", "", "rule name")
	cmd.Flags().StringSliceVar(&hosts, "host", nil, "limit to these hosts (repeatable)")
	cmd.Flags().StringSliceVar(&containers, "container", nil, "limit to these container names (repeatable)")
	cmd.Flags().StringSliceVar(&projects, "project", nil, "limit to these compose projects (repeatable)")
	cmd.Flags().StringSliceVar(&events, "events", nil, "events to match: die, oom, unhealthy, or restart on its own (only with --type event)")
	cmd.Flags().StringVar(&minLevel, "min-level", "", "minimum log level to match, e.g. ERROR (only with --type log)")
	cmd.Flags().StringVar(&pattern, "pattern", "", "regex the log message must match (only with --type log)")
	cmd.Flags().StringVar(&metric, "metric", "", "usage to watch: cpu or memory (only with --type resource)")
	cmd.Flags().StringVar(&above, "above", "", "limit usage must stay over: a percentage (90%; for cpu 100% is one core) or for memory a size (512m, 1.5g)")
	cmd.Flags().IntVar(&threshold, "threshold", 0, "fire only after this many matches within --window")
	cmd.Flags().StringVar(&window, "window", "", "threshold window, or for collection and resource how long the condition lasts first (e.g. 60s, 5m, or bare seconds)")
	cmd.Flags().StringVar(&cooldown, "cooldown", "", "minimum time between deliveries (e.g. 5m); 0 or omitted uses the server default of 300s")
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
					strconv.Itoa(al.Suppressed),
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

func newAlertChannelsCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "Manage notification channels (webhook, ntfy, gotify, telegram, slack, discord, email, pagerduty, opsgenie)",
	}
	cmd.AddCommand(
		newAlertChannelsListCmd(a),
		newAlertChannelAddCmd(a),
		newAlertChannelTestCmd(a),
		newAlertChannelDeleteCmd(a),
	)
	return cmd
}

func channelDest(c alertChannel) string {
	if c.Type == "telegram" {
		if c.Target == "" {
			return "-"
		}
		return "chat " + c.Target
	}
	if c.Type == "email" && c.Target != "" {
		return c.Target
	}
	if c.URL == "" {
		return "-"
	}
	return c.URL
}

func newAlertChannelsListCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List notification channels",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			var resp struct {
				Channels json.RawMessage `json:"channels"`
			}
			if err := a.client.get(cmd.Context(), "/alerts/channels", nil, &resp); err != nil {
				return err
			}

			if a.jsonOutput() {
				var channels any
				if len(resp.Channels) > 0 {
					if err := json.Unmarshal(resp.Channels, &channels); err != nil {
						return err
					}
				}
				if channels == nil {
					channels = []any{}
				}
				return a.printJSON(map[string]any{"channels": channels})
			}

			var channels []alertChannel
			if len(resp.Channels) > 0 {
				if err := json.Unmarshal(resp.Channels, &channels); err != nil {
					return err
				}
			}
			rows := make([][]string, 0, len(channels))
			for _, c := range channels {
				rows = append(rows, []string{
					c.ID,
					c.Type,
					c.Name,
					strconv.FormatBool(c.Enabled),
					channelDest(c),
				})
			}
			renderTable(os.Stdout, []string{"ID", "TYPE", "NAME", "ENABLED", "DESTINATION"}, rows)
			return nil
		}),
	}
}

func newAlertChannelAddCmd(a *app) *cobra.Command {
	var (
		channelType, name, channelURL, token, target, username, from string
		disabled                                                     bool
		req                                                          alertChannel
	)

	cmd := &cobra.Command{
		Use:   "add --type <webhook|ntfy|gotify|telegram|slack|discord|email|pagerduty|opsgenie>",
		Short: "Add a notification channel",
		Args:  cobra.NoArgs,
		// Flag validation runs in PreRunE so bad combinations are usage errors
		// (exit 2) and never reach the server.
		PreRunE: func(cmd *cobra.Command, args []string) error {
			switch channelType {
			case "webhook", "ntfy", "slack", "discord":
				if channelURL == "" {
					return fmt.Errorf("--endpoint is required for --type %s", channelType)
				}
			case "gotify":
				if channelURL == "" {
					return fmt.Errorf("--endpoint is required for --type gotify (the server base URL)")
				}
				if token == "" {
					return fmt.Errorf("--secret is required for --type gotify (the app token)")
				}
			case "telegram":
				if token == "" {
					return fmt.Errorf("--secret is required for --type telegram (the bot token)")
				}
				if target == "" {
					return fmt.Errorf("--target is required for --type telegram (the chat id)")
				}
			case "email":
				if channelURL == "" {
					return fmt.Errorf("--endpoint is required for --type email (smtp://host:587 or smtps://host:465)")
				}
				if from == "" {
					return fmt.Errorf("--from is required for --type email")
				}
				if target == "" {
					return fmt.Errorf("--target is required for --type email (the recipients)")
				}
			case "pagerduty":
				if token == "" {
					return fmt.Errorf("--secret is required for --type pagerduty (the routing key)")
				}
			case "opsgenie":
				if token == "" {
					return fmt.Errorf("--secret is required for --type opsgenie (the API key)")
				}
			case "":
				return fmt.Errorf("--type is required (webhook, ntfy, gotify, telegram, slack, discord, email, pagerduty, or opsgenie)")
			default:
				return fmt.Errorf("invalid --type %q (must be webhook, ntfy, gotify, telegram, slack, discord, email, pagerduty, or opsgenie)", channelType)
			}

			req = alertChannel{
				Type:     channelType,
				Name:     name,
				Enabled:  !disabled,
				URL:      channelURL,
				Token:    token,
				Target:   target,
				Username: username,
				From:     from,
			}
			return nil
		},
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			var created map[string]any
			if err := a.client.post(cmd.Context(), "/alerts/channels", nil, req, &created); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(created)
			}
			id, _ := created["id"].(string)
			fmt.Printf("added %s channel (%s)\n", channelType, id)
			return nil
		}),
	}

	cmd.Flags().StringVar(&channelType, "type", "", "channel type: webhook, ntfy, gotify, telegram, slack, discord, email, pagerduty, or opsgenie")
	cmd.Flags().StringVar(&name, "name", "", "display name for the channel")
	cmd.Flags().StringVar(&channelURL, "endpoint", "", "webhook/ntfy/slack/discord: full destination URL; gotify: server base URL; email: smtp:// or smtps:// server URL; pagerduty/opsgenie: optional API URL for an EU account")
	cmd.Flags().StringVar(&token, "secret", "", "gotify app token, telegram bot token, email SMTP password, pagerduty routing key, or opsgenie API key")
	cmd.Flags().StringVar(&target, "target", "", "telegram chat id, or email recipients (comma-separated)")
	cmd.Flags().StringVar(&username, "username", "", "email: SMTP username")
	cmd.Flags().StringVar(&from, "from", "", "email: sender address")
	cmd.Flags().BoolVar(&disabled, "disabled", false, "add the channel disabled")
	return cmd
}

func newAlertChannelTestCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "test <id>",
		Short: "Send a test alert to a channel",
		Args:  cobra.ExactArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			var result alertDelivery
			if err := a.client.post(cmd.Context(), "/alerts/channels/"+args[0]+"/test", nil, nil, &result); err != nil {
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
				msg := "channel test failed"
				if result.Error != "" {
					msg += ": " + result.Error
				}
				return fmt.Errorf("%s", msg)
			}
			return nil
		}),
	}
}

func newAlertChannelDeleteCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a notification channel",
		Args:  cobra.ExactArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			if err := a.client.do(cmd.Context(), http.MethodDelete, "/alerts/channels/"+args[0], nil, nil, nil); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]string{"message": "channel deleted", "id": args[0]})
			}
			fmt.Printf("deleted channel %q\n", args[0])
			return nil
		}),
	}
}
