package cli

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newGrepCmd(a *app) *cobra.Command {
	var since, level, host string
	var where []string
	var tail int

	cmd := &cobra.Command{
		Use:   "grep [<regex>]",
		Short: "Search recent logs of all running containers",
		Long:  "Search the recent logs of every running container across all hosts,\nmerged by timestamp. Bounded to the last 15 minutes by default (--since).",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("accepts at most one regex")
			}
			if len(args) == 0 && len(where) == 0 {
				return fmt.Errorf("specify a regex, --where, or both")
			}
			return nil
		},
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var pattern string
			if len(args) == 1 {
				pattern = args[0]
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("invalid regex %q: %v", pattern, err)
			}

			sinceValue, err := parseTimeArg(since, time.Now())
			if err != nil {
				return err
			}

			resp, err := a.fetchContainers(ctx)
			if err != nil {
				return err
			}

			var running []containerInfo
			for _, c := range resp.Containers {
				if c.State != "running" {
					continue
				}
				if host != "" && c.Host != host {
					continue
				}
				running = append(running, c)
			}
			if len(running) == 0 {
				fmt.Fprintln(os.Stderr, "no running containers to search")
				return a.printAggregatedLogs(nil, nil)
			}

			query := url.Values{}
			if pattern != "" {
				query.Set("search", pattern)
			}
			for _, w := range where {
				query.Add("where", w)
			}
			query.Set("tail", strconv.Itoa(tail))
			if sinceValue != "" {
				query.Set("since", sinceValue)
			}
			if level != "" {
				query.Set("level", strings.ToUpper(level))
			}

			logs, sources, err := a.aggregatedLogs(ctx, buildTargets(running), query)
			if err != nil {
				return err
			}
			if len(logs) == 0 {
				// Containers that could not be read were not searched.
				searched := len(running) - len(missingSources(sources))
				fmt.Fprintf(os.Stderr, "no matches in %d containers since %s\n", searched, since)
			}
			return a.printAggregatedLogs(logs, sources)
		}),
	}

	cmd.Flags().StringVar(&since, "since", "15m", "only logs after this time (RFC3339 or relative: 30s, 15m, 2h, 1d)")
	cmd.Flags().StringVar(&level, "level", "", "filter by log level (TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC)")
	cmd.Flags().StringVar(&host, "host", "", "only search containers on this host")
	cmd.Flags().StringArrayVar(&where, "where", nil, whereUsage)
	cmd.Flags().IntVar(&tail, "tail", 1000, "lines scanned per container (max 10000)")
	return cmd
}
