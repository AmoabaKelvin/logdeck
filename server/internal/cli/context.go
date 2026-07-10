package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newLoginCmd(a *app) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "login --url <url> [--token <token>] [--name <context>]",
		Short: "Verify a server connection and save it as the current context",
		Long:  "Verify a server connection and save it as a context in the CLI config\nfile, making it the current context for future invocations.",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !cmd.Flags().Changed("url") {
				return fmt.Errorf("--url is required (example: logdeck login --url https://logdeck.example.com --token ldk_...)")
			}
			if name == "" {
				return fmt.Errorf("--name must not be empty")
			}
			serverURL := strings.TrimRight(a.url, "/")
			token := ""
			if cmd.Flags().Changed("token") {
				token = a.token
			}

			// Verify the connection with exactly what was given, ignoring
			// env vars and existing contexts.
			probe := newClient(serverURL, token)
			var health struct {
				Status string `json:"status"`
			}
			if err := probe.get(ctx, "/healthz", nil, &health); err != nil {
				return err
			}
			// Prove the token works (or that the server needs none): hit an
			// authenticated endpoint. A 401 carries the Settings hint.
			if err := probe.get(ctx, "/containers", nil, nil); err != nil {
				return err
			}

			path, err := configPath()
			if err != nil {
				return err
			}
			cfg, err := loadConfig(path)
			if err != nil {
				return err
			}
			cfg.setContext(name, contextConfig{URL: serverURL, Token: token})
			if err := saveConfig(path, cfg); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]any{
					"message": "context saved",
					"context": name,
					"url":     serverURL,
					"token":   tokenPreview(token),
					"current": true,
				})
			}
			fmt.Printf("Saved context %q (%s, token %s) and made it current.\n", name, serverURL, tokenPreview(token))
			return nil
		}),
	}

	cmd.Flags().StringVar(&name, "name", "default", "name to save the context under")
	return cmd
}

func newLogoutCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "logout [<context>]",
		Short: "Remove the saved token from a context (keeps its URL)",
		Args:  cobra.MaximumNArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			path, err := configPath()
			if err != nil {
				return err
			}
			cfg, err := loadConfig(path)
			if err != nil {
				return err
			}

			name := cfg.CurrentContext
			if len(args) == 1 {
				name = args[0]
			}
			if name == "" {
				return fmt.Errorf("no current context; pass a context name (see: logdeck context list)")
			}
			if err := cfg.clearToken(name); err != nil {
				return err
			}
			if err := saveConfig(path, cfg); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]string{"message": "token removed", "context": name})
			}
			fmt.Printf("Removed token from context %q.\n", name)
			return nil
		}),
	}
}

func newContextCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage saved server contexts",
	}
	cmd.AddCommand(newContextListCmd(a), newContextUseCmd(a), newContextRmCmd(a))
	return cmd
}

func newContextListCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved contexts",
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			path, err := configPath()
			if err != nil {
				return err
			}
			cfg, err := loadConfig(path)
			if err != nil {
				return err
			}

			names := make([]string, 0, len(cfg.Contexts))
			for name := range cfg.Contexts {
				names = append(names, name)
			}
			sort.Strings(names)

			if a.jsonOutput() {
				contexts := make([]map[string]any, 0, len(names))
				for _, name := range names {
					contexts = append(contexts, map[string]any{
						"name":    name,
						"url":     cfg.Contexts[name].URL,
						"token":   tokenPreview(cfg.Contexts[name].Token),
						"current": name == cfg.CurrentContext,
					})
				}
				return a.printJSON(map[string]any{
					"currentContext": cfg.CurrentContext,
					"contexts":       contexts,
				})
			}

			rows := make([][]string, 0, len(names))
			for _, name := range names {
				marker := ""
				if name == cfg.CurrentContext {
					marker = "*"
				}
				rows = append(rows, []string{marker, name, cfg.Contexts[name].URL, tokenPreview(cfg.Contexts[name].Token)})
			}
			renderTable(os.Stdout, []string{"CURRENT", "NAME", "URL", "TOKEN"}, rows)
			return nil
		}),
	}
}

func newContextUseCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Make a saved context the current one",
		Args:  cobra.ExactArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			path, err := configPath()
			if err != nil {
				return err
			}
			cfg, err := loadConfig(path)
			if err != nil {
				return err
			}
			if err := cfg.useContext(args[0]); err != nil {
				return err
			}
			if err := saveConfig(path, cfg); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]string{"message": "context switched", "context": args[0]})
			}
			fmt.Printf("Switched to context %q (%s).\n", args[0], cfg.Contexts[args[0]].URL)
			return nil
		}),
	}
}

func newContextRmCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Delete a saved context",
		Args:  cobra.ExactArgs(1),
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			path, err := configPath()
			if err != nil {
				return err
			}
			cfg, err := loadConfig(path)
			if err != nil {
				return err
			}
			if err := cfg.removeContext(args[0]); err != nil {
				return err
			}
			if err := saveConfig(path, cfg); err != nil {
				return err
			}

			if a.jsonOutput() {
				return a.printJSON(map[string]string{"message": "context removed", "context": args[0]})
			}
			fmt.Printf("Removed context %q.\n", args[0])
			return nil
		}),
	}
}
