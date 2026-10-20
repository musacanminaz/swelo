package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/musacanminaz/swelo/internal/clean"
	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/perm"
	"github.com/musacanminaz/swelo/internal/rules"
	"github.com/musacanminaz/swelo/internal/safety"
	"github.com/musacanminaz/swelo/internal/ui"
	"github.com/spf13/cobra"
)

type cleanOpts struct {
	apply, risky, yes, all, pick bool
	only                         []string
	recentDays                   int
}

func cleanCmd() *cobra.Command {
	var o cleanOpts
	c := &cobra.Command{
		Use:   "clean",
		Short: i18n.T("cmd.clean"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runClean(bufio.NewReader(os.Stdin), cmd.OutOrStdout(), home(), o, stdinIsTerminal())
		},
	}
	f := c.Flags()
	f.BoolVar(&o.apply, "apply", false, "delete the listed items (asks first)")
	f.BoolVar(&o.risky, "risky", false, "include 🟡 rules")
	f.BoolVar(&o.yes, "yes", false, "do not ask for confirmation (never approves sudo)")
	f.BoolVar(&o.all, "all", false, "list every item, not only the largest")
	f.BoolVar(&o.pick, "pick", false, "choose what to delete interactively")
	f.StringSliceVar(&o.only, "only", nil, "only these rule ids or groups (build, cache, docker, tools)")
	f.IntVar(&o.recentDays, "recent-days", -1, "skip projects changed within N days (0 = off; default from config, 7)")
	return c
}

func runClean(in *bufio.Reader, w io.Writer, home string, o cleanOpts, interactive bool) error {
	introGate(in, w, interactive)
	rs, cfg, err := rules.Load(home)
	if err != nil {
		return err
	}
	if o.recentDays >= 0 {
		cfg.RecentDays = &o.recentDays
	}
	if len(o.only) > 0 {
		for i := range rs {
			rs[i].Disabled = !matchesOnly(rs[i], o.only)
		}
	}
	eng := clean.New(home, rs, cfg)
	eng.FDA = perm.FullDiskAccess(home)

	del := i18n.T("plan.delete.dry")
	if o.apply {
		del = i18n.T("plan.delete.apply")
	}
	ui.ShowPlan(w, ui.Plan{
		Title:    "swelo clean",
		Scan:     append(shortAll(rules.ProjectRoots(cfg, home), home), "~/Library/Caches", "~/Library/Developer", "Docker"),
		Read:     i18n.T("plan.read.clean"),
		Delete:   del,
		Perm:     i18n.T("plan.perm.none"),
		Duration: i18n.T("plan.time.clean"),
	})
	fmt.Fprintf(w, " %s\n", ui.Dim(i18n.T("scanning")))
	st := ui.NewStatus(w)
	done := watchScan(eng, st, home)
	results := eng.Scan()
	done()
	st.Stop()
	before := clean.FreeBytes(home)

	var safe, risky int64
	for _, r := range results {
		if !printResult(w, r, cfg.Top, o, home) {
			continue
		}
		switch {
		case clean.Selected(r, false):
			safe += r.Size
		case clean.Selected(r, true):
			risky += r.Size
		}
	}
	total := safe
	if o.risky {
		total += risky
	}
	fmt.Fprintf(w, "\n %s %s", ui.Bold(i18n.T("total.safe")), ui.Bold(ui.Bytes(safe)))
	if risky > 0 {
		fmt.Fprintf(w, "   %s %s", i18n.T("total.risky"), ui.Bytes(risky))
	}
	fmt.Fprintf(w, "   %s %s\n", i18n.T("disk.free"), ui.Bytes(before))

	if o.pick {
		var ok, recent bool
		if results, total, recent, ok = pickResults(results, eng.Guard.RecentDays); !ok {
			fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
			return nil
		}
		if len(results) == 0 {
			fmt.Fprintf(w, " %s\n\n", i18n.T("pick.none"))
			return nil
		}
		o.risky = true // risky rules in results were ticked by the user
		if recent {
			eng.Guard.RecentDays = 0 // the user ticked recently changed projects; other checks still apply
		}
	} else if !o.apply {
		fmt.Fprintf(w, " %s\n\n", ui.Dim(i18n.T("hint.apply")))
		return nil
	}
	if !o.yes && !ask(in, w, fmt.Sprintf(i18n.T("confirm.delete"), ui.Bytes(total)), o.pick) {
		fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
		return nil
	}
	var failed int
	freed := eng.Apply(results, o.risky, func(r clean.Result, p string, size int64, err error) {
		switch {
		case r.Rule.Kind == "command" && err != nil:
			failed++
			fmt.Fprintf(w, "   %s %s: %v\n%s\n", ui.Red("✗"), r.Rule.ID, err, ui.Indent(lastLines(p, 3), "     "))
		case r.Rule.Kind == "command":
			fmt.Fprintf(w, "   %s %s  %s\n", ui.Green("✓"), r.Rule.ID, ui.Dim(lastLines(p, 1)))
		case err != nil:
			failed++
			fmt.Fprintf(w, "   %s %s: %s\n", ui.Red("✗"), short(p, home), ui.Safe(err.Error()))
		}
	})
	after := clean.FreeBytes(home)
	fmt.Fprintf(w, "\n %s %s   %s %s → %s\n", ui.Bold(i18n.T("done.freed")), ui.Bold(ui.Bytes(freed)),
		i18n.T("disk.free"), ui.Bytes(before), ui.Bytes(after))
	if failed > 0 {
		fmt.Fprintf(w, " %s\n", ui.Yellow(fmt.Sprintf(i18n.T("done.failed"), failed)))
	}
	fmt.Fprintf(w, " %s %s\n\n", ui.Dim(i18n.T("done.log")), filepath.Join(stateDir(), "history.jsonl"))
	return nil
}

func matchesOnly(r rules.Rule, only []string) bool {
	for _, o := range only {
		if o == r.ID || o == r.Group {
			return true
		}
	}
	return false
}

func shortAll(paths []string, home string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = short(p, home)
	}
	return out
}

// printResult prints one rule's findings; returns false if there was nothing to show.
func printResult(w io.Writer, r clean.Result, top int, o cleanOpts, home string) bool {
	if r.Rule.Disabled || strings.HasSuffix(r.Note, "not installed") {
		return false
	}
	if len(r.Items) == 0 && len(r.Skips) == 0 && r.Note == "" && r.Show == "" && r.Size == 0 {
		return false
	}
	mark, tag := ui.Green("●"), ""
	switch r.Rule.Risk {
	case rules.Risky:
		mark = ui.Yellow("●")
		if !o.risky {
			tag = ui.Dim(" " + i18n.T("tag.risky"))
		}
	case rules.Report:
		mark, tag = ui.Dim("○"), ui.Dim(" "+i18n.T("tag.report"))
	}
	count := ""
	if len(r.Items) > 0 {
		count = fmt.Sprintf(" · %d", len(r.Items))
	}
	size := ui.Bytes(r.Size)
	if !sizeKnown(r) {
		size = "" // size unknown until the command runs
	}
	fmt.Fprintf(w, "\n %s %s%s — %s  %s%s\n", mark, ui.Bold(r.Rule.ID), tag, r.Rule.Description(i18n.Lang()),
		ui.Bold(size), ui.Dim(count))
	if r.Note != "" {
		fmt.Fprintf(w, "     %s\n", ui.Dim(i18n.Reason(r.Note)))
	}
	if r.Show != "" {
		show := r.Show
		if lines := strings.Split(show, "\n"); !o.all && len(lines) > 6 {
			show = strings.Join(lines[:6], "\n") + "\n" + fmt.Sprintf(i18n.T("more.items"), len(lines)-6)
		}
		fmt.Fprintln(w, ui.Dim(ui.Indent(show, "     ")))
	}
	n := len(r.Items)
	if !o.all && n > top {
		n = top
	}
	for _, it := range r.Items[:n] {
		fmt.Fprintf(w, "     %9s  %s\n", ui.Bytes(it.Size), short(it.Path, home))
	}
	if rest := len(r.Items) - n; rest > 0 {
		fmt.Fprintf(w, "     %s\n", ui.Dim(fmt.Sprintf(i18n.T("more.items"), rest)))
	}
	if len(r.Skips) > 0 {
		fmt.Fprintf(w, "     %s\n", ui.Dim(skipSummary(r.Skips, o.all, home)))
	}
	if r.Rule.Rebuild != "" && len(r.Items) > 0 {
		fmt.Fprintf(w, "     %s %s\n", ui.Dim(i18n.T("rebuild")), ui.Dim(r.Rule.Rebuild))
	}
	return true
}

func skipSummary(skips []clean.Item, all bool, home string) string {
	if all {
		var lines []string
		for _, s := range skips {
			lines = append(lines, fmt.Sprintf("%s %s (%s)", i18n.T("skipped"), short(s.Path, home), i18n.Reason(s.Skip)))
		}
		return strings.Join(lines, "\n     ")
	}
	counts := map[string]int{}
	for _, s := range skips {
		counts[i18n.Reason(s.Skip)]++
	}
	var parts []string
	for reason, c := range counts {
		parts = append(parts, fmt.Sprintf("%s ×%d", reason, c))
	}
	sort.Strings(parts)
	return fmt.Sprintf("%s %d: %s", i18n.T("skipped"), len(skips), strings.Join(parts, ", "))
}

// short abbreviates home as ~ and makes the path safe to print.
func short(p, home string) string {
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+"/") {
		p = "~" + p[len(home):]
	}
	return ui.Safe(p)
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// pickResults lets the user tick rules (🟢 ticked, 🟡 and recently changed
// projects not) and returns the ticked results with their total; recent is
// true if recently changed projects were ticked, ok false if cancelled.
func pickResults(results []clean.Result, recentDays int) (chosen []clean.Result, total int64, recent, ok bool) {
	items, selected, resolve := cleanRows(results, recentDays)
	pk := ui.NewPicker(i18n.T("pick.clean"), items)
	copy(pk.Selected, selected)
	if _, err := tea.NewProgram(pk, tea.WithAltScreen()).Run(); err != nil || pk.Cancelled {
		return nil, 0, false, false
	}
	chosen, total, recent = resolve(pk.Selected)
	return chosen, total, recent, true
}

// cleanRows builds the pick list: one row per rule with something to free,
// then one row per rule for projects skipped only for being recent.
func cleanRows(results []clean.Result, recentDays int) (items []ui.PickItem, selected []bool,
	resolve func([]bool) ([]clean.Result, int64, bool)) {
	var idx []int
	for _, risk := range []string{rules.Safe, rules.Risky} {
		for i, r := range results {
			command := r.Rule.Kind == "command" && r.Note == "" && (r.Size > 0 || !sizeKnown(r)) // measured 0 B: nothing to free
			if r.Rule.Risk == risk && clean.Selected(r, true) && (len(r.Items) > 0 || command) {
				idx = append(idx, i)
			}
		}
	}
	items = make([]ui.PickItem, len(idx))
	for n, i := range idx {
		r := results[i]
		size := ui.Bytes(r.Size)
		if !sizeKnown(r) {
			size = "?"
		}
		items[n] = ui.PickItem{Group: i18n.T("group." + r.Rule.Risk), Label: fmt.Sprintf("%9s  %s", size, r.Rule.ID), Detail: r.Rule.Description(i18n.Lang())}
	}
	// one more row per rule for the projects skipped only for being recent
	recentItems := map[int][]clean.Item{}
	var recentIdx []int
	for i, r := range results {
		if !clean.Selected(r, true) {
			continue
		}
		var size int64
		var names []string
		seen := map[string]bool{}
		for _, it := range r.Skips {
			if safety.IsRecent(it.Skip) {
				it.Skip = ""
				recentItems[i] = append(recentItems[i], it)
				size += it.Size
				if !seen[it.Project] { // "repo/ios": a project's own name is often just ios or web
					seen[it.Project] = true
					names = append(names, filepath.Join(filepath.Base(filepath.Dir(it.Project)), filepath.Base(it.Project)))
				}
			}
		}
		if len(names) > 0 {
			recentIdx = append(recentIdx, i)
			items = append(items, ui.PickItem{Group: fmt.Sprintf(i18n.T("group.recent"), recentDays),
				Label:  fmt.Sprintf("%9s  %s", ui.Bytes(size), r.Rule.ID),
				Detail: fmt.Sprintf(i18n.T("pick.recent"), len(names), ui.Safe(strings.Join(names, ", ")))})
		}
	}
	selected = make([]bool, len(items))
	for n, i := range idx {
		selected[n] = clean.Selected(results[i], false)
	}
	resolve = func(selected []bool) (chosen []clean.Result, total int64, recent bool) {
		return resolveRows(results, idx, recentIdx, recentItems, selected)
	}
	return items, selected, resolve
}

func resolveRows(results []clean.Result, idx, recentIdx []int, recentItems map[int][]clean.Item, selected []bool) (chosen []clean.Result, total int64, recent bool) {
	picked := map[int]*clean.Result{}
	var order []int
	take := func(i int) *clean.Result {
		if picked[i] == nil {
			r := results[i]
			r.Items, r.Size = nil, 0
			picked[i] = &r
			order = append(order, i)
		}
		return picked[i]
	}
	for n, i := range idx {
		if selected[n] {
			r := take(i)
			r.Items, r.Size = append([]clean.Item(nil), results[i].Items...), results[i].Size
		}
	}
	for n, i := range recentIdx {
		if selected[len(idx)+n] {
			r := take(i)
			for _, it := range recentItems[i] {
				r.Items = append(r.Items, it)
				r.Size += it.Size
			}
			recent = true
		}
	}
	for _, i := range order {
		chosen = append(chosen, *picked[i])
		total += picked[i].Size
	}
	return chosen, total, recent
}

// ask is a yes/no question: a menu when picking, typed y/N otherwise.
func ask(in *bufio.Reader, w io.Writer, q string, pick bool) bool {
	if !pick {
		return ui.Confirm(in, w, q, false)
	}
	fmt.Fprintln(w)
	return ui.Choose(q, []ui.PickItem{{Label: i18n.T("choice.no")}, {Label: i18n.T("choice.yes")}}) == 1
}

// sizeKnown: false for a command rule that cannot say what it would free.
func sizeKnown(r clean.Result) bool {
	return r.Rule.Kind != "command" || r.Size > 0 || len(r.Rule.SizeCmd) > 0 || len(r.Rule.SizeTextCmd) > 0
}
