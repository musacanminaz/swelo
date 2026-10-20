package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/musacanminaz/swelo/internal/clean"
	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/optimize"
	"github.com/musacanminaz/swelo/internal/perm"
	"github.com/musacanminaz/swelo/internal/rules"
	"github.com/musacanminaz/swelo/internal/ui"
	"github.com/spf13/cobra"
)

func optimizeCmd() *cobra.Command {
	var apply, yes bool
	var only []string
	c := &cobra.Command{
		Use:   "optimize",
		Short: i18n.T("cmd.optimize"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runOptimize(bufio.NewReader(os.Stdin), cmd.OutOrStdout(), home(), apply, yes, only)
		},
	}
	c.Flags().BoolVar(&apply, "apply", false, "run the listed tasks (asks first)")
	c.Flags().BoolVar(&yes, "yes", false, "do not ask for confirmation")
	c.Flags().StringSliceVar(&only, "only", nil, "tasks: timemachine, git-gc, brew-autoremove")
	return c
}

func runOptimize(in *bufio.Reader, w io.Writer, home string, apply, yes bool, only []string) error {
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
		Title: "swelo optimize", Scan: existingRoots(cfg.Roots, home),
		Read: i18n.T("plan.read.optimize"), Delete: del, Perm: i18n.T("plan.perm.none"),
		Duration: i18n.T("plan.time.clean"),
	})
	fmt.Fprintf(w, " %s\n", ui.Dim(i18n.T("scanning")))

	eng := clean.New(home, rs, cfg)
	eng.FDA = perm.FullDiskAccess(home)
	results := eng.Scan()
	var regen []string
	skip := map[string]bool{}
	for _, r := range results {
		for _, n := range r.Rule.Names {
			skip[n] = true
		}
		if r.Rule.Risk == rules.Report || r.Rule.Kind == "command" {
			continue
		}
		for _, it := range append(append([]clean.Item{}, r.Items...), r.Skips...) {
			regen = append(regen, it.Path)
		}
	}
	var roots []string
	for _, r := range cfg.Roots {
		roots = append(roots, rules.Expand(r, home))
	}
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
	if !apply || ready == 0 {
		if ready > 0 {
			fmt.Fprintf(w, " %s\n\n", ui.Dim(i18n.T("opt.hint")))
		}
		return nil
	}
	if !yes && !ui.Confirm(in, w, fmt.Sprintf(i18n.T("opt.confirm"), ready), false) {
		fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
		return nil
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

func existingRoots(roots []string, home string) []string {
	var out []string
	for _, r := range roots {
		if _, err := os.Stat(rules.Expand(r, home)); err == nil {
			out = append(out, r)
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
