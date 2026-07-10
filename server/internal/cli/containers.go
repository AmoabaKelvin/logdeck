package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

func newContainersCmd(a *app) *cobra.Command {
	var state, host string

	cmd := &cobra.Command{
		Use:   "containers",
		Short: "List containers across all hosts",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			resp, err := a.fetchContainers(cmd.Context())
			if err != nil {
				return err
			}

			filtered := make([]containerInfo, 0, len(resp.Containers))
			for _, c := range resp.Containers {
				if state != "" && c.State != state {
					continue
				}
				if host != "" && c.Host != host {
					continue
				}
				filtered = append(filtered, c)
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]any{
					"containers": filtered,
					"hostErrors": resp.HostErrors,
					"readOnly":   resp.ReadOnly,
				})
			}

			warnHostErrors(resp.HostErrors)
			rows := make([][]string, 0, len(filtered))
			for _, c := range filtered {
				rows = append(rows, []string{containerName(c), c.State, c.Image, c.Host, c.Status})
			}
			renderTable(os.Stdout, []string{"NAME", "STATE", "IMAGE", "HOST", "UPTIME"}, rows)
			return nil
		}),
	}

	cmd.Flags().StringVar(&state, "state", "", "filter by state (running, exited, paused, restarting, dead)")
	cmd.Flags().StringVar(&host, "host", "", "filter by host name")
	return cmd
}

// warnHostErrors reports unreachable hosts on stderr in table mode.
func warnHostErrors(errors []hostError) {
	for _, he := range errors {
		fmt.Fprintf(os.Stderr, "warning: host %s unreachable: %s\n", he.Host, he.Message)
	}
}

func newInspectCmd(a *app) *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:   "inspect <name|id>",
		Short: "Show full inspect data for a container",
		Args:  cobra.ExactArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			container, err := a.resolve(ctx, args[0], host)
			if err != nil {
				return err
			}

			var resp struct {
				Container json.RawMessage `json:"container"`
			}
			query := url.Values{"host": {container.Host}}
			if err := a.client.get(ctx, "/containers/"+container.ID+"/", query, &resp); err != nil {
				return err
			}

			if a.jsonOutput() {
				var inspect any
				if err := json.Unmarshal(resp.Container, &inspect); err != nil {
					return err
				}
				return a.printJSON(inspect)
			}

			var inspect map[string]any
			if err := json.Unmarshal(resp.Container, &inspect); err != nil {
				return err
			}
			printInspectSummary(container, inspect)
			return nil
		}),
	}

	cmd.Flags().StringVar(&host, "host", "", "host name (disambiguates duplicate container names)")
	return cmd
}

// dig walks nested maps, returning "" if any key is missing.
func dig(m map[string]any, keys ...string) string {
	current := any(m)
	for _, key := range keys {
		asMap, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = asMap[key]
	}
	switch v := current.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%v", v)
	default:
		return ""
	}
}

func printInspectSummary(container containerInfo, inspect map[string]any) {
	rows := [][]string{
		{"Name", containerName(container)},
		{"ID", container.ID},
		{"Host", container.Host},
		{"Image", dig(inspect, "Config", "Image")},
		{"State", dig(inspect, "State", "Status")},
		{"Started", dig(inspect, "State", "StartedAt")},
		{"Exit code", dig(inspect, "State", "ExitCode")},
		{"Restarts", dig(inspect, "RestartCount")},
		{"Restart policy", dig(inspect, "HostConfig", "RestartPolicy", "Name")},
		{"Created", dig(inspect, "Created")},
	}
	if project := composeProject(container.Labels); project != "" {
		rows = append(rows, []string{"Compose project", project})
	}
	if cmd := dig(inspect, "Path"); cmd != "" {
		rows = append(rows, []string{"Command", cmd})
	}
	renderTable(os.Stdout, []string{"FIELD", "VALUE"}, rows)
	fmt.Fprintln(os.Stderr, "\nhint: use -o json for the full inspect document")
}
