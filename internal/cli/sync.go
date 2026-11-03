package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/reckerp/git-sync/internal/config"
)

func newSyncCmd(cfgPath *string) *cobra.Command {
	var interactive bool

	cmd := &cobra.Command{
		Use:   "sync [repo...]",
		Short: "Clone or pull all repositories, or only the named ones",
		Example: `  git-sync sync
  git-sync sync git-sync
  git-sync sync -i`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if interactive && len(args) > 0 {
				return errors.New("pass repo names or --interactive, not both")
			}
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return err
			}

			repos := cfg.Repos
			switch {
			case interactive:
				repos, err = pickRepos(newPrompter(cmd.InOrStdin(), cmd.OutOrStdout()), cfg)
			case len(args) > 0:
				repos, err = findRepos(cfg, args)
			}
			if err != nil {
				return err
			}
			if len(repos) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No repos configured in %s; add one with `git-sync add`.\n", *cfgPath)
				return nil
			}
			return report(cmd.OutOrStdout(), syncRepos(cmd.Context(), cfg, repos))
		},
	}
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "choose which repos to sync")
	return cmd
}

func findRepos(cfg *config.Config, names []string) ([]config.Repo, error) {
	repos := make([]config.Repo, 0, len(names))
	for _, name := range names {
		r, ok := cfg.Find(name)
		if !ok {
			return nil, fmt.Errorf("no repo named %q in config", name)
		}
		repos = append(repos, r)
	}
	return repos, nil
}

func pickRepos(p prompter, cfg *config.Config) ([]config.Repo, error) {
	for i, r := range cfg.Repos {
		fmt.Fprintf(p.out, "%3d) %s\n", i+1, r.Source)
	}
	answer, err := p.ask("Repos to sync (e.g. 1,3 or blank for all): ")
	if err != nil {
		return nil, err
	}
	idx, err := parseSelection(answer, len(cfg.Repos))
	if err != nil {
		return nil, err
	}
	if idx == nil {
		return cfg.Repos, nil
	}
	repos := make([]config.Repo, len(idx))
	for i, n := range idx {
		repos[i] = cfg.Repos[n]
	}
	return repos, nil
}

// parseSelection turns "1, 3 4" into zero-based indexes. A blank answer returns nil, meaning all.
func parseSelection(s string, n int) ([]int, error) {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' })
	seen := make(map[int]bool, len(fields))
	var idx []int
	for _, f := range fields {
		v, err := strconv.Atoi(f)
		if err != nil || v < 1 || v > n {
			return nil, fmt.Errorf("invalid choice %q: pick numbers from 1 to %d", f, n)
		}
		if !seen[v] {
			seen[v] = true
			idx = append(idx, v-1)
		}
	}
	return idx, nil
}
