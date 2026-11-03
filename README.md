# git-sync

Declare your git repositories in a TOML file and git-sync keeps them cloned and up to date.

## Install

```sh
brew install reckerp/tap/git-sync
brew services start git-sync   # run the background daemon
```

Or build from source: `go install github.com/reckerp/git-sync@latest`.

## Config

`~/.config/git-sync/config.toml`:

```toml
folder = "~/git"     # default parent folder for clones
interval = "1h"      # default pull interval

[[repos]]
repo = "reckerp/git-sync"            # owner/name → git@github.com:owner/name.git

[[repos]]
repo = "https://gitlab.com/me/notes.git"   # any URL git understands
folder = "~/notes"                         # per-repo override
interval = "10m"                           # per-repo override
```

Each repo is cloned to `<folder>/<name>`. Updates use `git pull --ff-only`, so local
changes are never merged or overwritten; a diverged clone is reported as an error.
git-sync uses your installed `git`, so your SSH keys and credential helpers apply.

## Usage

```sh
git-sync add reckerp/git-sync                 # asks whether to use the default folder/interval
git-sync add owner/repo -f ~/work -i 15m      # or set them with flags (-y accepts defaults)
git-sync sync                                 # clone or pull every repo
git-sync sync git-sync                        # only the named repos
git-sync sync -i                              # choose repos interactively
git-sync remove git-sync                      # drop from config (alias rm); the clone stays on disk
git-sync daemon                               # foreground loop; syncs each repo on its interval
```

The daemon re-reads the config every minute, so new repos are picked up without a restart.
Logs from `brew services` go to `$(brew --prefix)/var/log/git-sync.log`.

## Releasing

Push a `v*` tag. GoReleaser builds the binaries, creates the GitHub release and updates the
formula in `reckerp/homebrew-tap`. The workflow needs a `PUBLISHER_TOKEN` secret with write
access to both repositories.
