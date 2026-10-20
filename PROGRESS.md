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
- [ ] Translate skip reasons (currently English in TR output)
- [ ] Docker: show reclaimable size instead of raw `docker system df`
- [ ] v0.1.0 release (goreleaser, musacanminaz/homebrew-tap)
