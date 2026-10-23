package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/speed"
	"github.com/musacanminaz/swelo/internal/ui"
	"github.com/spf13/cobra"
)

func speedCmd() *cobra.Command {
	var top int
	var stop bool
	c := &cobra.Command{
		Use:   "speed",
		Short: i18n.T("cmd.speed"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jsonFlag {
				s := speed.Snapshot(speed.Real)
				return writeJSON(cmd.OutOrStdout(), speedJSON{s, speed.StaleProcesses(speed.Real, s.Procs, home())})
			}
			if stop && !stdinIsTerminal() {
				return errors.New(i18n.T("speed.stop.needtty"))
			}
			return runSpeed(bufio.NewReader(os.Stdin), cmd.OutOrStdout(), home(), top, stop)
		},
	}
	c.Flags().IntVar(&top, "top", 10, i18n.T("flag.speed.top"))
	c.Flags().BoolVar(&stop, "stop", false, i18n.T("flag.speed.stop"))
	return c
}

type speedJSON struct {
	speed.Summary
	Stale []speed.Stale `json:"stale_processes"`
}

// runSpeed shows what is slowing the Mac down: memory, swap, load, uptime,
// the applications behind them, forgotten developer processes, and what
// would help. With pick, the forgotten processes can be ticked and stopped.
func runSpeed(in *bufio.Reader, w io.Writer, home string, top int, pick bool) error {
	del := i18n.T("plan.delete.none")
	if pick {
		del = i18n.T("plan.delete.speed")
	}
	ui.ShowPlan(w, ui.Plan{
		Title: "swelo speed", Scan: []string{i18n.T("plan.scan.speed")}, Read: i18n.T("plan.read.speed"),
		Delete: del, Perm: i18n.T("plan.perm.none"), Duration: i18n.T("plan.time.speed"),
	})
	fmt.Fprintf(w, " %s\n", ui.Dim(i18n.T("scanning")))
	s := speed.Snapshot(speed.Real)
	for _, e := range s.Errors {
		fmt.Fprintf(w, " %s %s\n", ui.Yellow("!"), ui.Safe(e))
	}
	printSummary(w, s, top)
	stale := speed.StaleProcesses(speed.Real, s.Procs, home)
	fmt.Fprintf(w, " %s\n", ui.Bold(i18n.T("speed.stale")))
	if len(stale) == 0 {
		fmt.Fprintf(w, "   %s\n\n", ui.Dim(i18n.T("speed.stale.none")))
		return nil
	}
	for _, p := range stale {
		fmt.Fprintf(w, "   %9s  %s %s\n", ui.Bytes(p.Mem), ui.Pad(ui.Safe(p.App), 36), ui.Dim(staleInfo(p, home)))
	}
	if !pick {
		fmt.Fprintf(w, "\n   %s\n\n", ui.Dim(i18n.T("speed.stale.hint")))
		return nil
	}
	items := make([]ui.PickItem, len(stale))
	for i, p := range stale {
		items[i] = ui.PickItem{Label: ui.Safe(p.App), Detail: staleInfo(p, home), Size: p.Mem}
	}
	fmt.Fprintln(w)
	pk := ui.NewPicker(i18n.T("pick.stale"), items)
	if _, err := tea.NewProgram(pk, tea.WithAltScreen()).Run(); err != nil || pk.Cancelled {
		fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
		return ErrCancelled
	}
	var pids []int
	var mem int64
	for i, sel := range pk.Selected {
		if sel {
			pids = append(pids, stale[i].PID)
			mem += stale[i].Mem
		}
	}
	if len(pids) == 0 {
		fmt.Fprintf(w, " %s\n\n", i18n.T("pick.none"))
		return nil
	}
	if !ask(in, w, fmt.Sprintf(i18n.T("speed.stop.confirm"), len(pids), ui.Bytes(mem)), true) {
		fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
		return ErrCancelled
	}
	alive, err := speed.Stop(pids)
	if err != nil {
		fmt.Fprintf(w, " %s %s\n", ui.Red("✗"), ui.Safe(err.Error()))
	}
	fmt.Fprintf(w, " %s %s\n", ui.Green("✓"), fmt.Sprintf(i18n.T("speed.stopped"), len(pids)-len(alive)))
	if len(alive) > 0 {
		var names []string
		for _, p := range stale {
			if slices.Contains(alive, p.PID) {
				names = append(names, fmt.Sprintf("%s (pid %d)", ui.Safe(p.App), p.PID))
			}
		}
		fmt.Fprintf(w, " %s %s\n", ui.Yellow("!"), fmt.Sprintf(i18n.T("speed.stillrunning"), strings.Join(names, ", ")))
	}
	fmt.Fprintln(w)
	return nil
}

// staleInfo: why a process is listed, its age, and its folder when relevant.
func staleInfo(p speed.Stale, home string) string {
	s := i18n.Reason(p.Reason) + " · " + days(p.Age)
	if p.Reason == speed.ReasonTrash || p.Reason == speed.ReasonGone {
		s += " · " + short(p.Cwd, home)
	}
	if p.Children > 0 {
		s += " · " + fmt.Sprintf(i18n.T("speed.procs"), p.Children+1)
	}
	return s
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
