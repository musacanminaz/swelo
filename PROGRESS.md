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
- [ ] Category summary (projects, package caches, SDKs, Docker.raw, browsers, messaging, Downloads, Trash) with "clean can free" column
- [ ] Largest single files (> 1 GB)
- [ ] Projects list: last change, artifact size, untouched > 90 days
- [ ] TUI tree (ncdu-like), delete = move to Trash
- [ ] `--full` uses Full Disk Access folders

## Phase 4 — uninstall
## Phase 5 — optimize, status, history
## Phase 6 — release polish (README GIF, SECURITY.md, tap auto-bump)
## Phase 7 — homebrew-core (needs ≥75 stars or 30 forks/watchers, repo > 30 days old)
