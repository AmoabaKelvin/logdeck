package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// maxAggregateTargets mirrors the server's per-request limit for
// /logs/aggregate.
const maxAggregateTargets = 20

type logFlags struct {
	tail   int
	level  string
	search string
	since  string
	until  string
	follow bool
}

func (f *logFlags) register(cmd *cobra.Command) {
	cmd.Flags().IntVar(&f.tail, "tail", 100, "number of lines from the end of the logs (max 10000)")
	cmd.Flags().StringVar(&f.level, "level", "", "filter by log level (TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC)")
	cmd.Flags().StringVar(&f.search, "search", "", "filter by regex")
	cmd.Flags().StringVar(&f.since, "since", "", "only logs after this time (RFC3339 or relative: 30s, 15m, 2h, 1d)")
	cmd.Flags().StringVar(&f.until, "until", "", "only logs before this time (RFC3339 or relative: 30s, 15m, 2h, 1d)")
	cmd.Flags().BoolVarP(&f.follow, "follow", "f", false, "stream new logs continuously")
}

// query converts flags to log endpoint query params, resolving relative times.
func (f *logFlags) query(now time.Time) (url.Values, error) {
	query := url.Values{}
	query.Set("tail", strconv.Itoa(f.tail))
	since, err := parseTimeArg(f.since, now)
	if err != nil {
		return nil, err
	}
	if since != "" {
		query.Set("since", since)
	}
	until, err := parseTimeArg(f.until, now)
	if err != nil {
		return nil, err
	}
	if until != "" {
		query.Set("until", until)
	}
	if f.level != "" {
		query.Set("level", strings.ToUpper(f.level))
	}
	if f.search != "" {
		query.Set("search", f.search)
	}
	return query, nil
}

func newLogsCmd(a *app) *cobra.Command {
	var flags logFlags
	var stack, host string

	cmd := &cobra.Command{
		Use:   "logs [<name|id>]",
		Short: "Read or follow container logs (or a whole stack with --stack)",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("accepts at most one container")
			}
			stackFlag, _ := cmd.Flags().GetString("stack")
			if (stackFlag != "") == (len(args) == 1) {
				return fmt.Errorf("specify either a container or --stack <project>")
			}
			return nil
		},
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			query, err := flags.query(time.Now())
			if err != nil {
				return err
			}

			if stack != "" {
				return a.stackLogs(ctx, stack, host, flags.follow, query)
			}

			container, err := a.resolve(ctx, args[0], host)
			if err != nil {
				return err
			}
			query.Set("host", container.Host)
			path := "/containers/" + container.ID + "/logs/parsed"

			if flags.follow {
				query.Set("follow", "true")
				return a.followLogs(ctx, path, query, false)
			}

			var resp struct {
				Logs  []logEntry `json:"logs"`
				Count int        `json:"count"`
			}
			if err := a.client.get(ctx, path, query, &resp); err != nil {
				return err
			}
			return a.printLogs(resp.Logs, false)
		}),
	}

	flags.register(cmd)
	cmd.Flags().StringVar(&stack, "stack", "", "read aggregated logs for a compose project instead of one container")
	cmd.Flags().StringVar(&host, "host", "", "host name (disambiguates, or narrows --stack to one host)")
	return cmd
}

// stackLogs reads aggregated logs for every container of a compose project.
func (a *app) stackLogs(ctx context.Context, project, host string, follow bool, query url.Values) error {
	resp, err := a.fetchContainers(ctx)
	if err != nil {
		return err
	}

	var members []containerInfo
	for _, c := range resp.Containers {
		if composeProject(c.Labels) != project {
			continue
		}
		if host != "" && c.Host != host {
			continue
		}
		members = append(members, c)
	}
	if len(members) == 0 {
		return fmt.Errorf("no containers found for stack %q", project)
	}

	targets := buildTargets(members)

	if follow {
		if len(targets) > maxAggregateTargets {
			fmt.Fprintf(os.Stderr, "warning: stack %s has %d containers; following only the first %d\n", project, len(targets), maxAggregateTargets)
			targets = targets[:maxAggregateTargets]
		}
		query.Set("targets", strings.Join(targets, ","))
		query.Set("follow", "true")
		return a.followLogs(ctx, "/logs/aggregate", query, true)
	}

	logs, err := a.aggregatedLogs(ctx, targets, query)
	if err != nil {
		return err
	}
	return a.printLogs(logs, true)
}

// buildTargets encodes containers as host~id~name aggregate targets.
func buildTargets(containers []containerInfo) []string {
	targets := make([]string, 0, len(containers))
	for _, c := range containers {
		targets = append(targets, c.Host+"~"+c.ID+"~"+containerName(c))
	}
	return targets
}

// aggregatedLogs fetches one-shot aggregate logs, batching targets by the
// server's per-request limit and merging results by timestamp.
func (a *app) aggregatedLogs(ctx context.Context, targets []string, query url.Values) ([]logEntry, error) {
	var all []logEntry
	for _, batch := range batchStrings(targets, maxAggregateTargets) {
		batchQuery := url.Values{}
		for key, values := range query {
			batchQuery[key] = values
		}
		batchQuery.Set("targets", strings.Join(batch, ","))

		var resp struct {
			Logs []logEntry `json:"logs"`
		}
		if err := a.client.get(ctx, "/logs/aggregate", batchQuery, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Logs...)
	}
	mergeByTimestamp(all)
	return all, nil
}

// mergeByTimestamp sorts entries chronologically (stable, so same-timestamp
// lines keep their per-container order).
func mergeByTimestamp(entries []logEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
}

// printLogs writes one-shot log results: a single JSON document in json mode,
// one formatted line per entry otherwise.
func (a *app) printLogs(logs []logEntry, withName bool) error {
	if a.jsonOutput() {
		if logs == nil {
			logs = []logEntry{}
		}
		return a.printJSON(map[string]any{"logs": logs, "count": len(logs)})
	}
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for _, entry := range logs {
		fmt.Fprintln(out, formatLogLine(entry, withName))
	}
	return nil
}

// followLogs streams an NDJSON log endpoint, skipping heartbeats. Table mode
// formats entries; json mode echoes the raw NDJSON lines.
func (a *app) followLogs(ctx context.Context, path string, query url.Values, withName bool) error {
	body, err := a.client.stream(ctx, path, query)
	if err != nil {
		return err
	}
	defer body.Close()

	return a.scanNDJSON(ctx, body, func(line []byte) error {
		if a.jsonOutput() {
			fmt.Println(string(line))
			return nil
		}
		var entry logEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil // skip malformed lines rather than aborting the stream
		}
		fmt.Println(formatLogLine(entry, withName))
		return nil
	})
}

// scanNDJSON reads an NDJSON stream line by line, skipping blank lines and
// heartbeats. Context cancellation (Ctrl-C, --for expiry) is a clean stop.
func (a *app) scanNDJSON(ctx context.Context, body interface{ Read([]byte) (int, error) }, handle func(line []byte) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || isHeartbeat(line) {
			continue
		}
		if err := handle(line); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

func isHeartbeat(line []byte) bool {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(line, &probe); err != nil {
		return false
	}
	return probe.Type == "heartbeat"
}
