package cli

import (
	"github.com/musacanminaz/swelo/internal/clean"
	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/ui"
)

// watchScan shows what eng.Scan is doing on st: the rule, folder or item in
// work on the live line, and a ✓ line with counts as each stage ends. Call
// the returned func after Scan to close the last stage.
func watchScan(eng *clean.Engine, st *ui.Status, home string) func() {
	var last clean.Progress
	rules := 0
	finish := func() {
		switch last.Stage {
		case "rule":
			st.Step(i18n.T("step.rules"), rules)
		case "walk":
			st.Step(i18n.T("step.walk"), last.N, last.Found)
		case "measure":
			st.Step(i18n.T("step.measure"), last.Total)
		case "check":
			st.Step(i18n.T("step.check"), last.Total)
		}
	}
	eng.OnProgress = func(p clean.Progress) {
		if p.Stage != last.Stage {
			finish()
		}
		last = p
		switch p.Stage {
		case "rule":
			rules++
			desc := ""
			for _, r := range eng.Rules {
				if r.ID == p.Detail {
					desc = r.Description(i18n.Lang())
				}
			}
			st.Set(i18n.T("prog.rule"), p.N, p.Total, p.Detail, desc)
		case "walk":
			st.Set(i18n.T("prog.walk"), p.N, p.Found, short(p.Detail, home))
		case "measure":
			st.Set(i18n.T("prog.measure"), p.N, p.Total, short(p.Detail, home))
		case "check":
			st.Set(i18n.T("prog.check"), p.N, p.Total, short(p.Detail, home))
		}
	}
	return func() {
		eng.OnProgress = nil
		if last.Stage != "" {
			finish()
		}
	}
}

// watchDirs returns a progress func for analyze and tree scans.
func watchDirs(st *ui.Status, home string) func(dir string, dirs, files, bytes int64) {
	return func(dir string, dirs, files, bytes int64) {
		st.Set(i18n.T("prog.dirs"), dirs, files, ui.Bytes(bytes), short(dir, home))
	}
}
