// Package config loads and updates the git-sync TOML configuration.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	defaultFolder   = "~/git"
	defaultInterval = Duration(time.Hour)
)

// Config is the on-disk configuration.
type Config struct {
	Folder   string   `toml:"folder"`
	Interval Duration `toml:"interval"`
	Repos    []Repo   `toml:"repos,omitempty"`
}

// Repo is one repository to keep in sync. Folder and Interval override the defaults when set.
type Repo struct {
	// Source is "owner/name" (cloned from GitHub over SSH) or any URL git understands.
	Source   string   `toml:"repo"`
	Folder   string   `toml:"folder,omitempty"`
	Interval Duration `toml:"interval,omitzero"`
}

// DefaultPath returns ~/.config/git-sync/config.toml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "git-sync", "config.toml"), nil
}

// Load reads the config at path. A missing file yields the default config.
func Load(path string) (*Config, error) {
	cfg := &Config{Folder: defaultFolder, Interval: defaultInterval}
	if _, err := toml.DecodeFile(path, cfg); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	return cfg, nil
}

// Validate checks that every repo has a source and that no two repos share a directory.
func (c *Config) Validate() error {
	seen := make(map[string]string, len(c.Repos))
	for _, r := range c.Repos {
		if strings.TrimSpace(r.Source) == "" {
			return errors.New("repo entry without a repo value")
		}
		dir, err := c.Dir(r)
		if err != nil {
			return err
		}
		key := strings.ToLower(dir) // macOS file systems are case-insensitive by default
		if other, ok := seen[key]; ok {
			return fmt.Errorf("repos %q and %q would both sync to %s", other, r.Source, dir)
		}
		seen[key] = r.Source
	}
	return nil
}

// Find returns the repo whose name or source matches name.
func (c *Config) Find(name string) (Repo, bool) {
	for _, r := range c.Repos {
		if r.Name() == name || r.Source == name {
			return r, true
		}
	}
	return Repo{}, false
}

// Dir returns the absolute directory r is cloned into.
func (c *Config) Dir(r Repo) (string, error) {
	folder := r.Folder
	if folder == "" {
		folder = c.Folder
	}
	folder, err := ExpandHome(folder)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(folder) {
		return "", fmt.Errorf("folder %q must be an absolute path or start with ~", folder)
	}
	return filepath.Join(folder, r.Name()), nil
}

// IntervalFor returns how often r should be pulled.
func (c *Config) IntervalFor(r Repo) time.Duration {
	if r.Interval > 0 {
		return time.Duration(r.Interval)
	}
	return time.Duration(c.Interval)
}

// Name is the repository name without host, owner or .git suffix, e.g. "git-sync".
func (r Repo) Name() string {
	s := strings.TrimSuffix(strings.TrimRight(r.Source, "/"), ".git")
	return s[strings.LastIndexAny(s, "/:")+1:]
}

// CloneURL expands "owner/name" to a GitHub SSH URL and returns anything else unchanged.
func (r Repo) CloneURL() string {
	s := r.Source
	// URLs, scp-style addresses and local paths (/x, ./x, ~/x, a/b/c) go to git as written.
	if strings.Contains(s, ":") || strings.Count(s, "/") != 1 || strings.ContainsAny(s[:1], "/.~") {
		return s
	}
	return "git@github.com:" + strings.TrimSuffix(s, ".git") + ".git"
}

// Append adds r to the config file at path, creating the file with defaults if needed.
// It appends instead of rewriting so comments and formatting in the file survive.
func Append(path string, cfg *Config, r Repo) error {
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var buf bytes.Buffer
	if errors.Is(err, fs.ErrNotExist) {
		if err := toml.NewEncoder(&buf).Encode(Config{Folder: cfg.Folder, Interval: cfg.Interval}); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
	}
	buf.WriteString("\n")
	if err := toml.NewEncoder(&buf).Encode(struct {
		Repos []Repo `toml:"repos"`
	}{[]Repo{r}}); err != nil {
		return err
	}
	// [[repos]] cannot extend repos written another way, e.g. as an inline array.
	if _, err := toml.Decode(string(old)+buf.String(), &Config{}); err != nil {
		return fmt.Errorf("cannot append to %s: %w", path, err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(buf.Bytes())
	return errors.Join(err, f.Close())
}

// Remove deletes r's [[repos]] block from the config file at path.
// It edits the text instead of re-encoding so comments and formatting elsewhere survive.
func Remove(path string, r Repo) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cfg Config
	if _, err := toml.Decode(string(raw), &cfg); err != nil {
		return fmt.Errorf("remove from %s: %w", path, err)
	}
	idx := slices.Index(cfg.Repos, r)
	if idx < 0 {
		return fmt.Errorf("%s is not in %s", r.Source, path)
	}
	want := slices.Delete(slices.Clone(cfg.Repos), idx, idx+1)

	lines := strings.SplitAfter(string(raw), "\n")
	start, end, n := -1, len(lines), 0
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if start >= 0 && strings.HasPrefix(t, "[") {
			end = i
			break
		}
		if strings.HasPrefix(t, "[[repos]]") {
			if n == idx {
				start = i
			}
			n++
		}
	}
	// Blank lines and comments right above the next table belong to it, so keep them.
	for start >= 0 && end > start+1 && isBlankOrComment(lines[end-1]) {
		end--
	}
	if start > 0 && strings.TrimSpace(lines[start-1]) == "" {
		start-- // drop the separating blank line too
	}

	// Refuse rather than corrupt the file if the text layout did not match the parsed config,
	// e.g. repos written as an inline array.
	var out string
	if start >= 0 {
		out = strings.Join(slices.Delete(lines, start, end), "")
	}
	var check Config
	if _, err := toml.Decode(out, &check); start < 0 || err != nil || !slices.Equal(check.Repos, want) {
		return fmt.Errorf("cannot safely remove %s from %s; edit the file by hand", r.Source, path)
	}

	// Write to a temp file and rename, so a crash cannot leave a half-written config.
	// Rename the real file, not a symlink to it (e.g. a config linked from a dotfiles repo).
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func isBlankOrComment(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || strings.HasPrefix(t, "#")
}

// ExpandHome replaces a leading "~" with the user's home directory.
func ExpandHome(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, p[1:]), nil
}

// Duration is a time.Duration written as a string like "30m" in TOML.
type Duration time.Duration

// ParseDuration parses a positive duration such as "15m" or "2h".
func ParseDuration(s string) (Duration, error) {
	var d Duration
	err := d.UnmarshalText([]byte(s))
	return d, err
}

func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	if v <= 0 {
		return fmt.Errorf("interval must be positive, got %q", b)
	}
	*d = Duration(v)
	return nil
}

// MarshalText writes the shortest form, e.g. "1h" instead of "1h0m0s".
func (d Duration) MarshalText() ([]byte, error) {
	s := time.Duration(d).String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return []byte(s), nil
}

func (d Duration) String() string {
	b, _ := d.MarshalText()
	return string(b)
}
