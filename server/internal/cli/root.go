// Package cli implements the logdeck command-line client. It talks to a
// running LogDeck server over its HTTP API and is designed to be
// non-interactive, scriptable, and machine-readable (see the -o json flag).
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

type app struct {
	url    string
	token  string
	output string

	client *client

	// ran flips to true once a command's RunE body starts executing, so
	// Execute can tell usage errors (exit 2) from runtime errors (exit 1).
	ran bool
}

func (a *app) jsonOutput() bool { return a.output == "json" }

// run wraps a RunE body, marking that command execution started.
func (a *app) run(fn func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		a.ran = true
		return fn(cmd, args)
	}
}

// printJSON writes a single indented JSON document to stdout.
func (a *app) printJSON(v any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

func (a *app) printError(err error) {
	if a.jsonOutput() {
		payload, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintln(os.Stderr, string(payload))
		return
	}
	fmt.Fprintln(os.Stderr, "Error: "+err.Error())
}

// Execute runs the CLI and returns the process exit code:
// 0 success, 1 runtime error, 2 usage error.
func Execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := &app{}
	root := newRootCmd(a)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	if !a.ran {
		fmt.Fprintln(os.Stderr, "Error: "+err.Error())
		fmt.Fprintln(os.Stderr, "Run 'logdeck --help' for usage.")
		return 2
	}
	a.printError(err)
	return 1
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func newRootCmd(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:           "logdeck",
		Short:         "Command-line client for a running LogDeck server",
		Long:          "logdeck talks to a running LogDeck server over its HTTP API.\nIt is non-interactive and scriptable; use -o json for machine-readable output.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if a.output != "table" && a.output != "json" {
				return fmt.Errorf("invalid output format %q (must be table or json)", a.output)
			}
			a.client = newClient(strings.TrimRight(a.url, "/"), a.token)
			return nil
		},
	}

	root.PersistentFlags().StringVar(&a.url, "url", envOr("LOGDECK_URL", "http://localhost:8080"), "LogDeck server URL (env: LOGDECK_URL)")
	root.PersistentFlags().StringVar(&a.token, "token", os.Getenv("LOGDECK_TOKEN"), "API token sent as a Bearer token (env: LOGDECK_TOKEN)")
	root.PersistentFlags().StringVarP(&a.output, "output", "o", "table", "output format: table or json")

	root.AddCommand(
		newStatusCmd(a),
		newContainersCmd(a),
		newStacksCmd(a),
		newInspectCmd(a),
		newLogsCmd(a),
		newGrepCmd(a),
		newStatsCmd(a),
		newEventsCmd(a),
		newActionCmd(a, "start", "start", "started"),
		newActionCmd(a, "stop", "stop", "stopped"),
		newActionCmd(a, "restart", "restart", "restarted"),
		newActionCmd(a, "rm", "remove", "removed"),
		newStackCmd(a),
		newEnvCmd(a),
		newResourcesCmd(a),
		newImagesCmd(a),
		newVolumesCmd(a),
		newNetworksCmd(a),
	)

	return root
}
