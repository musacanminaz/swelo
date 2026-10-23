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

Run `swelo permissions` to see every permission swelo may use and why. In the menu, **Permissions**
grants admin rights for the session (sudo asks for the password once, every sudo step is still shown and
confirmed, and `sudo -k` runs when swelo quits) and opens the Full Disk Access settings.

## Commands

| Command | | Status |
|---|---|---|
| `swelo` | Opens a menu on a terminal: Quick clean (safe rules, one confirmation), Clean (tick rules, open a rule with → to keep single items), Analyze, Folder tree, Uninstall (pick a kind, then tick), Optimize, Status, History, Permissions (grant admin rights for the session, open Full Disk Access). Pick lists: space ticks, `a`/`n` all/none, `/` filters. Prints the overview when not on a terminal | ✅ |
| `swelo permissions` | Every permission (Full Disk Access, sudo) explained | ✅ |
| `swelo clean [rule …]` | Clean regenerable files; dry run unless `--apply`, `--pick` to tick rules; name rules or groups to run only those (`swelo clean node docker`) | ✅ |
| `swelo rules` | Every rule by group: risk, id, what it looks at, whether the config disabled it | ✅ |
| `swelo analyze` | Where disk space went: categories, files > 1 GB, projects; `--tree` browses folders and moves picks to the Trash | ✅ |
| `swelo uninstall` | Lists apps (with their ~/Library leftovers) and developer toolchains (Homebrew, nvm, pyenv, rustup, simulator runtimes, Android SDK, JDKs, global npm, Docker images, editor extensions, Ollama) with size, age and which projects pin them; `--pick` or `--remove <name>` uninstalls with each tool's own command after showing it | ✅ |
| `swelo optimize` | Time Machine exclusions for regenerable folders, `git gc` on large repos, `brew autoremove` (dry run unless `--apply`, `--pick` to tick tasks) | ✅ |
| `swelo speed` | What slows the Mac down: RAM, swap, load, uptime, the apps behind them (memory as Activity Monitor counts it) and what would help | ✅ |
| `swelo status` / `swelo history` | Disk status, free-space trend, recent cleanups / full log of everything swelo removed | ✅ |

Output language follows your system (Turkish or English); override with `--lang tr|en`.
`--json` prints one JSON document instead (clean, analyze, uninstall, status; no prompts, so
`clean --apply` needs `--yes`). Exit status: 0 done, 1 error, 2 cancelled by you.

## Install

```sh
brew install musacanminaz/tap/swelo
```

Homebrew also installs shell completions (zsh, bash, fish): rule names, providers and config keys complete with Tab.

or from source: `go install github.com/musacanminaz/swelo/cmd/swelo@latest`
(then `swelo completion zsh|bash|fish` prints the completion script).
Prebuilt binaries are on the [releases page](https://github.com/musacanminaz/swelo/releases)
(not notarized: run `xattr -d com.apple.quarantine swelo` after downloading).

## Quick start

```sh
swelo                      # menu: Quick clean, Clean (tick), Analyze, Uninstall …
swelo clean                # dry run: lists what can be freed, deletes nothing
swelo clean --apply        # deletes the 🟢 items after you confirm
swelo clean docker tools --apply --risky   # only these rules/groups
```

What `clean` looks at:

| Risk | Rules |
|---|---|
| 🟢 safe (default) | node_modules / .next / dist / … (only next to package.json), Flutter build + .dart_tool, Pods, .venv / .tox, Rust and Maven target, Gradle build (Java, Kotlin, Android), sbt target (Scala), .NET bin/obj, Swift .build, Elixir _build/deps, Haskell .stack-work, Zig caches, CLion cmake-build-*, .terraform, package manager caches (npm, Yarn, pip, Poetry, CocoaPods, Deno, Bun, Cargo, Composer, SwiftPM), Go build cache, Gradle, Xcode DerivedData, VS Code caches, browser caches (browser closed), old Playwright browsers, closed Playwright MCP profiles, crash reports > 30 days, Docker build cache + unused images, `brew cleanup`, `pnpm store prune`, `uv cache clean` |
| 🟡 `--risky` | Docker containers stopped > 7 days, Dart pub cache, Go module cache, Maven repository, NuGet cache, PHP vendor (with composer.lock), Unity Library/Temp, old Android build-tools, unavailable iOS simulators |
| ⚪ report only | WhatsApp / Telegram data, Xcode archives, simulator devices, Downloads, Trash |

Always skipped: git-tracked files, projects changed in the last 7 days (in the menu they get their
own unticked rows, so you can choose them; `--recent-days 0` on the command line), projects where a
process is running (dev server, editor terminal), `.env*`, `*.sql` / `*.dump` / `*.db`,
`.git`, `~/Documents`, `~/.ssh`, Docker volumes and Docker's disk image.
Every deletion is logged to `~/.local/state/swelo/history.jsonl`.

## Configuration

`~/.config/swelo/config.yaml` — shown and changed with `swelo config` (`config set lang tr`,
`config add roots ~/code`, `config add disable docker`, `config edit`); the menu's language switch is saved here too:

```yaml
roots: [~/Desktop, ~/code]        # where projects live (default: ~/* — every folder in home
                                  # except Library, Applications, Public, hidden, protected
                                  # and cloud-synced folders like OneDrive or Dropbox)
exclude: ["*/my-release/*"]       # never touched
recent_days: 7                    # skip projects changed this recently
disable: [browser-cache, docker]  # rule ids or groups
lang: tr                          # output language (default: the system's)
```

`~/.config/swelo/rules.yaml` adds rules or overrides built-in ones by `id`; see
[default.yaml](internal/rules/default.yaml) for the format.

## License

MIT
