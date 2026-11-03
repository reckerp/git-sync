// Package config loads and updates the git-sync TOML configuration.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
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
		if other, ok := seen[dir]; ok {
			return fmt.Errorf("repos %q and %q would both sync to %s", other, r.Source, dir)
		}
		seen[dir] = r.Source
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
	return filepath.Join(folder, r.Name()), nil
}

// IntervalFor returns how often r should be pulled.
func (c *Config) IntervalFor(r Repo) time.Duration {
	if r.Interval > 0 {
		return time.Duration(r.Interval)
	}
	return time.Duration(c.Interval)
}

// Name is the repository name without owner or .git suffix, e.g. "git-sync".
func (r Repo) Name() string {
	return path.Base(strings.TrimSuffix(r.Source, ".git"))
}

// CloneURL expands "owner/name" to a GitHub SSH URL and returns anything else unchanged.
func (r Repo) CloneURL() string {
	if strings.Contains(r.Source, ":") {
		return r.Source
	}
	return "git@github.com:" + r.Source + ".git"
}

// Append adds r to the config file at path, creating the file with defaults if needed.
// It appends instead of rewriting so comments and formatting in the file survive.
func Append(path string, cfg *Config, r Repo) error {
	var buf bytes.Buffer
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
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

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
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
