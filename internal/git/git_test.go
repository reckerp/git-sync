package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSyncClonesThenPulls(t *testing.T) {
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t",
		"GIT_CONFIG_GLOBAL": os.DevNull,
	} {
		t.Setenv(k, v)
	}
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
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
		t.Fatal(err)
	}
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
