package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/musacanminaz/swelo/internal/clean"
	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/perm"
	"github.com/musacanminaz/swelo/internal/rules"
	"github.com/musacanminaz/swelo/internal/ui"
	"github.com/spf13/cobra"
)

type cleanOpts struct {
	apply, risky, yes, all bool
	only                   []string
	recentDays             int
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
		Scan:     append(existing(eng, cfg.Roots), "~/Library/Caches", "~/Library/Developer", "Docker"),
		Read:     i18n.T("plan.read.clean"),
		Delete:   del,
		Perm:     i18n.T("plan.perm.none"),
		Duration: i18n.T("plan.time.clean"),
	})
	fmt.Fprintf(w, " %s\n", ui.Dim(i18n.T("scanning")))
	results := eng.Scan()
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

	if !o.apply {
		fmt.Fprintf(w, " %s\n\n", ui.Dim(i18n.T("hint.apply")))
		return nil
	}
	if !o.yes && !ui.Confirm(in, w, fmt.Sprintf(i18n.T("confirm.delete"), ui.Bytes(total)), false) {
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
			fmt.Fprintf(w, "   %s %s: %v\n", ui.Red("✗"), short(p, home), err)
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

func existing(eng *clean.Engine, roots []string) []string {
	var out []string
	for _, r := range roots {
		if _, err := os.Stat(rules.Expand(r, eng.Home)); err == nil {
			out = append(out, r)
		}
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
	if r.Rule.Kind == "command" && r.Size == 0 && len(r.Rule.SizeCmd) == 0 && len(r.Rule.SizeTextCmd) == 0 {
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

func short(p, home string) string {
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+"/") {
		return "~" + p[len(home):]
	}
	return p
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
