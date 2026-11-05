package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSyncClonesThenPulls(t *testing.T) {
	isolateGit(t)
	tmp := t.TempDir()
	remote := filepath.Join(tmp, "remote.git")
	work := filepath.Join(tmp, "work")
	clone := filepath.Join(tmp, "nested", "clone")
	ctx := context.Background()

	git(t, "init", "--bare", "-b", "main", remote)
	git(t, "clone", remote, work)
	commitFile(t, work, "a.txt")

	if err := Sync(ctx, remote, clone); err != nil {
		t.Fatal(err)
	}
	assertExists(t, filepath.Join(clone, "a.txt"))

	commitFile(t, work, "b.txt")
	if err := Sync(ctx, remote, clone); err != nil {
		t.Fatal(err)
	}
	assertExists(t, filepath.Join(clone, "b.txt"))
}

func TestSyncReportsGitErrors(t *testing.T) {
	err := Sync(context.Background(), filepath.Join(t.TempDir(), "missing.git"), filepath.Join(t.TempDir(), "x"))
	if err == nil {
		t.Fatal("expected error for missing remote")
	}
}

func commitFile(t *testing.T, dir, name string) {
	t.Helper()
	writeFile(t, dir, name, name)
	git(t, "-C", dir, "add", name)
	git(t, "-C", dir, "commit", "-m", name)
	git(t, "-C", dir, "push", "origin", "HEAD:main")
}

func git(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestCheckSafeToDelete(t *testing.T) {
	isolateGit(t)
	ctx := context.Background()
	for name, tc := range map[string]struct {
		dirty func(t *testing.T, clone string)
		safe  bool
	}{
		"clean": {func(*testing.T, string) {}, true},
		"ignored": {func(t *testing.T, c string) {
			writeFile(t, c, ".gitignore", "x")
			git(t, "-C", c, "add", ".gitignore")
			git(t, "-C", c, "commit", "-m", "ignore")
			git(t, "-C", c, "push", "origin", "HEAD:main")
			writeFile(t, c, "x", "")
		}, true},
		"untracked": {func(t *testing.T, c string) { writeFile(t, c, "new.txt", "") }, false},
		"modified":  {func(t *testing.T, c string) { writeFile(t, c, "a.txt", "changed") }, false},
		"unpushed": {func(t *testing.T, c string) {
			writeFile(t, c, "b.txt", "")
			git(t, "-C", c, "add", "b.txt")
			git(t, "-C", c, "commit", "-m", "local")
		}, false},
		"stashed": {func(t *testing.T, c string) { writeFile(t, c, "a.txt", "changed"); git(t, "-C", c, "stash") }, false},
	} {
		t.Run(name, func(t *testing.T) {
			tmp := t.TempDir()
			remote, clone := filepath.Join(tmp, "remote.git"), filepath.Join(tmp, "clone")
			git(t, "init", "--bare", "-b", "main", remote)
			git(t, "clone", remote, clone)
			commitFile(t, clone, "a.txt")
			tc.dirty(t, clone)

			if err := CheckSafeToDelete(ctx, clone); (err == nil) != tc.safe {
				t.Fatalf("safe = %v, err = %v", tc.safe, err)
			}
		})
	}

	if err := CheckSafeToDelete(ctx, t.TempDir()); err == nil {
		t.Fatal("expected error for a folder that is not a clone")
	}
}

func isolateGit(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t",
		"GIT_CONFIG_GLOBAL": os.DevNull,
	} {
		t.Setenv(k, v)
	}
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSSHEnv(t *testing.T) {
	isolateGit(t)
	t.Setenv("GIT_SSH", "")
	ctx := context.Background()
	repo := t.TempDir()
	git(t, "init", "-q", repo)

	t.Setenv("GIT_SSH_COMMAND", "")
	if got := sshEnv(ctx, repo); len(got) != 1 || got[0] != "GIT_SSH_COMMAND="+keepaliveSSH {
		t.Errorf("default: got %v", got)
	}

	t.Setenv("GIT_SSH_COMMAND", "ssh -i key")
	if got := sshEnv(ctx, repo); got != nil {
		t.Errorf("GIT_SSH_COMMAND set: got %v", got)
	}

	t.Setenv("GIT_SSH_COMMAND", "")
	git(t, "-C", repo, "config", "core.sshCommand", "ssh -i key")
	if got := sshEnv(ctx, repo); got != nil {
		t.Errorf("core.sshCommand set: got %v", got)
	}
}
