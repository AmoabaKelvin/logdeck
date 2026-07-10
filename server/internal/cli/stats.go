package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newStatsCmd(a *app) *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:   "stats [<name|id>]",
		Short: "Show CPU and memory usage for running containers",
		Args:  cobra.MaximumNArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			containers, err := a.fetchContainers(ctx)
			if err != nil {
				return err
			}

			var statsResp struct {
				Stats []containerStats `json:"stats"`
			}
			if err := a.client.get(ctx, "/containers/stats", nil, &statsResp); err != nil {
				return err
			}

			names := map[string]string{}
			for _, c := range containers.Containers {
				names[c.Host+"/"+c.ID] = containerName(c)
			}

			stats := statsResp.Stats
			if len(args) == 1 {
				container, err := resolveContainer(containers.Containers, args[0], host)
				if err != nil {
					return err
				}
				stats = nil
				for _, s := range statsResp.Stats {
					if s.ID == container.ID && s.Host == container.Host {
						stats = append(stats, s)
					}
				}
				if len(stats) == 0 {
					return fmt.Errorf("no stats for container %q (is it running?)", containerName(container))
				}
			}

			if a.jsonOutput() {
				if stats == nil {
					stats = []containerStats{}
				}
				return a.printJSON(map[string]any{"stats": stats})
			}

			rows := make([][]string, 0, len(stats))
			for _, s := range stats {
				name := names[s.Host+"/"+s.ID]
				if name == "" {
					name = shortID(s.ID)
				}
				rows = append(rows, []string{
					name,
					s.Host,
					fmt.Sprintf("%.1f%%", s.CPUPercent),
					fmt.Sprintf("%.1f%%", s.MemoryPercent),
					humanBytes(s.MemoryUsed),
					humanBytes(s.MemoryLimit),
				})
			}
			renderTable(os.Stdout, []string{"NAME", "HOST", "CPU%", "MEM%", "MEM USED", "MEM LIMIT"}, rows)
			return nil
		}),
	}

	cmd.Flags().StringVar(&host, "host", "", "host name (disambiguates duplicate container names)")
	return cmd
}
