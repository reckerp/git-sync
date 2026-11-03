package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Folder != defaultFolder || cfg.Interval != defaultInterval || len(cfg.Repos) != 0 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, `
folder = "/src"
interval = "30m"

[[repos]]
repo = "reckerp/git-sync"

[[repos]]
repo = "https://gitlab.com/me/notes.git"
folder = "/notes"
interval = "5m"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a, b := cfg.Repos[0], cfg.Repos[1]
	assertDir(t, cfg, a, "/src/git-sync")
	assertDir(t, cfg, b, "/notes/notes")
	if got := cfg.IntervalFor(a); got != 30*time.Minute {
		t.Errorf("default interval = %v", got)
	}
	if got := cfg.IntervalFor(b); got != 5*time.Minute {
		t.Errorf("override interval = %v", got)
	}
	if _, ok := cfg.Find("notes"); !ok {
		t.Error("Find by name failed")
	}
	if _, ok := cfg.Find("reckerp/git-sync"); !ok {
		t.Error("Find by source failed")
	}
}

func TestLoadRejectsBadConfig(t *testing.T) {
	for name, body := range map[string]string{
		"duplicate dir":    "[[repos]]\nrepo = \"a/dots\"\n[[repos]]\nrepo = \"b/dots\"\n",
		"empty source":     "[[repos]]\nfolder = \"/x\"\n",
		"invalid interval": "interval = \"soon\"\n",
		"zero interval":    "interval = \"0s\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			writeFile(t, path, body)
			if _, err := Load(path); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestAppendCreatesThenPreservesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")
	cfg, _ := Load(path)

	if err := Append(path, cfg, Repo{Source: "a/one"}); err != nil {
		t.Fatal(err)
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("# keep me\n")
	f.Close()
	if err := Append(path, cfg, Repo{Source: "a/two", Folder: "/x", Interval: Duration(90 * time.Minute)}); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "# keep me") || !strings.Contains(string(raw), `interval = "1h30m"`) {
		t.Fatalf("unexpected file:\n%s", raw)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 2 || got.Repos[1].Folder != "/x" || got.IntervalFor(got.Repos[1]) != 90*time.Minute {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestRepoNameAndCloneURL(t *testing.T) {
	for _, tc := range []struct{ src, name, url string }{
		{"reckerp/git-sync", "git-sync", "git@github.com:reckerp/git-sync.git"},
		{"git@github.com:a/b.git", "b", "git@github.com:a/b.git"},
		{"https://example.com/a/c.git", "c", "https://example.com/a/c.git"},
	} {
		r := Repo{Source: tc.src}
		if r.Name() != tc.name || r.CloneURL() != tc.url {
			t.Errorf("%s: got name %q url %q", tc.src, r.Name(), r.CloneURL())
		}
	}
}

func TestDurationString(t *testing.T) {
	for d, want := range map[time.Duration]string{
		time.Hour: "1h", 30 * time.Minute: "30m", 90 * time.Minute: "1h30m", 10 * time.Second: "10s", 2*time.Hour + 5*time.Second: "2h0m5s",
	} {
		if got := Duration(d).String(); got != want {
			t.Errorf("%v: got %q want %q", d, got, want)
		}
	}
}

func assertDir(t *testing.T, cfg *Config, r Repo, want string) {
	t.Helper()
	got, err := cfg.Dir(r)
	if err != nil || got != want {
		t.Errorf("Dir(%s) = %q, %v; want %q", r.Source, got, err, want)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
