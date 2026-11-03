// Package git clones and updates repositories by shelling out to the git binary,
// so the user's SSH keys, credential helpers and git config apply unchanged.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Sync clones url into dir, or fast-forwards dir if it already holds a clone.
// It never merges or rebases, so local work is left untouched; diverged clones report an error.
func Sync(ctx context.Context, url, dir string) error {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	switch {
	case err == nil:
		return run(ctx, "-C", dir, "pull", "--ff-only", "--quiet")
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	return run(ctx, "clone", "--quiet", url, dir)
}

func run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	// The daemon has no terminal; fail instead of hanging on a credential prompt.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return nil
}
