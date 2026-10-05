package cli

import (
	"fmt"
	"net/url"
	"sort"

	"github.com/AmoabaKelvin/logdeck/internal/coolify"
	"github.com/spf13/cobra"
)

type envResponse struct {
	Env             map[string]string `json:"env"`
	Source          string            `json:"source,omitempty"`
	MappingRequired bool              `json:"mapping_required,omitempty"`
	Variables       []coolify.EnvVar  `json:"variables,omitempty"`
}

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

			var resp envResponse
			query := url.Values{"host": {container.Host}}
			if err := a.client.get(ctx, "/containers/"+container.ID+"/env", query, &resp); err != nil {
				return err
			}

			if a.jsonOutput() {
				if resp.Env == nil {
					resp.Env = map[string]string{}
				}
				return a.printJSON(resp)
			}

			if resp.Source == "dokploy" {
				return fmt.Errorf("confirm the owning Dokploy deployment in the LogDeck environment panel before reading or editing its saved configuration")
			}
			if resp.Source == "coolify" {
				for _, variable := range resp.Variables {
					scope := "production"
					if variable.IsPreview {
						scope = "preview"
					}
					value := "<unknown>"
					if variable.Value != nil {
						value = *variable.Value
					}
					fmt.Printf("%s [%s]=%s\n", variable.Key, scope, value)
				}
				return nil
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
