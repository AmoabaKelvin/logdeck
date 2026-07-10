package cli

import (
	"fmt"
	"net/url"
	"sort"

	"github.com/spf13/cobra"
)

func newEnvCmd(a *app) *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:   "env <name|id>",
		Short: "Show a container's environment variables",
		Args:  cobra.ExactArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			container, err := a.resolve(ctx, args[0], host)
			if err != nil {
				return err
			}

			var resp struct {
				Env map[string]string `json:"env"`
			}
			query := url.Values{"host": {container.Host}}
			if err := a.client.get(ctx, "/containers/"+container.ID+"/env", query, &resp); err != nil {
				return err
			}

			if a.jsonOutput() {
				if resp.Env == nil {
					resp.Env = map[string]string{}
				}
				return a.printJSON(map[string]any{"env": resp.Env})
			}

			keys := make([]string, 0, len(resp.Env))
			for key := range resp.Env {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Printf("%s=%s\n", key, resp.Env[key])
			}
			return nil
		}),
	}

	cmd.Flags().StringVar(&host, "host", "", "host name (disambiguates duplicate container names)")
	return cmd
}
