package cli

import (
	"context"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/reckerp/git-sync/internal/config"
)

// tick is how often the daemon checks for due repos, so intervals below it round up.
const tick = time.Minute

func newDaemonCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run in the foreground and sync each repo on its interval",
		Long: `Run in the foreground and sync each repo on its interval.

The config is re-read every minute, so added repos are picked up without a restart.
With Homebrew, run it in the background via: brew services start git-sync`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runDaemon(cmd.Context(), *cfgPath)
			return nil
		},
	}
}

func runDaemon(ctx context.Context, cfgPath string) {
	slog.Info("git-sync daemon started", "config", cfgPath)
	lastSync := map[string]time.Time{} // keyed by clone directory
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		if cfg, err := config.Load(cfgPath); err != nil {
			slog.Error("loading config", "err", err)
		} else {
			syncDue(ctx, cfg, lastSync)
		}
		select {
		case <-ctx.Done():
			slog.Info("git-sync daemon stopped")
			return
		case <-ticker.C:
		}
	}
}

func syncDue(ctx context.Context, cfg *config.Config, lastSync map[string]time.Time) {
	now := time.Now()
	var due []config.Repo
	for _, r := range cfg.Repos {
		dir, err := cfg.Dir(r)
		if err != nil {
			slog.Error("resolving folder", "repo", r.Source, "err", err)
			continue
		}
		if now.Sub(lastSync[dir]) >= cfg.IntervalFor(r) {
			due = append(due, r)
		}
	}
	for _, res := range syncRepos(ctx, cfg, due) {
		// Record failures too, so a broken repo retries on its interval rather than every tick.
		lastSync[res.dir] = now
		if res.err != nil {
			slog.Error("sync failed", "repo", res.repo.Source, "err", res.err)
		} else {
			slog.Info("synced", "repo", res.repo.Source, "dir", res.dir)
		}
	}
}
