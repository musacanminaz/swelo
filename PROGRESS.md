# Progress

Plan: phases 1–7 (intro & safety → clean → analyze → uninstall → optimize/status → release → homebrew-core).

## Phase 1 — skeleton, intro, permissions, safety
- [x] Go module, cobra root, TR/EN messages (`internal/i18n`)
- [x] Intro screen (`swelo`): what it does / never does / permission status / commands
- [x] `swelo permissions`: Full Disk Access + admin detection, every permission explained
- [x] `internal/safety`: single deletion gate (roots, protected paths, symlink escape, git-tracked, recent project, .env/.sql, log) + tests
- [x] CI (macOS: gofmt, vet, test, build), MIT license, README
- [ ] First-run intro gate (shown once before the first scan, Enter to continue)
- [ ] "Plan box" helper shown before every command
- [ ] sudo step helper (show command, reason, undo; ask [y/N]; --no-sudo; --yes never approves)

## Phase 2 — clean
- [ ] YAML rule engine (embedded defaults + ~/.config/swelo/rules.yaml)
- [ ] Port all temizle.sh rules, add new 🟢 rules
- [ ] Risk levels (--risky), confirmation, history log
- [ ] v0.1.0 release (goreleaser, musacanminaz/homebrew-tap)
