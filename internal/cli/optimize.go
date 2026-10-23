package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/musacanminaz/swelo/internal/clean"
	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/optimize"
	"github.com/musacanminaz/swelo/internal/perm"
	"github.com/musacanminaz/swelo/internal/rules"
	"github.com/musacanminaz/swelo/internal/ui"
	"github.com/spf13/cobra"
)

func optimizeCmd() *cobra.Command {
	var apply, yes, pick bool
	var only []string
	c := &cobra.Command{
		Use:   "optimize",
		Short: i18n.T("cmd.optimize"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runOptimize(bufio.NewReader(os.Stdin), cmd.OutOrStdout(), home(), apply, yes, pick, only)
		},
	}
	c.Flags().BoolVar(&apply, "apply", false, i18n.T("flag.optimize.apply"))
	c.Flags().BoolVar(&yes, "yes", false, i18n.T("flag.optimize.yes"))
	c.Flags().BoolVar(&pick, "pick", false, i18n.T("flag.optimize.pick"))
	c.Flags().StringSliceVar(&only, "only", nil, i18n.T("flag.optimize.only"))
	return c
}

func runOptimize(in *bufio.Reader, w io.Writer, home string, apply, yes, pick bool, only []string) error {
	introGate(in, w, stdinIsTerminal())
	rs, cfg, err := rules.Load(home)
	if err != nil {
		return err
	}
	del := i18n.T("plan.delete.optimize.dry")
	if apply {
		del = i18n.T("plan.delete.optimize")
	}
	ui.ShowPlan(w, ui.Plan{
		Title: "swelo optimize", Scan: rootsLabel(rules.ProjectRoots(cfg, home), home),
		Read: i18n.T("plan.read.optimize"), Delete: del, Perm: i18n.T("plan.perm.none"),
		Duration: i18n.T("plan.time.clean"),
	})
	fmt.Fprintf(w, " %s\n", ui.Dim(i18n.T("scanning")))

	eng := clean.New(home, rs, cfg)
	eng.FDA = perm.FullDiskAccess(home)
	st := ui.NewStatus(w)
	done := watchScan(eng, st, home)
	results := eng.Scan()
	done()
	st.Stop()
	var regen []string
	skip := map[string]bool{}
	for _, r := range results {
		for _, n := range r.Rule.Names {
			skip[n] = true
		}
		if r.Rule.Risk == rules.Report || r.Rule.Kind == "command" {
			continue
		}
		for _, it := range r.Items { // only what passed the safety gate, never skipped items
			if eng.Guard.Check(it.Path, it.Project) == nil {
				regen = append(regen, it.Path)
			}
		}
	}
	roots := rules.ProjectRoots(cfg, home)
	have := func(n string) bool { _, err := exec.LookPath(n); return err == nil }

	var tasks []optimize.Task
	want := func(id string) bool { return len(only) == 0 || contains(only, id) }
	if want("timemachine") {
		tasks = append(tasks, optimize.TimeMachine(optimize.Real, regen))
	}
	if want("git-gc") {
		tasks = append(tasks, optimize.Git(optimize.Real, optimize.FindRepos(roots, skip), 200e6, eng.Busy))
	}
	if want("brew-autoremove") {
		tasks = append(tasks, optimize.BrewAutoremove(optimize.Real, have))
	}

	ready := 0
	for _, t := range tasks {
		mark := ui.Green("●")
		if !t.Ready {
			mark = ui.Dim("○")
		} else {
			ready++
		}
		fmt.Fprintf(w, "\n %s %s — %s\n", mark, ui.Bold(t.ID), i18n.T("opt."+t.ID))
		if t.Note != "" {
			fmt.Fprintf(w, "     %s\n", ui.Dim(i18n.Reason(t.Note)))
		}
		if len(t.Items) > 0 {
			fmt.Fprintf(w, "     %s\n", fmt.Sprintf(i18n.T("opt.items."+t.ID), len(t.Items), ui.Bytes(t.Size)))
			for i, it := range t.Items {
				if i == 5 {
					fmt.Fprintf(w, "     %s\n", ui.Dim(fmt.Sprintf(i18n.T("more.items"), len(t.Items)-5)))
					break
				}
				fmt.Fprintf(w, "       %s\n", ui.Dim(short(it, home)))
			}
		} else if t.Note == "" {
			fmt.Fprintf(w, "     %s\n", ui.Dim(i18n.T("opt.nothing")))
		}
		fmt.Fprintf(w, "     %s\n", ui.Dim(i18n.T("opt.undo."+t.ID)))
	}
	fmt.Fprintln(w)
	if pick && ready > 0 {
		var items []ui.PickItem
		var idx []int
		for i, t := range tasks {
			if t.Ready {
				idx = append(idx, i)
				items = append(items, ui.PickItem{Label: t.ID, Detail: i18n.T("opt." + t.ID)})
			}
		}
		pk := ui.NewPicker(i18n.T("pick.optimize"), items)
		for n := range pk.Selected {
			pk.Selected[n] = true
		}
		if _, err := tea.NewProgram(pk, tea.WithAltScreen()).Run(); err != nil || pk.Cancelled {
			fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
			return ErrCancelled
		}
		ready = 0
		for n, i := range idx {
			tasks[i].Ready = pk.Selected[n]
			if pk.Selected[n] {
				ready++
			}
		}
		apply = true
	}
	if !apply || ready == 0 {
		if ready > 0 {
			fmt.Fprintf(w, " %s\n\n", ui.Dim(i18n.T("opt.hint")))
		}
		return nil
	}
	if !yes && !ask(in, w, fmt.Sprintf(i18n.T("opt.confirm"), ready), pick) {
		fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
		return ErrCancelled
	}
	for _, t := range tasks {
		if !t.Ready {
			continue
		}
		out, err := t.Apply()
		if err != nil {
			fmt.Fprintf(w, "   %s %s: %s\n%s\n", ui.Red("✗"), t.ID, ui.Safe(err.Error()), ui.Indent(ui.Safe(lastLines(out, 3)), "     "))
			continue
		}
		fmt.Fprintf(w, "   %s %s  %s\n", ui.Green("✓"), t.ID, ui.Dim(ui.Safe(lastLines(out, 1))))
	}
	fmt.Fprintln(w)
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
