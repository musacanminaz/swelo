package cli

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/musacanminaz/swelo/internal/analyze"
	"github.com/musacanminaz/swelo/internal/clean"
	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/perm"
	"github.com/musacanminaz/swelo/internal/rules"
	"github.com/musacanminaz/swelo/internal/state"
	"github.com/musacanminaz/swelo/internal/tree"
	"github.com/musacanminaz/swelo/internal/ui"
)

// dupeMinSize: smaller duplicates are not worth the hashing.
const dupeMinSize = 50 << 20

// runDupes finds identical files of 50 MB or more under dir and reports
// them; with pick, copies can be ticked and moved to the Trash (never the
// last copy of a group).
func runDupes(in *bufio.Reader, w io.Writer, home, dir string, full, pick bool, jsonOut io.Writer) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	_, cfg, err := rules.Load(home)
	if err != nil {
		return err
	}
	protected := perm.ProtectedDirs(home)
	var skip []string
	if !full || !perm.FullDiskAccess(home) {
		skip = protected
	}
	for _, p := range cfg.Protected { // never offered for the Trash, so not worth hashing
		skip = append(skip, rules.Expand(p, home))
	}
	del := i18n.T("plan.delete.none")
	if pick {
		del = i18n.T("plan.delete.dupes")
	}
	ui.ShowPlan(w, ui.Plan{
		Title: "swelo analyze --dupes", Scan: []string{short(abs, home)}, Read: i18n.T("plan.read.dupes"),
		Delete: del, Perm: i18n.T("plan.perm.analyze"), Duration: i18n.T("plan.time.dupes"),
	})
	fmt.Fprintf(w, " %s\n", ui.Dim(i18n.T("scanning")))
	st := ui.NewStatus(w)
	groups := analyze.FindDupes(abs, analyze.DupeOptions{MinSize: dupeMinSize,
		Skip: func(d string) bool {
			for _, s := range skip {
				if d == s || strings.HasPrefix(d, s+"/") {
					return true
				}
			}
			return filepath.Base(d) == ".git" || filepath.Base(d) == "node_modules"
		},
		Progress: func(files, hashing int) {
			if hashing == 0 {
				st.Set(i18n.T("prog.dupes.files"), files)
			} else {
				st.Set(i18n.T("prog.dupes.hash"), hashing)
			}
		}})
	st.Stop()
	if jsonOut != nil {
		return writeJSON(jsonOut, struct {
			Root   string          `json:"root"`
			Groups []analyze.Group `json:"groups"`
		}{abs, groups})
	}
	var wasted int64
	for _, g := range groups {
		wasted += g.Wasted()
	}
	if len(groups) == 0 {
		fmt.Fprintf(w, "\n %s\n\n", i18n.T("dupes.none"))
		return nil
	}
	fmt.Fprintf(w, "\n %s %s\n", ui.Bold(fmt.Sprintf(i18n.T("dupes.found"), len(groups), ui.Bytes(wasted))), ui.Dim(i18n.T("dupes.note")))
	for _, g := range groups {
		fmt.Fprintf(w, "\n   %9s × %d\n", ui.Bytes(g.Size), len(g.Files))
		for _, f := range g.Files {
			fmt.Fprintf(w, "     %s\n", short(f, home))
		}
	}
	fmt.Fprintln(w)
	if !pick {
		fmt.Fprintf(w, " %s\n\n", ui.Dim(i18n.T("dupes.hint")))
		return nil
	}
	return trashDupes(in, w, home, abs, groups, protected)
}

// trashDupes: one pick row per group with the copies as children, nothing
// ticked; a group's last copy can never be trashed.
func trashDupes(in *bufio.Reader, w io.Writer, home, root string, groups []analyze.Group, protected []string) error {
	items := make([]ui.PickItem, len(groups))
	for i, g := range groups {
		items[i] = ui.PickItem{Label: fmt.Sprintf("%s × %d", ui.Bytes(g.Size), len(g.Files)), Detail: short(filepath.Dir(g.Files[0]), home)}
		for _, f := range g.Files {
			items[i].Children = append(items[i].Children, ui.PickItem{Label: short(f, home), Size: g.Size})
		}
	}
	pk := ui.NewPicker(i18n.T("pick.dupes"), items)
	if _, err := tea.NewProgram(pk, tea.WithAltScreen()).Run(); err != nil || pk.Cancelled {
		fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
		return ErrCancelled
	}
	var chosen []string
	var total int64
	for i, g := range groups {
		n := 0
		for _, v := range pk.Sub[i] {
			if v {
				n++
			}
		}
		if n == len(g.Files) {
			fmt.Fprintf(w, " %s %s\n", ui.Yellow("!"), fmt.Sprintf(i18n.T("dupes.keepone"), short(g.Files[0], home)))
			pk.Sub[i][0] = false
		}
		for j, v := range pk.Sub[i] {
			if v {
				chosen = append(chosen, g.Files[j])
				total += g.Size
			}
		}
	}
	if len(chosen) == 0 {
		fmt.Fprintf(w, " %s\n\n", i18n.T("pick.none"))
		return nil
	}
	if !ask(in, w, fmt.Sprintf(i18n.T("dupes.confirm"), len(chosen), ui.Bytes(total)), true) {
		fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
		return ErrCancelled
	}
	logPath := filepath.Join(stateDir(), "history.jsonl")
	var freed int64
	for _, f := range chosen {
		if why := tree.CanTrash(f, root, home, protected); why != "" {
			fmt.Fprintf(w, "   %s %s: %s\n", ui.Red("✗"), short(f, home), i18n.Reason(why))
			continue
		}
		size := clean.DirSize(f)
		if err := tree.Trash(f, home); err != nil {
			fmt.Fprintf(w, "   %s %s: %s\n", ui.Red("✗"), short(f, home), ui.Safe(err.Error()))
			continue
		}
		freed += size
		state.Log(logPath, state.Record{Path: f, Rule: "dupes", Bytes: size})
		fmt.Fprintf(w, "   %s %s\n", ui.Green("✓"), short(f, home))
	}
	fmt.Fprintf(w, "\n %s\n\n", fmt.Sprintf(i18n.T("tree.summary"), ui.Bytes(freed)))
	return nil
}
