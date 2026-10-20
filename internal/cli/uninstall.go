package cli

import (
	"bufio"
	"fmt"
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

func uninstallCmd() *cobra.Command {
	var only []string
	c := &cobra.Command{
		Use:   "uninstall",
		Short: i18n.T("cmd.uninstall"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUninstallList(bufio.NewReader(os.Stdin), cmd.OutOrStdout(), home(), only)
		},
	}
	c.Flags().StringSliceVar(&only, "only", nil, "providers: simruntime, android, brew, node, python, rust, jdk, npm-global, docker-image, vscode, ollama")
	return c
}

func runUninstallList(in *bufio.Reader, w io.Writer, home string, only []string) error {
	introGate(in, w, stdinIsTerminal())
	_, cfg, err := rules.Load(home)
	if err != nil {
		return err
	}
	ui.ShowPlan(w, ui.Plan{
		Title:    "swelo uninstall",
		Scan:     []string{i18n.T("plan.scan.uninstall")},
		Read:     i18n.T("plan.read.uninstall"),
		Delete:   i18n.T("plan.delete.uninstall"),
		Perm:     i18n.T("plan.perm.uninstall"),
		Duration: i18n.T("plan.time.uninstall"),
	})
	fmt.Fprintf(w, " %s\n", ui.Dim(i18n.T("scanning")))
	var roots []string
	for _, r := range cfg.Roots {
		roots = append(roots, rules.Expand(r, home))
	}
	pkgs := uninstall.List(uninstall.NewEnv(home, roots), only)
	uninstall.SortBySize(pkgs)
	printPackages(w, pkgs, home)
	fmt.Fprintf(w, "\n %s\n\n", ui.Dim(i18n.T("uninstall.next")))
	return nil
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
		size := ""
		if p.Size > 0 {
			size = ui.Bytes(p.Size)
		}
		fmt.Fprintf(w, "   %9s  %s %s\n", size, ui.Pad(ui.Safe(p.Name), 46), ui.Dim(strings.Join(info, " · ")))
	}
	fmt.Fprintf(w, "\n %s %s\n", ui.Bold(i18n.T("col.total")), ui.Bold(ui.Bytes(total)))
}

// ago renders a duration in days ("today", "3 days ago").
func ago(t time.Time) string {
	d := int(time.Since(t).Hours() / 24)
	if d <= 0 {
		return i18n.T("today")
	}
	return fmt.Sprintf(i18n.T("days.ago"), d)
}
