# Progress

Plan: phases 1–7 (intro & safety → clean → analyze → uninstall → optimize/status → release → homebrew-core).

## Phase 1 — skeleton, intro, permissions, safety
- [x] Go module, cobra root, TR/EN messages (`internal/i18n`)
- [x] Intro screen (`swelo`): what it does / never does / permission status / commands
- [x] `swelo permissions`: Full Disk Access + admin detection, every permission explained
- [x] `internal/safety`: single deletion gate (roots, protected paths, symlink escape, git-tracked, recent project, .env/.sql, log) + tests
- [x] CI (macOS: gofmt, vet, test, build), MIT license, README
- [x] First-run intro gate (`introGate`, shown once before the first scan, Enter to continue; `swelo --intro` re-arms) — wired into scanning commands in phase 2
- [x] "Plan box" helper (`ui.ShowPlan`) + `ui.Confirm` (y/e/evet, default on empty/EOF)
- [x] sudo step dialog (`ui.RunSudo`): exact command, reason, undo; [y/N]; --no-sudo; `sudo -k` afterwards; never auto-approved

## Phase 2 — clean
- [x] YAML rule engine (`internal/rules/default.yaml` embedded; user rules.yaml overrides by id; config.yaml roots/exclude/protected/recent_days/disable/top)
- [x] Ported temizle.sh rules + new: python, cocoapods, rust, terraform, project .gradle, pip/CocoaPods caches, browser caches, Playwright browsers (keep newest), crash reports, brew, uv, risky pub-cache / android build-tools / old containers / unavailable simulators, report-only big user data
- [x] Risk levels (--risky), confirmation before delete, history log, --only, --all, --recent-days
- [x] Per-project busy check (a process has its cwd in the project) instead of global "vite is running"
- [x] Playwright MCP: a SingletonLock is only "in use" if its PID is alive
- [x] Engine tests on a fake home; tests never run command rules or touch absolute paths
- [x] Skip reasons translated (`i18n.Reason`)
- [x] Docker: reclaimable build cache size via `size_text_cmd`
- [x] Security: git runs with fsmonitor/hooks disabled (hostile .git/config cannot execute); roots outside home must be owned by the user, not symlinks, not group/world-writable
- [x] v0.1.0 release: tag → release workflow (darwin arm64/amd64 tarballs + checksums); tap formula builds from source (same shape homebrew-core needs)

**v0.1.0 released 2026-10-20** — https://github.com/musacanminaz/swelo/releases/tag/v0.1.0
Tap: https://github.com/musacanminaz/homebrew-tap (`brew install musacanminaz/tap/swelo`);
`brew audit --strict --online` and `brew test` pass. To bump: update url + sha256 in
Formula/swelo.rb after tagging.

## Phase 3 — analyze
- [x] Category summary in one parallel walk of ~ (≈30 s here) with "clean can free" column; Docker.raw allocated vs max
- [x] Largest single files (> 1 GB)
- [x] Projects list: regenerable size, last change (> 90 days highlighted)
- [x] macOS-protected folders (Containers, Group Containers, Mail, Messages, Trash…) are never read without --full + Full Disk Access, in analyze and clean, so no privacy dialogs pop up
- [x] Test: every i18n key used in code exists
- [x] TUI tree: `swelo analyze --tree [folder]` (bubbletea); d → confirm → macOS `/usr/bin/trash` (fallback: rename into ~/.Trash); refuses home, scan root, outside home, ~/Library and its direct children, protected folders
- [x] Real TTY detection via ioctl (/dev/null is not a terminal)
- [x] `--full` uses Full Disk Access folders

## Phase 4 — uninstall
- [x] Providers (list, read-only): simulator runtimes (real last-used date), Android SDK (sdkmanager ids), Homebrew leaves + casks, nvm Node, pyenv Python, rustup, JDKs (sudo), global npm, Docker images (container use), VS Code/Cursor extensions, Ollama
- [x] "Pinned by" from .nvmrc / .node-version / .python-version / .tool-versions in project roots; nvm/rustup defaults flagged
- [x] `swelo uninstall [--only …]` listing; provider tests with a fake Env (no real commands)
- [x] `--pick` (bubbletea multi-select) and `--remove <id,...>`; review lists every exact command + warnings (pinned by projects, defaults, containers), one confirmation, sudo steps asked separately via ui.RunSudo (`--yes` never approves them, `--no-sudo` skips), commands run attached to the terminal, every attempt logged
- [x] JDKs installed by a Homebrew cask are removed with `brew uninstall --cask` (not rm) and listed once
- [x] Apps: /Applications + ~/Applications with ~/Library leftovers by bundle id (Containers/Group Containers only with Full Disk Access; protected paths such as Docker's disk image never included); moved to the Trash; cask apps via `brew uninstall --cask`; running apps, root-owned apps and Apple apps not removable
## Phase 5 — optimize, status, history
- [x] `swelo optimize` (dry run; --apply): Time Machine sticky exclusions for regenerable folders (skipped when TM is not set up), `git gc` + `worktree prune` on repos with .git ≥ 200 MB (locked/busy repos skipped, repo hooks/fsmonitor disabled), `brew autoremove`
- Dropped from the plan after checking: Spotlight `.metadata_never_index` (not honored in subfolders on current macOS), Docker.raw compaction (no verified method for current Docker Desktop — `analyze` reports allocated vs max instead)
- [x] `swelo status`: disk (colored at 80/90%), Docker.raw, free-space trend from hourly snapshots (shown once ≥ 1 day old), cleanups per day
- [x] `swelo history [-n N]`: every deletion/uninstall with time, size, rule, path
- [x] optimize hardening: repos with command-running config keys or another owner are skipped; TM exclusions only for guard-approved items
## Phase 6 — release polish
- [x] SECURITY.md (private GitHub advisories enabled); docs corrected: swelo makes no network connections
- [x] v0.2.0 released (all commands); tap bumped; `brew upgrade`, `brew test`, `brew audit --strict --online` pass
- [x] v0.3.0 released: interactive menu (`swelo`), `--pick` for clean/optimize, live scan progress, whole-home project scan (`~/*`); tap bumped, brew test + audit pass
- [x] v0.4.0 / v0.4.1 released: admin rights from the menu (isolated child processes, terminal neighbours listed), more language rules, opt-in recent projects, measured docker/simulator rules; release workflow moved to Linux runners
- [ ] Tap auto-bump on release — needs a fine-grained token with write access to musacanminaz/homebrew-tap stored as a repo secret (owner decision); until then bump url + sha256 by hand
- [ ] README screenshot/GIF (needs a recording on a real terminal)
## Phase 7 — homebrew-core
Waiting on notability: ≥75 stars or ≥30 forks/watchers, and the repo older than 30 days (eligible from 2026-11-19).
Status on 2026-10-20: 0 stars, 0 forks, 0 watchers, created 2026-10-20.
When eligible: `brew create`-style formula (the tap formula already builds from source with a test), `brew audit --new --strict --online swelo`, PR to Homebrew/homebrew-core.
## Phase 8 — user experience (v0.5.0)
Remove/simplify:
- [x] clean output: rules with nothing to delete are one line (`● flutter — … · skipped 5: …`); rules with no findings at all are counted at the end
- [x] plan box "Scan" line: `~ (16 project folders), ~/Library/Caches, …` (listed when ≤ 3 roots)
- [x] uninstall: no `id:` lines; `--remove` accepts names (case-insensitive, ".app" optional) or ids; ambiguous names ask for the id
- [x] `--intro` flag removed → menu Permissions → "Show the introduction again"
- [x] intro text fixed (confirmation instead of --apply; recent projects skipped unless ticked)
- [x] status trend sentence ("3 days ago 91.1 GB was free, now 53.0 GB (−38.1 GB)"); `completion`/`help` hidden in help. Analyze header checked: it is right-aligned over the numbers, no change
Add:
- [x] picker: a rule row expands with → (← closes) into its items; single items can be unticked ([-] = some), sizes shown per row and summed in the header
- [x] picker keys: a all, n none (visible rows), / filter on label/detail/group (Enter keeps, Esc clears), PgUp/PgDn/Home/End
- [x] uninstall --pick without --only asks the kind first (Everything / apps / Homebrew / …); only that provider is scanned
- [x] menu "Quick clean": safe rules, one menu confirmation (default No), no picker
- [x] menu header: last cleanup ("last cleanup 2 days ago · 5.1 GB")
## Phase 9 — CLI style (v0.6.0)
- [ ] `--json` for clean, analyze, uninstall, status; exit codes 0 ok / 1 error / 2 cancelled
- [ ] `swelo rules`; `swelo clean node docker` shortcut for --only
- [ ] `swelo config` (set lang, roots add, edit); menu language choice saved
- [ ] shell completions installed by the brew formula
- [ ] flag descriptions localized (--lang tr)
## Phase 10 — speed (v0.6.1)
- [ ] optimize scans artifacts only when Time Machine is set up
- [ ] sizes measured once per menu session (analyze → clean), reset after deletions
