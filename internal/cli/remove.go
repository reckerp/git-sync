package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/reckerp/git-sync/internal/config"
)

func newRemoveCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <repo>",
		Aliases: []string{"rm"},
		Short:   "Remove a repository from the config (the local clone is kept)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return err
			}
			repo, ok := cfg.Find(args[0])
			if !ok {
				return fmt.Errorf("no repo named %q in config", args[0])
			}
			dir, err := cfg.Dir(repo)
			if err != nil {
				return err
			}
			if err := config.Remove(*cfgPath, repo); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %s from %s. The clone at %s was left in place.\n", repo.Source, *cfgPath, dir)
			return nil
		},
	}
}
