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
			runStatus(cmd.OutOrStdout(), home(), stateDir(), time.Now())
			return nil
		},
	}
}

func runStatus(w io.Writer, home, dir string, now time.Time) {
	var st syscall.Statfs_t
	syscall.Statfs(home, &st)
	total, free := int64(st.Blocks)*int64(st.Bsize), int64(st.Bavail)*int64(st.Bsize)
	used := 0.0
	if total > 0 {
		used = float64(total-free) / float64(total) * 100
	}
	mark := ui.Green
	if used >= 90 {
		mark = ui.Red
	} else if used >= 80 {
		mark = ui.Yellow
	}
	fmt.Fprintf(w, "\n %s %s %s / %s (%s)\n", ui.Bold(i18n.T("disk")), mark(ui.Bytes(free)+" "+i18n.T("status.free")),
		ui.Bytes(total-free), ui.Bytes(total), mark(fmt.Sprintf("%.0f%%", used)))
	if d := dockerImage(home, perm.FullDiskAccess(home)); d != "" {
		fmt.Fprintf(w, "   %s\n", ui.Dim(d))
	}

	// free space trend from snapshots
	snaps := state.Snapshots(dir)
	cutoff := now.AddDate(0, 0, -30)
	for _, s := range snaps {
		if s.Time.After(cutoff) && now.Sub(s.Time) >= 24*time.Hour { // a trend needs at least a day
			delta := free - s.Free
			sign := "+"
			if delta < 0 {
				sign, delta = "−", -delta
			}
			fmt.Fprintf(w, "   %s\n", ui.Dim(fmt.Sprintf(i18n.T("status.trend"), ago(s.Time), ui.Bytes(s.Free), sign+ui.Bytes(delta))))
			break
		}
	}

	// cleanups per day
	hist := state.History(dir)
	byDay := map[string]int64{}
	count := map[string]int{}
	for _, r := range hist {
		if r.Succeeded() {
			d := r.Time.Local().Format("2006-01-02")
			byDay[d] += r.Bytes
			count[d]++
		}
	}
	if len(byDay) == 0 {
		fmt.Fprintf(w, "\n %s\n\n", ui.Dim(i18n.T("status.never")))
		return
	}
	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(days)))
	fmt.Fprintf(w, "\n %s\n", ui.Bold(i18n.T("status.recent")))
	for i, d := range days {
		if i == 5 {
			break
		}
		fmt.Fprintf(w, "   %s  %9s  %s\n", d, ui.Bytes(byDay[d]), ui.Dim(fmt.Sprintf(i18n.T("status.items"), count[d])))
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
	c.Flags().IntVarP(&n, "number", "n", 30, "how many entries")
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
		mark := ui.Green("✓")
		if !r.Succeeded() {
			mark = ui.Red("✗")
		}
		fmt.Fprintf(w, " %s %s  %9s  %-16s %s\n", mark, r.Time.Local().Format("2006-01-02 15:04"), ui.Bytes(r.Bytes),
			ui.Safe(kind), short(what, home))
	}
	fmt.Fprintf(w, "\n %s %s\n\n", ui.Dim(i18n.T("history.file")), filepath.Join(dir, "history.jsonl"))
}
