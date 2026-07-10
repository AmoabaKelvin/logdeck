package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// newActionCmd builds start/stop/restart/rm: resolve the container, then POST
// the matching lifecycle endpoint.
func newActionCmd(a *app, name, endpoint, pastTense string) *cobra.Command {
	var host string

	verb := strings.ToUpper(endpoint[:1]) + endpoint[1:]
	cmd := &cobra.Command{
		Use:   name + " <name|id>",
		Short: verb + " a container",
		Args:  cobra.ExactArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			container, err := a.resolve(ctx, args[0], host)
			if err != nil {
				return err
			}

			query := url.Values{"host": {container.Host}}
			if err := a.client.post(ctx, "/containers/"+container.ID+"/"+endpoint, query, nil, nil); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]string{
					"message": "Container " + pastTense,
					"name":    containerName(container),
					"id":      container.ID,
					"host":    container.Host,
				})
			}
			fmt.Printf("%s %s (host %s)\n", pastTense, containerName(container), container.Host)
			return nil
		}),
	}

	cmd.Flags().StringVar(&host, "host", "", "host name (disambiguates duplicate container names)")
	return cmd
}

var validStackActions = map[string]bool{"start": true, "stop": true, "restart": true}

func newStackCmd(a *app) *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:   "stack <start|stop|restart> <project>",
		Short: "Start, stop, or restart a whole compose project",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("expected an action and a project (e.g. logdeck stack restart myapp)")
			}
			if !validStackActions[args[0]] {
				return fmt.Errorf("invalid action %q: must be one of start, stop, restart", args[0])
			}
			return nil
		},
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			action, project := args[0], args[1]

			hosts := []string{host}
			if host == "" {
				resp, err := a.fetchContainers(ctx)
				if err != nil {
					return err
				}
				seen := map[string]bool{}
				hosts = nil
				for _, c := range resp.Containers {
					if composeProject(c.Labels) == project && !seen[c.Host] {
						seen[c.Host] = true
						hosts = append(hosts, c.Host)
					}
				}
				if len(hosts) == 0 {
					return fmt.Errorf("no containers found for stack %q", project)
				}
			}

			var results []composeResult
			var failures int
			for _, h := range hosts {
				result, err := a.composeAction(ctx, project, action, h)
				if err != nil {
					return err
				}
				results = append(results, result)
				failures += len(result.Failed)
			}

			if a.jsonOutput() {
				if err := a.printJSON(map[string]any{"results": results}); err != nil {
					return err
				}
			} else {
				for _, r := range results {
					fmt.Printf("host %s: %d/%d succeeded\n", r.Host, r.Succeeded, r.Total)
					for _, f := range r.Failed {
						fmt.Fprintf(os.Stderr, "  failed: %s: %s\n", f.Name, f.Error)
					}
				}
			}

			if failures > 0 {
				return fmt.Errorf("%d container(s) failed to %s", failures, action)
			}
			return nil
		}),
	}

	cmd.Flags().StringVar(&host, "host", "", "apply only on this host (default: every host that has the project)")
	return cmd
}

// composeAction posts a compose project action. The server returns the result
// body with HTTP 500 when some containers fail, so decode it in both cases.
func (a *app) composeAction(ctx context.Context, project, action, host string) (composeResult, error) {
	status, body, err := a.client.postRaw(ctx, "/compose/"+project+"/"+action, url.Values{"host": {host}})
	if err != nil {
		return composeResult{}, err
	}

	var result composeResult
	if decodeErr := json.Unmarshal(body, &result); decodeErr == nil && result.Project != "" {
		return result, nil
	}

	message := strings.TrimSpace(string(body))
	if message == "" {
		message = fmt.Sprintf("HTTP %d", status)
	}
	if status == 401 {
		message += " (" + authHint + ")"
	}
	return composeResult{}, fmt.Errorf("host %s: %s", host, message)
}
