package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/reckerp/git-sync/internal/config"
	"github.com/reckerp/git-sync/internal/git"
)

func newRemoveCmd(cfgPath *string) *cobra.Command {
	var del bool

	cmd := &cobra.Command{
		Use:     "remove <repo>",
		Aliases: []string{"rm"},
		Short:   "Remove a repository from the config, keeping the local clone unless --delete is set",
		Long: `Remove a repository from the config, keeping the local clone unless --delete is set.

--delete refuses to delete a clone with uncommitted or untracked files, unpushed
commits or stashes; nothing is changed in that case. Ignored files are deleted.`,
		Args: cobra.ExactArgs(1),
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

			// Check before touching the config, so a refusal leaves everything as it was.
			cloneExists := false
			if del {
				if _, err := os.Stat(dir); err == nil {
					if err := git.CheckSafeToDelete(cmd.Context(), dir); err != nil {
						return fmt.Errorf("%w; nothing was changed", err)
					}
					cloneExists = true
				} else if !errors.Is(err, fs.ErrNotExist) {
					return err
				}
			}

			if err := config.Remove(*cfgPath, repo); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Removed %s from %s.\n", repo.Source, *cfgPath)
			switch {
			case !del:
				fmt.Fprintf(out, "The clone at %s was left in place.\n", dir)
			case cloneExists:
				if err := os.RemoveAll(dir); err != nil {
					return err
				}
				fmt.Fprintf(out, "Deleted %s.\n", dir)
			default:
				fmt.Fprintf(out, "No clone found at %s.\n", dir)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&del, "delete", "d", false, "also delete the local clone")
	return cmd
}
