package cmd

import (
	"fmt"

	"gitea.jonn.me/jons-org/nyt_cli/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage the saved API key and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown config subcommand %q", args[0])
			}
			return cmd.Help()
		},
	}

	setKey := &cobra.Command{
		Use:   "set-key <KEY>",
		Short: "Save your NYT API key to the config file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Save(&config.Config{APIKey: args[0]})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved API key to %s\n", path)
			return nil
		},
	}

	path := &cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), config.ConfigPath())
		},
	}

	show := &cobra.Command{
		Use:   "show",
		Short: "Show where the API key resolves from (value redacted)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			key, source, err := config.ResolveAPIKey(flagAPIKey)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "API key: %s (from %s)\n", redactKey(key), source)
			return nil
		},
	}

	configCmd.AddCommand(setKey, path, show)
	rootCmd.AddCommand(configCmd)
}

func redactKey(k string) string {
	if len(k) <= 6 {
		return "****"
	}
	return k[:3] + "…" + k[len(k)-2:]
}
