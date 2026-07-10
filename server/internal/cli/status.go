package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

func newStatusCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show server health, version, and per-host summary",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var health struct {
				Status string `json:"status"`
			}
			if err := a.client.get(ctx, "/healthz", nil, &health); err != nil {
				return err
			}

			var version struct {
				Version string `json:"version"`
			}
			if err := a.client.get(ctx, "/version", nil, &version); err != nil {
				return err
			}

			var hosts struct {
				Hosts []hostInfo `json:"hosts"`
			}
			if err := a.client.get(ctx, "/hosts/stats", nil, &hosts); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]any{
					"url":         a.client.baseURL,
					"urlSource":   a.conn.urlSource,
					"tokenSource": a.conn.tokenSource,
					"status":      health.Status,
					"version":     version.Version,
					"hosts":       hosts.Hosts,
				})
			}

			source := "url from " + a.conn.urlSource
			if a.conn.tokenSource == "none" {
				source += ", no token"
			} else {
				source += ", token from " + a.conn.tokenSource
			}

			fmt.Printf("Server:  %s\n", a.client.baseURL)
			fmt.Printf("Source:  %s\n", source)
			fmt.Printf("Status:  %s\n", health.Status)
			fmt.Printf("Version: %s\n\n", version.Version)

			rows := make([][]string, 0, len(hosts.Hosts))
			for _, h := range hosts.Hosts {
				name := h.Name
				if name == "" {
					name = h.Host
				}
				if !h.Available {
					rows = append(rows, []string{name, "no", "-", "-", "-", "-", h.Error})
					continue
				}
				rows = append(rows, []string{
					name,
					"yes",
					fmt.Sprintf("%d up / %d down", h.ContainersRunning, h.ContainersStopped),
					strconv.Itoa(h.NCPU),
					humanBytes(uint64(h.MemTotal)),
					h.ServerVersion,
					"",
				})
			}
			renderTable(os.Stdout, []string{"HOST", "AVAILABLE", "CONTAINERS", "CPUS", "MEMORY", "ENGINE", "ERROR"}, rows)
			return nil
		}),
	}
}
