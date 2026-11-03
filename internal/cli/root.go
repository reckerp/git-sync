// Package cli implements the git-sync command line.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/reckerp/git-sync/internal/config"
)

// Execute runs the CLI and returns any error after cobra has printed it.
func Execute(version string) error {
	return newRootCmd(version).Execute()
}

func newRootCmd(version string) *cobra.Command {
	defaultPath, _ := config.DefaultPath()
	var cfgPath string

	root := &cobra.Command{
		Use:          "git-sync",
		Short:        "Keep the git repositories declared in your config cloned and up to date",
		Version:      version,
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&cfgPath, "config", defaultPath, "path to the config file")

	root.AddCommand(newAddCmd(&cfgPath), newSyncCmd(&cfgPath), newDaemonCmd(&cfgPath))
	return root
}
