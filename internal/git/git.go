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
	"time"
)

// Sync clones url into dir, or fast-forwards dir if it already holds a clone.
// It never merges or rebases, so local work is left untouched; diverged clones report an error.
func Sync(ctx context.Context, url, dir string) error {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	switch {
	case err == nil:
		return run(ctx, sshEnv(ctx, dir), "-C", dir, "pull", "--ff-only", "--quiet")
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	return run(ctx, sshEnv(ctx, parent), "clone", "--quiet", url, dir)
}

// keepaliveSSH makes ssh give up on a dead connection, e.g. one the Mac slept through,
// instead of waiting on it forever.
const keepaliveSSH = "ssh -o ServerAliveInterval=15 -o ServerAliveCountMax=4"

// sshEnv returns a GIT_SSH_COMMAND with keepalives for git run in dir,
// unless the user already chose an ssh command.
func sshEnv(ctx context.Context, dir string) []string {
	if os.Getenv("GIT_SSH_COMMAND") != "" || os.Getenv("GIT_SSH") != "" {
		return nil
	}
	if out, _ := output(ctx, nil, "-C", dir, "config", "--get", "core.sshCommand"); out != "" {
		return nil
	}
	return []string{"GIT_SSH_COMMAND=" + keepaliveSSH}
}

// CheckSafeToDelete returns an error if dir is not a clone or holds work that exists
// nowhere else: uncommitted or untracked files, commits on no remote, or stashes.
// Files matched by .gitignore are not checked.
func CheckSafeToDelete(ctx context.Context, dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return fmt.Errorf("%s is not a git clone", dir)
	}
	for _, c := range []struct {
		what string
		args []string
	}{
		{"uncommitted or untracked files", []string{"status", "--porcelain"}},
		{"commits not pushed to any remote", []string{"log", "--branches", "--not", "--remotes", "--oneline"}},
		{"stashed changes", []string{"stash", "list"}},
	} {
		out, err := output(ctx, nil, append([]string{"-C", dir}, c.args...)...)
		if err != nil {
			return err
		}
		if out != "" {
			return fmt.Errorf("%s has %s", dir, c.what)
		}
	}
	return nil
}

func run(ctx context.Context, env []string, args ...string) error {
	_, err := output(ctx, env, args...)
	return err
}

// output runs git with env added to the environment and returns its trimmed combined output.
func output(ctx context.Context, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	// Interrupt rather than kill on cancel, so an aborted clone removes its half-written directory.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 10 * time.Second
	// The daemon has no terminal; fail instead of hanging on a credential prompt.
	cmd.Env = append(append(os.Environ(), "GIT_TERMINAL_PROMPT=0"), env...)
	out, err := cmd.CombinedOutput()
	out = bytes.TrimSpace(out)
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}
