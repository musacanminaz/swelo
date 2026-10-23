package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/speed"
	"github.com/musacanminaz/swelo/internal/ui"
	"github.com/spf13/cobra"
)

func speedCmd() *cobra.Command {
	var top int
	c := &cobra.Command{
		Use:   "speed",
		Short: i18n.T("cmd.speed"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jsonFlag {
				return writeJSON(cmd.OutOrStdout(), speed.Snapshot(speed.Real))
			}
			runSpeed(cmd.OutOrStdout(), top)
			return nil
		},
	}
	c.Flags().IntVar(&top, "top", 10, i18n.T("flag.speed.top"))
	return c
}

// runSpeed shows what is slowing the Mac down: memory, swap, load, uptime,
// the applications behind them, and what would help.
func runSpeed(w io.Writer, top int) {
	ui.ShowPlan(w, ui.Plan{
		Title: "swelo speed", Scan: []string{i18n.T("plan.scan.speed")}, Read: i18n.T("plan.read.speed"),
		Delete: i18n.T("plan.delete.none"), Perm: i18n.T("plan.perm.none"), Duration: i18n.T("plan.time.speed"),
	})
	fmt.Fprintf(w, " %s\n", ui.Dim(i18n.T("scanning")))
	s := speed.Snapshot(speed.Real)
	for _, e := range s.Errors {
		fmt.Fprintf(w, " %s %s\n", ui.Yellow("!"), ui.Safe(e))
	}
	printSummary(w, s, top)
}

func printSummary(w io.Writer, s speed.Summary, top int) {
	// swap and load are the two numbers that mean "slow"
	swap := ui.Green
	swapPct := 0.0
	if s.SwapTotal > 0 {
		swapPct = float64(s.SwapUsed) / float64(s.SwapTotal) * 100
		if swapPct >= 50 {
			swap = ui.Red
		} else if swapPct >= 20 {
			swap = ui.Yellow
		}
	}
	load := ui.Green
	if s.Cores > 0 && s.Load[0] > float64(s.Cores) {
		load = ui.Red
	} else if s.Cores > 0 && s.Load[0] > float64(s.Cores)*0.7 {
		load = ui.Yellow
	}
	fmt.Fprintf(w, "\n %s %s   %s %s   %s %s   %s %s\n\n",
		ui.Bold(i18n.T("speed.ram")), ui.Bytes(s.RAM),
		ui.Bold(i18n.T("speed.swap")), swap(fmt.Sprintf("%s / %s (%.0f%%)", ui.Bytes(s.SwapUsed), ui.Bytes(s.SwapTotal), swapPct)),
		ui.Bold(i18n.T("speed.load")), load(fmt.Sprintf("%.1f / %d %s", s.Load[0], s.Cores, i18n.T("speed.cores"))),
		ui.Bold(i18n.T("speed.uptime")), days(s.Uptime))

	fmt.Fprintf(w, " %s\n", ui.Bold(i18n.T("speed.bymem")))
	for i, a := range s.Apps {
		if i == top || a.Mem < 50<<20 {
			break
		}
		fmt.Fprintf(w, "   %9s  %s %s\n", ui.Bytes(a.Mem), ui.Pad(ui.Safe(a.Name), 44), ui.Dim(fmt.Sprintf(i18n.T("speed.procs"), a.Procs)))
	}
	byCPU := append([]speed.App(nil), s.Apps...)
	sortApps(byCPU, func(a, b speed.App) bool { return a.CPU > b.CPU })
	fmt.Fprintf(w, "\n %s\n", ui.Bold(i18n.T("speed.bycpu")))
	for i, a := range byCPU {
		if i == 5 || a.CPU < 1 {
			break
		}
		fmt.Fprintf(w, "   %8.1f%%  %s\n", a.CPU, ui.Safe(a.Name))
	}

	fmt.Fprintf(w, "\n %s\n", ui.Bold(i18n.T("speed.advice")))
	n := 0
	say := func(s string) { n++; fmt.Fprintf(w, "   %s %s\n", ui.Yellow("•"), s) }
	if swapPct >= 50 && len(s.Apps) > 0 {
		var big []string
		for i, a := range s.Apps {
			if i == 3 {
				break
			}
			big = append(big, fmt.Sprintf("%s (%s)", ui.Safe(a.Name), ui.Bytes(a.Mem)))
		}
		say(fmt.Sprintf(i18n.T("speed.advice.swap"), swapPct, strings.Join(big, ", ")))
	}
	if s.Cores > 0 && s.Load[0] > float64(s.Cores) && len(byCPU) > 0 {
		say(fmt.Sprintf(i18n.T("speed.advice.load"), s.Load[0], s.Cores, ui.Safe(byCPU[0].Name), byCPU[0].CPU))
	}
	if s.Uptime > 30*24*time.Hour {
		say(fmt.Sprintf(i18n.T("speed.advice.uptime"), days(s.Uptime)))
	}
	if n == 0 {
		fmt.Fprintf(w, "   %s\n", ui.Dim(i18n.T("speed.advice.none")))
	}
	fmt.Fprintln(w)
}

func sortApps(apps []speed.App, less func(a, b speed.App) bool) {
	for i := 1; i < len(apps); i++ { // small lists: insertion sort keeps it dependency-free
		for j := i; j > 0 && less(apps[j], apps[j-1]); j-- {
			apps[j], apps[j-1] = apps[j-1], apps[j]
		}
	}
}

func days(d time.Duration) string {
	if d < 48*time.Hour {
		return fmt.Sprintf(i18n.T("speed.hours"), int(d.Hours()))
	}
	return fmt.Sprintf(i18n.T("speed.days"), int(d.Hours()/24))
}
