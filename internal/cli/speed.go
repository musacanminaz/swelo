package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/perm"
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
				return writeJSON(cmd.OutOrStdout(), gather(home()))
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

// report is everything speed looks at (also the --json document).
type report struct {
	speed.Summary
	Stale    []speed.Stale   `json:"stale_processes"`
	Docker   speed.Docker    `json:"docker"`
	Services []speed.Service `json:"brew_services"`
}

func have(name string) bool { _, err := exec.LookPath(name); return err == nil }

func gather(home string) report {
	s := speed.Snapshot(speed.Real)
	return report{
		Summary:  s,
		Stale:    speed.StaleProcesses(speed.Real, s.Procs, home),
		Docker:   speed.DockerInfo(speed.Real, have, home, perm.FullDiskAccess(home)),
		Services: speed.BrewServices(speed.Real, have),
	}
}

// runSpeed shows what is slowing the Mac down: memory, swap, load, uptime,
// the applications behind them, Docker's share, Homebrew services,
// forgotten developer processes, and what would help. With pick, forgotten
// processes and services can be ticked and stopped.
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
	r := gather(home)
	for _, e := range r.Errors {
		fmt.Fprintf(w, " %s %s\n", ui.Yellow("!"), ui.Safe(e))
	}
	printLoad(w, r.Summary, top)
	printDocker(w, r.Docker, r.RAM)
	printServices(w, r.Services)
	printStale(w, r.Stale, home, pick)
	printAdvice(w, r)
	if !pick {
		return nil
	}
	if err := stopStale(in, w, r.Stale, home); err != nil {
		return err
	}
	return stopServices(in, w, r.Services)
}

func printLoad(w io.Writer, s speed.Summary, top int) {
	// swap and load are the two numbers that mean "slow"
	swap := ui.Green
	swapPct := swapPercent(s)
	if swapPct >= 50 {
		swap = ui.Red
	} else if swapPct >= 20 {
		swap = ui.Yellow
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
	fmt.Fprintf(w, "\n %s\n", ui.Bold(i18n.T("speed.bycpu")))
	for i, a := range byCPU(s) {
		if i == 5 || a.CPU < 1 {
			break
		}
		fmt.Fprintf(w, "   %8.1f%%  %s\n", a.CPU, ui.Safe(a.Name))
	}
	fmt.Fprintln(w)
}

func printDocker(w io.Writer, d speed.Docker, ram int64) {
	if !d.Installed {
		return
	}
	fmt.Fprintf(w, " %s\n", ui.Bold("Docker"))
	switch {
	case !d.SettingsRead:
		fmt.Fprintf(w, "   %s\n", ui.Dim(i18n.T("speed.docker.nosettings")))
	default:
		share := ""
		if ram > 0 {
			share = fmt.Sprintf(" (%.0f%%)", float64(d.MemoryBytes)/float64(ram)*100)
		}
		saver := i18n.T("speed.docker.saver.off")
		if d.ResourceSaver {
			saver = i18n.T("speed.docker.saver.on")
		}
		fmt.Fprintf(w, "   %s %s%s · %d CPU · %s   %s\n", i18n.T("speed.docker.mem"), ui.Bytes(d.MemoryBytes), share, d.CPUs, saver, ui.Dim(i18n.T("speed.docker.where")))
	}
	if !d.Running {
		fmt.Fprintf(w, "   %s\n\n", ui.Dim(i18n.T("speed.docker.off")))
		return
	}
	fmt.Fprintf(w, "   %s\n", fmt.Sprintf(i18n.T("speed.docker.containers"), len(d.Containers)))
	for _, c := range d.Containers {
		cpu := fmt.Sprintf("%.1f%%", c.CPUPercent)
		if c.Idle() {
			cpu = ui.Dim(i18n.T("speed.idle"))
		}
		fmt.Fprintf(w, "     %s %9s  %6s  %s\n", ui.Pad(ui.Safe(c.Name), 30), ui.Bytes(c.MemBytes), cpu, ui.Dim(ui.Safe(c.Up)))
	}
	fmt.Fprintln(w)
}

func printServices(w io.Writer, svcs []speed.Service) {
	if len(svcs) == 0 {
		return
	}
	fmt.Fprintf(w, " %s\n", ui.Bold(i18n.T("speed.services")))
	for _, s := range svcs {
		fmt.Fprintf(w, "   %s %s\n", ui.Pad(ui.Safe(s.Name), 30), ui.Dim(ui.Safe(s.Status)))
	}
	fmt.Fprintln(w)
}

func printStale(w io.Writer, stale []speed.Stale, home string, pick bool) {
	fmt.Fprintf(w, " %s\n", ui.Bold(i18n.T("speed.stale")))
	if len(stale) == 0 {
		fmt.Fprintf(w, "   %s\n\n", ui.Dim(i18n.T("speed.stale.none")))
		return
	}
	for _, p := range stale {
		fmt.Fprintf(w, "   %9s  %s %s\n", ui.Bytes(p.Mem), ui.Pad(ui.Safe(p.App), 36), ui.Dim(staleInfo(p, home)))
	}
	if !pick {
		fmt.Fprintf(w, "   %s\n", ui.Dim(i18n.T("speed.stale.hint")))
	}
	fmt.Fprintln(w)
}

func printAdvice(w io.Writer, r report) {
	fmt.Fprintf(w, " %s\n", ui.Bold(i18n.T("speed.advice")))
	n := 0
	say := func(s string) { n++; fmt.Fprintf(w, "   %s %s\n", ui.Yellow("•"), s) }
	s := r.Summary
	if pct := swapPercent(s); pct >= 50 && len(s.Apps) > 0 {
		var big []string
		for i, a := range s.Apps {
			if i == 3 {
				break
			}
			big = append(big, fmt.Sprintf("%s (%s)", ui.Safe(a.Name), ui.Bytes(a.Mem)))
		}
		say(fmt.Sprintf(i18n.T("speed.advice.swap"), pct, strings.Join(big, ", ")))
	}
	if cpu := byCPU(s); s.Cores > 0 && s.Load[0] > float64(s.Cores) && len(cpu) > 0 {
		say(fmt.Sprintf(i18n.T("speed.advice.load"), s.Load[0], s.Cores, ui.Safe(cpu[0].Name), cpu[0].CPU))
	}
	if d := r.Docker; d.SettingsRead && s.RAM > 0 && float64(d.MemoryBytes) >= 0.4*float64(s.RAM) {
		say(fmt.Sprintf(i18n.T("speed.advice.docker"), ui.Bytes(d.MemoryBytes), float64(d.MemoryBytes)/float64(s.RAM)*100))
	}
	var idle []string
	for _, c := range r.Docker.Containers {
		if c.Idle() {
			idle = append(idle, ui.Safe(c.Name))
		}
	}
	if len(idle) > 0 {
		say(fmt.Sprintf(i18n.T("speed.advice.idle"), len(idle), "docker stop "+strings.Join(idle, " ")))
	}
	if len(r.Stale) > 0 {
		var mem int64
		for _, p := range r.Stale {
			mem += p.Mem
		}
		say(fmt.Sprintf(i18n.T("speed.advice.stale"), len(r.Stale), ui.Bytes(mem)))
	}
	if s.Uptime > 30*24*time.Hour {
		say(fmt.Sprintf(i18n.T("speed.advice.uptime"), days(s.Uptime)))
	}
	if n == 0 {
		fmt.Fprintf(w, "   %s\n", ui.Dim(i18n.T("speed.advice.none")))
	}
	fmt.Fprintln(w)
}

// stopStale offers the forgotten processes in a pick list and SIGTERMs the
// ticked ones after a confirmation.
func stopStale(in *bufio.Reader, w io.Writer, stale []speed.Stale, home string) error {
	if len(stale) == 0 {
		return nil
	}
	items := make([]ui.PickItem, len(stale))
	for i, p := range stale {
		items[i] = ui.PickItem{Label: ui.Safe(p.App), Detail: staleInfo(p, home), Size: p.Mem}
	}
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

// stopServices offers always-on Homebrew services; ticked ones are stopped
// with brew's own command (undo: brew services start <name>).
func stopServices(in *bufio.Reader, w io.Writer, svcs []speed.Service) error {
	if len(svcs) == 0 {
		return nil
	}
	items := make([]ui.PickItem, len(svcs))
	for i, s := range svcs {
		items[i] = ui.PickItem{Label: ui.Safe(s.Name), Detail: ui.Safe(s.Status)}
	}
	pk := ui.NewPicker(i18n.T("pick.services"), items)
	if _, err := tea.NewProgram(pk, tea.WithAltScreen()).Run(); err != nil || pk.Cancelled {
		fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
		return ErrCancelled
	}
	var chosen []string
	for i, sel := range pk.Selected {
		if sel {
			chosen = append(chosen, svcs[i].Name)
		}
	}
	if len(chosen) == 0 {
		return nil
	}
	fmt.Fprintf(w, "\n   $ brew services stop %s\n   %s\n\n", ui.Safe(strings.Join(chosen, " ")), ui.Dim(i18n.T("speed.services.undo")))
	if !ask(in, w, fmt.Sprintf(i18n.T("speed.services.confirm"), len(chosen)), true) {
		fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
		return ErrCancelled
	}
	for _, name := range chosen {
		if err := runVisible([]string{"brew", "services", "stop", name}); err != nil {
			fmt.Fprintf(w, "   %s %s: %s\n", ui.Red("✗"), ui.Safe(name), ui.Safe(err.Error()))
		} else {
			fmt.Fprintf(w, "   %s %s\n", ui.Green("✓"), ui.Safe(name))
		}
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

func swapPercent(s speed.Summary) float64 {
	if s.SwapTotal == 0 {
		return 0
	}
	return float64(s.SwapUsed) / float64(s.SwapTotal) * 100
}

func byCPU(s speed.Summary) []speed.App {
	apps := append([]speed.App(nil), s.Apps...)
	slices.SortFunc(apps, func(a, b speed.App) int {
		switch {
		case a.CPU > b.CPU:
			return -1
		case a.CPU < b.CPU:
			return 1
		}
		return 0
	})
	return apps
}

func days(d time.Duration) string {
	if d < 48*time.Hour {
		return fmt.Sprintf(i18n.T("speed.hours"), int(d.Hours()))
	}
	return fmt.Sprintf(i18n.T("speed.days"), int(d.Hours()/24))
}
