package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version is overridable at build time:
//
//	go build -ldflags "-X gitea.jonn.me/jons-org/nyt_cli/cmd.Version=v1.2.3"
var Version = "dev"

func init() {
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate("nyt {{.Version}}\n")
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "nyt %s (%s/%s, %s)\n",
				Version, runtime.GOOS, runtime.GOARCH, runtime.Version())
		},
	})
}
