package cli

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/reckerp/git-sync/internal/config"
	"github.com/reckerp/git-sync/internal/git"
)

const maxParallelSyncs = 4

type result struct {
	repo config.Repo
	dir  string
	err  error
}

// syncRepos syncs repos in parallel and returns results in input order.
func syncRepos(ctx context.Context, cfg *config.Config, repos []config.Repo) []result {
	results := make([]result, len(repos))
	sem := make(chan struct{}, maxParallelSyncs)
	var wg sync.WaitGroup
	for i, r := range repos {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			dir, err := cfg.Dir(r)
			if err == nil {
				err = git.Sync(ctx, r.CloneURL(), dir)
			}
			results[i] = result{repo: r, dir: dir, err: err}
		})
	}
	wg.Wait()
	return results
}

// report prints one line per result and returns an error if any sync failed.
func report(w io.Writer, results []result) error {
	failed := 0
	for _, res := range results {
		if res.err != nil {
			failed++
			fmt.Fprintf(w, "✗ %s: %v\n", res.repo.Name(), res.err)
			continue
		}
		fmt.Fprintf(w, "✓ %s → %s\n", res.repo.Name(), res.dir)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d repos failed to sync", failed, len(results))
	}
	return nil
}
