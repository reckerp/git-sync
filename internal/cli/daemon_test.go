package cli

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/reckerp/git-sync/internal/config"
)

func TestSyncDue(t *testing.T) {
	// The remote does not exist, so each sync fails fast; lastSync is updated only for due repos.
	cfg := &config.Config{Folder: t.TempDir(), Interval: config.Duration(time.Hour), Repos: []config.Repo{
		{Source: "file://" + filepath.Join(t.TempDir(), "due.git")},
		{Source: "file://" + filepath.Join(t.TempDir(), "notdue.git")},
	}}
	dueDir, _ := cfg.Dir(cfg.Repos[0])
	notDueDir, _ := cfg.Dir(cfg.Repos[1])
	now := time.Now()
	lastSync := map[string]time.Time{
		dueDir:    now.Add(-time.Hour + 10*time.Second), // within the slack, so due
		notDueDir: now.Add(-30 * time.Minute),
	}
	before := lastSync[notDueDir]

	syncDue(context.Background(), cfg, lastSync)

	if !lastSync[dueDir].After(now.Add(-time.Second)) {
		t.Error("repo just under its interval was not synced")
	}
	if lastSync[notDueDir] != before {
		t.Error("repo not yet due was synced")
	}
}

func TestWithWallTimeout(t *testing.T) {
	ctx, cancel := withWallTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("context was not cancelled after its deadline")
	}
}
