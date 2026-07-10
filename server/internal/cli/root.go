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
	url         string
	token       string
	output      string
	contextName string

	client *client
	conn   connection

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

const rootLong = `logdeck talks to a running LogDeck server over its HTTP API.
It is non-interactive and scriptable; use -o json for machine-readable output.

Connect once with "logdeck login" to save the server as a context in
~/.config/logdeck/config.json. Connection settings resolve in this order:
explicit --url/--token flags > LOGDECK_URL/LOGDECK_TOKEN env vars > the
active context (--context selects another saved one) > http://localhost:8080.`

func newRootCmd(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:           "logdeck",
		Short:         "Command-line client for a running LogDeck server",
		Long:          rootLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if a.output != "table" && a.output != "json" {
				return fmt.Errorf("invalid output format %q (must be table or json)", a.output)
			}

			path, err := configPath()
			if err != nil {
				return err
			}
			cfg, err := loadConfig(path)
			if err != nil {
				return err
			}
			conn, err := resolveConnection(
				a.url, cmd.Flags().Changed("url"),
				a.token, cmd.Flags().Changed("token"),
				os.Getenv("LOGDECK_URL"), os.Getenv("LOGDECK_TOKEN"),
				cfg, a.contextName,
			)
			if err != nil {
				return err
			}
			a.conn = conn
			a.client = newClient(strings.TrimRight(conn.url, "/"), conn.token)
			return nil
		},
	}

	root.PersistentFlags().StringVar(&a.url, "url", "", "LogDeck server URL (overrides LOGDECK_URL and the active context; default http://localhost:8080)")
	root.PersistentFlags().StringVar(&a.token, "token", "", "API token sent as a Bearer token (overrides LOGDECK_TOKEN and the active context)")
	root.PersistentFlags().StringVar(&a.contextName, "context", "", "use this saved context instead of the current one for this invocation")
	root.PersistentFlags().StringVarP(&a.output, "output", "o", "table", "output format: table or json")

	root.AddCommand(
		newStatusCmd(a),
		newLoginCmd(a),
		newLogoutCmd(a),
		newContextCmd(a),
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
