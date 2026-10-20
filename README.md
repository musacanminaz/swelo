# swelo

Disk care for developer Macs. / Developer Mac'leri için disk bakımı.

> **Status: `clean` and `analyze` are ready; `uninstall`, `optimize`, `status` are in progress.**
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
| `swelo` | What swelo does, permission status, command list | ✅ |
| `swelo permissions` | Every permission (Full Disk Access, sudo) explained | ✅ |
| `swelo clean` | Clean regenerable files; dry run unless `--apply` | ✅ |
| `swelo analyze` | Where disk space went: categories, files > 1 GB, projects; `--tree` browses folders and moves picks to the Trash | ✅ |
| `swelo uninstall` | Remove apps and developer toolchains via their own uninstallers | planned |
| `swelo optimize` | Spotlight / Time Machine exclusions, Docker disk, git gc | planned |
| `swelo status` | Disk status and history | planned |

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
swelo                      # what swelo does, permission status
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
roots: [~/Desktop, ~/code]        # where projects live (default: common folders)
exclude: ["*/my-release/*"]       # never touched
recent_days: 7                    # skip projects changed this recently
disable: [browser-cache, docker]  # rule ids or groups
```

`~/.config/swelo/rules.yaml` adds rules or overrides built-in ones by `id`; see
[default.yaml](internal/rules/default.yaml) for the format.

## License

MIT
