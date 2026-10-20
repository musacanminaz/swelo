package cli

import (
	"bufio"
	"errors"
	"os/exec"
	"path/filepath"

	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"io"
	"os"
	"strings"
	"time"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/rules"
	"github.com/musacanminaz/swelo/internal/ui"
	"github.com/musacanminaz/swelo/internal/uninstall"
	"github.com/spf13/cobra"
)

type uninstallOpts struct {
	only, remove      []string
	pick, yes, noSudo bool
}

func uninstallCmd() *cobra.Command {
	var o uninstallOpts
	c := &cobra.Command{
		Use:   "uninstall",
		Short: i18n.T("cmd.uninstall"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUninstall(bufio.NewReader(os.Stdin), cmd.OutOrStdout(), home(), o)
		},
	}
	f := c.Flags()
	f.StringSliceVar(&o.only, "only", nil, "providers: app, simruntime, android, brew, node, python, rust, jdk, npm-global, docker-image, vscode, ollama")
	f.BoolVar(&o.pick, "pick", false, "choose what to uninstall interactively")
	f.StringSliceVar(&o.remove, "remove", nil, "uninstall these ids (shown in the list)")
	f.BoolVar(&o.yes, "yes", false, "do not ask for confirmation (never approves sudo)")
	f.BoolVar(&o.noSudo, "no-sudo", false, "skip every step that needs sudo")
	return c
}

func runUninstall(in *bufio.Reader, w io.Writer, home string, o uninstallOpts) error {
	if o.pick && !stdinIsTerminal() {
		return errors.New(i18n.T("uninstall.needtty"))
	}
	introGate(in, w, stdinIsTerminal())
	_, cfg, err := rules.Load(home)
	if err != nil {
		return err
	}
	del := i18n.T("plan.delete.uninstall")
	if o.pick || len(o.remove) > 0 {
		del = i18n.T("plan.delete.uninstall.sel")
	}
	ui.ShowPlan(w, ui.Plan{
		Title:    "swelo uninstall",
		Scan:     []string{i18n.T("plan.scan.uninstall")},
		Read:     i18n.T("plan.read.uninstall"),
		Delete:   del,
		Perm:     i18n.T("plan.perm.uninstall"),
		Duration: i18n.T("plan.time.uninstall"),
	})
	fmt.Fprintf(w, " %s\n", ui.Dim(i18n.T("scanning")))
	roots := rules.ProjectRoots(cfg, home)
	env := uninstall.NewEnv(home, roots)
	st := ui.NewStatus(w)
	env.Progress = func(id string, n int) {
		if n < 0 {
			st.Set(i18n.T("prog.provider"), i18n.T("prov."+id))
		} else {
			st.Step(i18n.T("step.provider"), i18n.T("prov."+id), n)
		}
	}
	pkgs := uninstall.List(env, o.only)
	st.Stop()
	uninstall.SortBySize(pkgs)

	var chosen []uninstall.Package
	switch {
	case o.pick:
		items := make([]ui.PickItem, len(pkgs))
		for i, p := range pkgs {
			items[i] = ui.PickItem{Group: i18n.T("prov." + p.Provider), Label: ui.Bytes(p.Size) + "  " + ui.Safe(p.Name), Detail: pkgInfo(p, home)}
		}
		pk := ui.NewPicker(i18n.T("pick.title"), items)
		if _, err := tea.NewProgram(pk, tea.WithAltScreen()).Run(); err != nil {
			return err
		}
		if pk.Cancelled {
			fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
			return nil
		}
		for i, sel := range pk.Selected {
			if sel {
				chosen = append(chosen, pkgs[i])
			}
		}
	case len(o.remove) > 0:
		byID := map[string]uninstall.Package{}
		for _, p := range pkgs {
			byID[p.ID] = p
		}
		for _, id := range o.remove {
			p, ok := byID[id]
			if !ok {
				return fmt.Errorf(i18n.T("uninstall.unknown"), id)
			}
			chosen = append(chosen, p)
		}
	default:
		printPackages(w, pkgs, home)
		fmt.Fprintf(w, "\n %s\n %s\n\n", ui.Dim(i18n.T("uninstall.next")), ui.Dim(i18n.T("uninstall.next2")))
		return nil
	}
	return reviewAndRemove(in, w, chosen, o, runVisible, stateDir())
}

// reviewAndRemove shows every command that will run, asks once, then runs them.
func reviewAndRemove(in *bufio.Reader, w io.Writer, chosen []uninstall.Package, o uninstallOpts,
	run func([]string) error, state string) error {
	if len(chosen) == 0 {
		fmt.Fprintf(w, " %s\n\n", i18n.T("uninstall.none"))
		return nil
	}
	var total int64
	fmt.Fprintf(w, "\n %s\n\n", ui.Bold(i18n.T("uninstall.review")))
	for _, p := range chosen {
		total += p.Size
		fmt.Fprintf(w, "   %9s  %s\n", ui.Bytes(p.Size), ui.Bold(ui.Safe(p.Name)))
		cmd := ui.ShellJoin(p.Command)
		if p.Sudo {
			cmd = "sudo " + cmd + "  " + ui.Dim(i18n.T("uninstall.sudonote"))
		}
		fmt.Fprintf(w, "              $ %s\n", ui.Safe(cmd))
		for _, u := range p.UsedBy {
			fmt.Fprintf(w, "              %s\n", ui.Yellow(fmt.Sprintf(i18n.T("uninstall.warn"), fmt.Sprintf(i18n.T("pkg.usedby"), ui.Safe(u), len(p.UsedBy)))))
			break
		}
		if p.Note != "" {
			fmt.Fprintf(w, "              %s\n", ui.Yellow(fmt.Sprintf(i18n.T("uninstall.warn"), i18n.Reason(p.Note))))
		}
	}
	fmt.Fprintln(w)
	if !o.yes && !ask(in, w, fmt.Sprintf(i18n.T("uninstall.confirm"), len(chosen), ui.Bytes(total)), o.pick) {
		fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
		return nil
	}
	sudo := func(p uninstall.Package) error {
		return ui.RunSudo(in, w, ui.SudoStep{
			Action: ui.Safe(p.Name) + " (" + ui.Bytes(p.Size) + ")", Reason: i18n.T("sudo.reason.jdk"),
			Command: p.Command, Undo: i18n.T("sudo.undo.jdk"),
		}, ui.SudoOptions{NoSudo: o.noSudo})
	}
	outs := uninstall.Remove(chosen, run, sudo, filepath.Join(state, "history.jsonl"))
	var ok int
	var freed int64
	for _, r := range outs {
		switch {
		case r.Err == nil:
			ok++
			freed += r.Package.Size
			fmt.Fprintf(w, "   %s %s\n", ui.Green("✓"), ui.Safe(r.Package.Name))
		case errors.Is(r.Err, ui.ErrSkipped):
			fmt.Fprintf(w, "   %s %s\n", ui.Dim("–"), ui.Safe(r.Package.Name))
		default:
			fmt.Fprintf(w, "   %s %s: %s\n", ui.Red("✗"), ui.Safe(r.Package.Name), ui.Safe(r.Err.Error()))
		}
	}
	fmt.Fprintf(w, "\n %s\n %s %s\n\n", fmt.Sprintf(i18n.T("uninstall.done"), ok, len(chosen), ui.Bytes(freed)),
		ui.Dim(i18n.T("done.log")), filepath.Join(state, "history.jsonl"))
	return nil
}

// runVisible runs a command attached to the terminal so its own prompts and output show.
func runVisible(argv []string) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func printPackages(w io.Writer, pkgs []uninstall.Package, home string) {
	var total int64
	last := ""
	for _, p := range pkgs {
		if p.Provider != last {
			fmt.Fprintf(w, "\n %s %s\n", ui.Green("●"), ui.Bold(i18n.T("prov."+p.Provider)))
			last = p.Provider
		}
		total += p.Size
		fmt.Fprintf(w, "   %9s  %s %s\n", sizeOrBlank(p.Size), ui.Pad(ui.Safe(p.Name), 46), pkgInfo(p, home))
		fmt.Fprintf(w, "   %9s  %s\n", "", ui.Dim("id: "+ui.Safe(p.ID)))
	}
	fmt.Fprintf(w, "\n %s %s\n", ui.Bold(i18n.T("col.total")), ui.Bold(ui.Bytes(total)))
}

func sizeOrBlank(n int64) string {
	if n > 0 {
		return ui.Bytes(n)
	}
	return ""
}

func pkgInfo(p uninstall.Package, home string) string {
	var info []string
	if !p.LastUsed.IsZero() {
		info = append(info, fmt.Sprintf(i18n.T("pkg.lastused"), ago(p.LastUsed)))
	} else if !p.Installed.IsZero() {
		info = append(info, fmt.Sprintf(i18n.T("pkg.installed"), ago(p.Installed)))
	}
	if len(p.UsedBy) > 0 {
		info = append(info, ui.Yellow(fmt.Sprintf(i18n.T("pkg.usedby"), short(p.UsedBy[0], home), len(p.UsedBy))))
	}
	if p.Note != "" {
		info = append(info, ui.Yellow(i18n.Reason(p.Note)))
	}
	if p.Sudo {
		info = append(info, ui.Yellow("sudo"))
	}
	return ui.Dim(strings.Join(info, " · "))
}

// ago renders a duration in days ("today", "3 days ago").
func ago(t time.Time) string {
	d := int(time.Since(t).Hours() / 24)
	if d <= 0 {
		return i18n.T("today")
	}
	return fmt.Sprintf(i18n.T("days.ago"), d)
}
