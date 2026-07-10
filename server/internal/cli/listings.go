package cli

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newImagesCmd(a *app) *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:   "images",
		Short: "List images across all hosts",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			var resp struct {
				Images     []imageInfo `json:"images"`
				HostErrors []hostError `json:"hostErrors"`
			}
			if err := a.client.get(cmd.Context(), "/images", nil, &resp); err != nil {
				return err
			}

			filtered := make([]imageInfo, 0, len(resp.Images))
			for _, image := range resp.Images {
				if host != "" && image.Host != host {
					continue
				}
				filtered = append(filtered, image)
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]any{"images": filtered, "hostErrors": resp.HostErrors})
			}

			warnHostErrors(resp.HostErrors)
			now := time.Now()
			rows := make([][]string, 0, len(filtered))
			for _, image := range filtered {
				tag := "<none>"
				if len(image.RepoTags) > 0 {
					tag = image.RepoTags[0]
				}
				rows = append(rows, []string{
					tag,
					shortID(image.ID),
					humanBytes(uint64(image.Size)),
					humanAge(time.Unix(image.Created, 0), now),
					image.Host,
				})
			}
			renderTable(os.Stdout, []string{"TAG", "ID", "SIZE", "CREATED", "HOST"}, rows)
			return nil
		}),
	}

	cmd.Flags().StringVar(&host, "host", "", "filter by host name")
	return cmd
}

func newVolumesCmd(a *app) *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:   "volumes",
		Short: "List volumes across all hosts",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			var resp struct {
				Volumes    []volumeInfo `json:"volumes"`
				HostErrors []hostError  `json:"hostErrors"`
			}
			if err := a.client.get(cmd.Context(), "/volumes", nil, &resp); err != nil {
				return err
			}

			filtered := make([]volumeInfo, 0, len(resp.Volumes))
			for _, volume := range resp.Volumes {
				if host != "" && volume.Host != host {
					continue
				}
				filtered = append(filtered, volume)
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]any{"volumes": filtered, "hostErrors": resp.HostErrors})
			}

			warnHostErrors(resp.HostErrors)
			rows := make([][]string, 0, len(filtered))
			for _, volume := range filtered {
				rows = append(rows, []string{volume.Name, volume.Driver, volume.Created, volume.Host})
			}
			renderTable(os.Stdout, []string{"NAME", "DRIVER", "CREATED", "HOST"}, rows)
			return nil
		}),
	}

	cmd.Flags().StringVar(&host, "host", "", "filter by host name")
	return cmd
}

func newNetworksCmd(a *app) *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:   "networks",
		Short: "List networks across all hosts",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			var resp struct {
				Networks   []networkInfo `json:"networks"`
				HostErrors []hostError   `json:"hostErrors"`
			}
			if err := a.client.get(cmd.Context(), "/networks", nil, &resp); err != nil {
				return err
			}

			filtered := make([]networkInfo, 0, len(resp.Networks))
			for _, network := range resp.Networks {
				if host != "" && network.Host != host {
					continue
				}
				filtered = append(filtered, network)
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]any{"networks": filtered, "hostErrors": resp.HostErrors})
			}

			warnHostErrors(resp.HostErrors)
			rows := make([][]string, 0, len(filtered))
			for _, network := range filtered {
				rows = append(rows, []string{
					network.Name,
					network.Driver,
					network.Scope,
					strings.Join(network.Subnets, ","),
					network.Host,
				})
			}
			renderTable(os.Stdout, []string{"NAME", "DRIVER", "SCOPE", "SUBNETS", "HOST"}, rows)
			return nil
		}),
	}

	cmd.Flags().StringVar(&host, "host", "", "filter by host name")
	return cmd
}
