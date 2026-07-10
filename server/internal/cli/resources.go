package cli

import (
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

func newResourcesCmd(a *app) *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:   "resources <name|id>",
		Short: "Show a container's resource limits and restart policy",
		Args:  cobra.ExactArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			container, err := a.resolve(ctx, args[0], host)
			if err != nil {
				return err
			}

			var resources containerResources
			query := url.Values{"host": {container.Host}}
			if err := a.client.get(ctx, "/containers/"+container.ID+"/resources", query, &resources); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(resources)
			}

			memory := "unlimited"
			if resources.MemoryBytes > 0 {
				memory = humanBytes(uint64(resources.MemoryBytes))
			}
			cpus := "unlimited"
			if resources.NanoCPUs > 0 {
				cpus = fmt.Sprintf("%g", float64(resources.NanoCPUs)/1e9)
			}
			policy := resources.RestartPolicy.Name
			if policy == "" {
				policy = "no"
			}
			if resources.RestartPolicy.MaximumRetryCount > 0 {
				policy = fmt.Sprintf("%s (max retries %d)", policy, resources.RestartPolicy.MaximumRetryCount)
			}

			renderTable(os.Stdout, []string{"FIELD", "VALUE"}, [][]string{
				{"Memory limit", memory},
				{"CPU limit", cpus},
				{"Restart policy", policy},
			})
			return nil
		}),
	}

	cmd.Flags().StringVar(&host, "host", "", "host name (disambiguates duplicate container names)")
	cmd.AddCommand(newResourcesSetCmd(a))
	return cmd
}

func newResourcesSetCmd(a *app) *cobra.Command {
	var host, memory, restart string
	var cpus float64
	var maxRetries int

	cmd := &cobra.Command{
		Use:   "set <name|id>",
		Short: "Update a container's resource limits or restart policy",
		Args:  cobra.ExactArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var req updateResourcesRequest
			if cmd.Flags().Changed("memory") {
				bytes, err := parseMemory(memory)
				if err != nil {
					return err
				}
				req.MemoryBytes = &bytes
			}
			if cmd.Flags().Changed("cpus") {
				if cpus < 0 {
					return fmt.Errorf("--cpus must be >= 0")
				}
				nano := cpusToNano(cpus)
				req.NanoCPUs = &nano
			}
			if cmd.Flags().Changed("restart") || cmd.Flags().Changed("max-retries") {
				if restart == "" {
					return fmt.Errorf("--max-retries requires --restart on-failure")
				}
				req.RestartPolicy = &restartPolicySpec{Name: restart, MaximumRetryCount: maxRetries}
			}
			if req.MemoryBytes == nil && req.NanoCPUs == nil && req.RestartPolicy == nil {
				return fmt.Errorf("nothing to update: pass at least one of --memory, --cpus, --restart")
			}

			container, err := a.resolve(ctx, args[0], host)
			if err != nil {
				return err
			}

			query := url.Values{"host": {container.Host}}
			if err := a.client.put(ctx, "/containers/"+container.ID+"/resources", query, req, nil); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]string{
					"message": "Container resources updated",
					"name":    containerName(container),
					"id":      container.ID,
					"host":    container.Host,
				})
			}
			fmt.Printf("updated resources for %s (host %s)\n", containerName(container), container.Host)
			return nil
		}),
	}

	cmd.Flags().StringVar(&host, "host", "", "host name (disambiguates duplicate container names)")
	cmd.Flags().StringVar(&memory, "memory", "", "memory limit (e.g. 512m, 1.5g; 0 = unlimited)")
	cmd.Flags().Float64Var(&cpus, "cpus", 0, "CPU limit (e.g. 1.5; 0 = unlimited)")
	cmd.Flags().StringVar(&restart, "restart", "", "restart policy: no, always, unless-stopped, on-failure")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 0, "maximum retries (only with --restart on-failure)")
	return cmd
}
