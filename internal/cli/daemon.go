package cli

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/reckerp/git-sync/internal/config"
)

// tick is how often the daemon checks for due repos, so intervals below it round up.
const tick = time.Minute

// syncTimeout bounds one round of syncs, so a hung git (e.g. a dead connection) cannot stall the daemon.
// shortcut: a single clone slower than this never finishes in the daemon; run `git-sync sync` once for huge repos.
const syncTimeout = time.Hour

func newDaemonCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run in the foreground and sync each repo on its interval",
		Long: `Run in the foreground and sync each repo on its interval.

The config is re-read every minute, so added repos are picked up without a restart.
With Homebrew, run it in the background via: brew services start git-sync`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Only the daemon traps signals; interactive commands keep the default Ctrl-C behavior.
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			runDaemon(ctx, *cfgPath)
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
	now := time.Now().Round(0) // wall clock, so time spent asleep counts toward the interval
	var due []config.Repo
	for _, r := range cfg.Repos {
		dir, err := cfg.Dir(r)
		if err != nil {
			slog.Error("resolving folder", "repo", r.Source, "err", err)
			continue
		}
		// Half a tick of slack so scheduling jitter does not push a sync to the next tick.
		if now.Sub(lastSync[dir]) >= cfg.IntervalFor(r)-tick/2 {
			due = append(due, r)
		}
	}
	// shortcut: due repos sync as one batch, so a slow clone delays the others; schedule repos independently if that bites.
	ctx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()
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
