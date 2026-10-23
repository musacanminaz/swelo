package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/perm"
	"github.com/musacanminaz/swelo/internal/state"
	"github.com/musacanminaz/swelo/internal/ui"
	"github.com/spf13/cobra"
)

// recordSnapshot stores free space at most once an hour so status can show a trend.
func recordSnapshot() {
	var st syscall.Statfs_t
	if syscall.Statfs(home(), &st) == nil {
		state.RecordSnapshot(stateDir(), time.Now(), int64(st.Bavail)*int64(st.Bsize), int64(st.Blocks)*int64(st.Bsize))
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: i18n.T("cmd.status"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jsonFlag {
				return writeJSON(cmd.OutOrStdout(), statusData(home(), stateDir(), time.Now()))
			}
			runStatus(cmd.OutOrStdout(), home(), stateDir(), time.Now())
			return nil
		},
	}
}

// statusData gathers what status shows: disk, Docker's disk image, the
// free-space trend and cleanups per day (newest first).
func statusData(home, dir string, now time.Time) statusJSON {
	var d statusJSON
	var st syscall.Statfs_t
	syscall.Statfs(home, &st)
	total, free := int64(st.Blocks)*int64(st.Bsize), int64(st.Bavail)*int64(st.Bsize)
	d.Disk.TotalBytes, d.Disk.FreeBytes = total, free
	if total > 0 {
		d.Disk.UsedPercent = float64(total-free) / float64(total) * 100
	}
	if alloc, maxSize, ok := dockerRaw(home, perm.FullDiskAccess(home)); ok {
		d.DockerImage = &jsonDocker{alloc, maxSize}
	}
	cutoff := now.AddDate(0, 0, -30)
	for _, s := range state.Snapshots(dir) {
		if s.Time.After(cutoff) && now.Sub(s.Time) >= 24*time.Hour { // a trend needs at least a day
			d.Trend = &struct {
				Since         time.Time `json:"since"`
				FreeBytesThen int64     `json:"free_bytes_then"`
				DeltaBytes    int64     `json:"delta_bytes"`
			}{s.Time, s.Free, free - s.Free}
			break
		}
	}
	byDay := map[string]int64{}
	count := map[string]int{}
	for _, r := range state.History(dir) {
		if r.Succeeded() {
			day := r.Time.Local().Format("2006-01-02")
			byDay[day] += r.Bytes
			count[day]++
		}
	}
	days := make([]string, 0, len(byDay))
	for day := range byDay {
		days = append(days, day)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(days)))
	d.Cleanups = make([]struct {
		Day   string `json:"day"`
		Bytes int64  `json:"bytes"`
		Items int    `json:"items"`
	}, len(days))
	for i, day := range days {
		d.Cleanups[i].Day, d.Cleanups[i].Bytes, d.Cleanups[i].Items = day, byDay[day], count[day]
	}
	return d
}

func runStatus(w io.Writer, home, dir string, now time.Time) {
	d := statusData(home, dir, now)
	total, free, used := d.Disk.TotalBytes, d.Disk.FreeBytes, d.Disk.UsedPercent
	mark := ui.Green
	if used >= 90 {
		mark = ui.Red
	} else if used >= 80 {
		mark = ui.Yellow
	}
	fmt.Fprintf(w, "\n %s %s %s / %s (%s)\n", ui.Bold(i18n.T("disk")), mark(ui.Bytes(free)+" "+i18n.T("status.free")),
		ui.Bytes(total-free), ui.Bytes(total), mark(fmt.Sprintf("%.0f%%", used)))
	if d.DockerImage != nil {
		fmt.Fprintf(w, "   %s\n", ui.Dim(fmt.Sprintf(i18n.T("analyze.docker"), ui.Bytes(d.DockerImage.AllocatedBytes), ui.Bytes(d.DockerImage.MaxBytes))))
	}
	if t := d.Trend; t != nil {
		delta, sign := t.DeltaBytes, "+"
		if delta < 0 {
			sign, delta = "−", -delta
		}
		fmt.Fprintf(w, "   %s\n", ui.Dim(fmt.Sprintf(i18n.T("status.trend"), ago(t.Since), ui.Bytes(t.FreeBytesThen), ui.Bytes(free), sign+ui.Bytes(delta))))
	}
	if len(d.Cleanups) == 0 {
		fmt.Fprintf(w, "\n %s\n\n", ui.Dim(i18n.T("status.never")))
		return
	}
	fmt.Fprintf(w, "\n %s\n", ui.Bold(i18n.T("status.recent")))
	for i, c := range d.Cleanups {
		if i == 5 {
			break
		}
		fmt.Fprintf(w, "   %s  %9s  %s\n", c.Day, ui.Bytes(c.Bytes), ui.Dim(fmt.Sprintf(i18n.T("status.items"), c.Items)))
	}
	fmt.Fprintf(w, "\n %s\n\n", ui.Dim(i18n.T("status.more")))
}

func historyCmd() *cobra.Command {
	var n int
	c := &cobra.Command{
		Use:   "history",
		Short: i18n.T("cmd.history"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			runHistory(cmd.OutOrStdout(), home(), stateDir(), n)
			return nil
		},
	}
	c.Flags().IntVarP(&n, "number", "n", 30, i18n.T("flag.history.number"))
	return c
}

func runHistory(w io.Writer, home, dir string, n int) {
	hist := state.History(dir)
	if len(hist) == 0 {
		fmt.Fprintf(w, "\n %s\n\n", ui.Dim(i18n.T("status.never")))
		return
	}
	if len(hist) > n {
		hist = hist[len(hist)-n:]
	}
	fmt.Fprintln(w)
	for _, r := range hist {
		what, kind := r.Path, r.Rule
		if r.Uninstall != "" {
			what, kind = r.Uninstall, "uninstall"
		}
		if r.Startup != "" {
			what, kind = r.Startup, "startup"
		}
		mark := ui.Green("✓")
		if !r.Succeeded() {
			mark = ui.Red("✗")
		}
		fmt.Fprintf(w, " %s %s  %9s  %-16s %s\n", mark, r.Time.Local().Format("2006-01-02 15:04"), ui.Bytes(r.Bytes),
			ui.Safe(kind), short(what, home))
	}
	fmt.Fprintf(w, "\n %s %s\n\n", ui.Dim(i18n.T("history.file")), filepath.Join(dir, "history.jsonl"))
}
