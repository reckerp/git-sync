package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/reckerp/git-sync/internal/config"
)

func newAddCmd(cfgPath *string) *cobra.Command {
	var folder, interval string
	var yes bool

	cmd := &cobra.Command{
		Use:   "add <owner/name | git-url>",
		Short: "Add a repository to the config and sync it",
		Long: `Add a repository to the config and sync it.

Without --folder or --interval you are asked whether to use the defaults.
Pass --yes (or run without a terminal) to accept the defaults silently.`,
		Example: `  git-sync add reckerp/git-sync
  git-sync add git@gitlab.com:me/notes.git --folder ~/notes --interval 10m`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return err
			}
			repo := config.Repo{Source: args[0], Folder: folder}
			if _, exists := cfg.Find(repo.Source); exists {
				return fmt.Errorf("%s is already in %s", repo.Source, *cfgPath)
			}
			if interval != "" {
				if repo.Interval, err = config.ParseDuration(interval); err != nil {
					return err
				}
			}

			if !yes && isTerminal(cmd.InOrStdin()) {
				p := newPrompter(cmd.InOrStdin(), cmd.OutOrStdout())
				if err := askOverrides(p, cfg, &repo, !cmd.Flags().Changed("folder"), !cmd.Flags().Changed("interval")); err != nil {
					return err
				}
			}

			cfg.Repos = append(cfg.Repos, repo)
			if err := cfg.Validate(); err != nil {
				return err
			}
			if err := config.Append(*cfgPath, cfg, repo); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added %s to %s\n", repo.Source, *cfgPath)
			return report(cmd.OutOrStdout(), syncRepos(cmd.Context(), cfg, []config.Repo{repo}))
		},
	}
	cmd.Flags().StringVarP(&folder, "folder", "f", "", "parent folder to clone into (default from config)")
	cmd.Flags().StringVarP(&interval, "interval", "i", "", "pull interval, e.g. 15m or 2h (default from config)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "use defaults without asking")
	return cmd
}

// askOverrides offers to replace the default folder and interval for fields not set by flags.
func askOverrides(p prompter, cfg *config.Config, repo *config.Repo, askFolder, askInterval bool) error {
	if askFolder {
		ok, err := p.confirm(fmt.Sprintf("Sync into default folder %s?", cfg.Folder))
		if err != nil {
			return err
		}
		if !ok {
			if repo.Folder, err = p.ask("Folder: "); err != nil {
				return err
			}
		}
	}
	if askInterval {
		ok, err := p.confirm(fmt.Sprintf("Use default sync interval %s?", cfg.Interval))
		if err != nil {
			return err
		}
		for !ok {
			answer, err := p.ask("Interval (e.g. 15m, 2h): ")
			if err != nil {
				return err
			}
			if repo.Interval, err = config.ParseDuration(answer); err == nil {
				break
			}
			fmt.Fprintln(p.out, err)
		}
	}
	return nil
}
