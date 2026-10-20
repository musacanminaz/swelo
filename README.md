# swelo

Disk care for developer Macs. / Developer Mac'leri için disk bakımı.

> **Status: all commands are available; feedback welcome.**
> See [PROGRESS.md](PROGRESS.md).

swelo finds files that pile up during development and can be regenerated — `node_modules`,
build folders, package manager caches, Docker leftovers, old simulators and SDKs — shows how
much space they take, and deletes only what you approve.

## What swelo never does

- Touch your source code, git-tracked files or documents
- Delete Docker volumes, databases or browser profiles
- Touch projects you changed in the last 7 days
- Run `sudo` without asking (the exact command is always shown first), or send data over the network
- Delete anything without `--apply` and your confirmation

Run `swelo permissions` to see every permission swelo may use and why.

## Commands

| Command | | Status |
|---|---|---|
| `swelo` | Opens a menu on a terminal: pick a command with ↑↓ and Enter, tick what to delete or uninstall, confirm by choosing — no flags to type. Prints the overview when not on a terminal | ✅ |
| `swelo permissions` | Every permission (Full Disk Access, sudo) explained | ✅ |
| `swelo clean` | Clean regenerable files; dry run unless `--apply`, `--pick` to tick rules | ✅ |
| `swelo analyze` | Where disk space went: categories, files > 1 GB, projects; `--tree` browses folders and moves picks to the Trash | ✅ |
| `swelo uninstall` | Lists apps (with their ~/Library leftovers) and developer toolchains (Homebrew, nvm, pyenv, rustup, simulator runtimes, Android SDK, JDKs, global npm, Docker images, editor extensions, Ollama) with size, age and which projects pin them; `--pick` or `--remove <id>` uninstalls with each tool's own command after showing it | ✅ |
| `swelo optimize` | Time Machine exclusions for regenerable folders, `git gc` on large repos, `brew autoremove` (dry run unless `--apply`, `--pick` to tick tasks) | ✅ |
| `swelo status` / `swelo history` | Disk status, free-space trend, recent cleanups / full log of everything swelo removed | ✅ |

Output language follows your system (Turkish or English); override with `--lang tr|en`.

## Install

```sh
brew install musacanminaz/tap/swelo
```

or from source: `go install github.com/musacanminaz/swelo/cmd/swelo@latest`.
Prebuilt binaries are on the [releases page](https://github.com/musacanminaz/swelo/releases)
(not notarized: run `xattr -d com.apple.quarantine swelo` after downloading).

## Quick start

```sh
swelo                      # menu: choose, tick, confirm
swelo clean                # dry run: lists what can be freed, deletes nothing
swelo clean --apply        # deletes the 🟢 items after you confirm
swelo clean --apply --risky --only docker,tools
```

What `clean` looks at:

| Risk | Rules |
|---|---|
| 🟢 safe (default) | node_modules / .next / dist / … (only next to package.json), Flutter build + .dart_tool, Pods, .venv, Rust target, .terraform, package manager caches, Gradle, Xcode DerivedData, VS Code caches, browser caches (browser closed), old Playwright browsers, closed Playwright MCP profiles, crash reports > 30 days, Docker build cache + unused images, `brew cleanup`, `pnpm store prune`, `uv cache clean` |
| 🟡 `--risky` | Docker containers stopped > 7 days, Dart pub cache, old Android build-tools, unavailable iOS simulators |
| ⚪ report only | WhatsApp / Telegram data, Xcode archives, simulator devices, Downloads, Trash |

Always skipped: git-tracked files, projects changed in the last 7 days, projects where a
process is running (dev server, editor terminal), `.env*`, `*.sql` / `*.dump` / `*.db`,
`.git`, `~/Documents`, `~/.ssh`, Docker volumes and Docker's disk image.
Every deletion is logged to `~/.local/state/swelo/history.jsonl`.

## Configuration

`~/.config/swelo/config.yaml`:

```yaml
roots: [~/Desktop, ~/code]        # where projects live (default: ~/* — every folder in home
                                  # except Library, Applications, Public, hidden, protected
                                  # and cloud-synced folders like OneDrive or Dropbox)
exclude: ["*/my-release/*"]       # never touched
recent_days: 7                    # skip projects changed this recently
disable: [browser-cache, docker]  # rule ids or groups
```

`~/.config/swelo/rules.yaml` adds rules or overrides built-in ones by `id`; see
[default.yaml](internal/rules/default.yaml) for the format.

## License

MIT
