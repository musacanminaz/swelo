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
- [ ] YAML rule engine (embedded defaults + ~/.config/swelo/rules.yaml)
- [ ] Port all temizle.sh rules, add new 🟢 rules
- [ ] Risk levels (--risky), confirmation, history log
- [ ] v0.1.0 release (goreleaser, musacanminaz/homebrew-tap)
