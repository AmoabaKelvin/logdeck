package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newEventsCmd(a *app) *cobra.Command {
	var follow bool
	var forDuration string

	cmd := &cobra.Command{
		Use:   "events",
		Short: "Stream container lifecycle events",
		Long:  "Stream container lifecycle events from all hosts. Streams until\ninterrupted; use --for to read for a fixed duration and exit.",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if forDuration != "" {
				d, err := parseDuration(forDuration)
				if err != nil || d <= 0 {
					return fmt.Errorf("invalid --for duration %q (examples: 10s, 1m)", forDuration)
				}
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, d)
				defer cancel()
			}

			body, err := a.client.stream(ctx, "/events", nil)
			if err != nil {
				return err
			}
			defer body.Close()

			return a.scanNDJSON(ctx, body, func(line []byte) error {
				var event containerEvent
				if err := json.Unmarshal(line, &event); err != nil {
					return nil // skip malformed lines
				}
				timestamp := time.Unix(event.Timestamp, 0).UTC().Format(time.RFC3339)
				if a.jsonOutput() {
					payload, err := json.Marshal(map[string]string{
						"timestamp":     timestamp,
						"host":          event.Host,
						"containerId":   event.ContainerID,
						"containerName": event.ContainerName,
						"action":        event.Action,
					})
					if err != nil {
						return err
					}
					fmt.Println(string(payload))
					return nil
				}
				fmt.Printf("%s %s %s %s\n", timestamp, event.Host, event.ContainerName, event.Action)
				return nil
			})
		}),
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "stream until interrupted (the default)")
	cmd.Flags().StringVar(&forDuration, "for", "", "read events for this long, then exit (e.g. 10s)")
	return cmd
}
