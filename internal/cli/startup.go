package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/startup"
	"github.com/musacanminaz/swelo/internal/state"
	"github.com/musacanminaz/swelo/internal/ui"
	"github.com/spf13/cobra"
)

func startupCmd() *cobra.Command {
	var pick, noSudo bool
	c := &cobra.Command{
		Use:   "startup",
		Short: i18n.T("cmd.startup"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jsonFlag {
				return writeJSON(cmd.OutOrStdout(), startup.Scan(startup.Real, home(), os.Getuid()))
			}
			if pick && !stdinIsTerminal() {
				return errors.New(i18n.T("startup.needtty"))
			}
			return runStartup(bufio.NewReader(os.Stdin), cmd.OutOrStdout(), home(), pick, noSudo)
		},
	}
	c.Flags().BoolVar(&pick, "pick", false, i18n.T("flag.startup.pick"))
	c.Flags().BoolVar(&noSudo, "no-sudo", false, i18n.T("flag.uninstall.nosudo"))
	return c
}

// runStartup lists what starts at login or boot, grouped by kind, flagging
// jobs whose app is gone and auto-updaters. With pick, ticked jobs are turned
// off: agents for this user with launchctl, daemons as separate sudo steps.
func runStartup(in *bufio.Reader, w io.Writer, home string, pick, noSudo bool) error {
	del := i18n.T("plan.delete.none")
	if pick {
		del = i18n.T("plan.delete.startup")
	}
	ui.ShowPlan(w, ui.Plan{
		Title: "swelo startup", Scan: []string{"~/Library/LaunchAgents", "/Library/LaunchAgents", "/Library/LaunchDaemons"},
		Read: i18n.T("plan.read.startup"), Delete: del, Perm: i18n.T("plan.perm.startup"), Duration: i18n.T("plan.time.speed"),
	})
	fmt.Fprintf(w, " %s\n", ui.Dim(i18n.T("scanning")))
	items := startup.Scan(startup.Real, home, os.Getuid())
	kind := ""
	orphans, updaters := 0, 0
	for _, it := range items {
		if it.Kind != kind {
			kind = it.Kind
			fmt.Fprintf(w, "\n %s %s\n", ui.Bold(i18n.T("startup."+strings.ReplaceAll(kind, " ", "."))), ui.Dim(i18n.T("startup."+strings.ReplaceAll(kind, " ", ".")+".desc")))
		}
		mark := ui.Dim("○")
		if it.Running {
			mark = ui.Green("●")
		}
		if it.Disabled {
			mark = ui.Dim("–")
		}
		fmt.Fprintf(w, "   %s %s %s\n", mark, ui.Pad(ui.Safe(it.Label), 48), ui.Dim(startupInfo(it, home)))
		if it.Orphan {
			orphans++
		}
		if it.Updater {
			updaters++
		}
	}
	fmt.Fprintf(w, "\n %s\n", fmt.Sprintf(i18n.T("startup.summary"), len(items), orphans, updaters))
	if !pick {
		fmt.Fprintf(w, " %s\n\n", ui.Dim(i18n.T("startup.hint")))
		return nil
	}
	return disableStartup(in, w, items, noSudo)
}

// startupInfo: the app or program, and what stands out.
func startupInfo(it startup.Item, home string) string {
	var parts []string
	switch {
	case it.Empty:
		parts = append(parts, i18n.T("startup.empty"))
	case it.App != "":
		parts = append(parts, ui.Safe(it.App))
	case it.Program != "":
		parts = append(parts, short(it.Program, home))
	}
	if it.Orphan && !it.Empty {
		parts = append(parts, i18n.T("startup.orphan"))
	}
	if it.Updater {
		parts = append(parts, i18n.T("startup.updater"))
	}
	if it.Running {
		parts = append(parts, fmt.Sprintf(i18n.T("startup.running"), it.PID))
	}
	if it.Disabled {
		parts = append(parts, i18n.T("startup.disabled"))
	}
	if it.KeepAlive {
		parts = append(parts, i18n.T("startup.keepalive"))
	}
	return strings.Join(parts, " · ")
}

// disableStartup offers the jobs in a pick list (orphans and updaters first,
// nothing ticked) and turns the chosen ones off.
func disableStartup(in *bufio.Reader, w io.Writer, items []startup.Item, noSudo bool) error {
	var rows []startup.Item
	for _, it := range items {
		if !it.Disabled {
			rows = append(rows, it)
		}
	}
	if len(rows) == 0 {
		fmt.Fprintf(w, " %s\n\n", i18n.T("startup.none"))
		return nil
	}
	pickItems := make([]ui.PickItem, len(rows))
	for i, it := range rows {
		pickItems[i] = ui.PickItem{Group: i18n.T("startup." + strings.ReplaceAll(it.Kind, " ", ".")), Label: ui.Safe(it.Label), Detail: startupInfo(it, home())}
	}
	pk := ui.NewPicker(i18n.T("pick.startup"), pickItems)
	if _, err := tea.NewProgram(pk, tea.WithAltScreen()).Run(); err != nil || pk.Cancelled {
		fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
		return ErrCancelled
	}
	var chosen []startup.Item
	for i, sel := range pk.Selected {
		if sel {
			chosen = append(chosen, rows[i])
		}
	}
	if len(chosen) == 0 {
		fmt.Fprintf(w, " %s\n\n", i18n.T("pick.none"))
		return nil
	}
	uid := os.Getuid()
	fmt.Fprintf(w, "\n %s\n\n", ui.Bold(i18n.T("startup.review")))
	for _, it := range chosen {
		off, undo, sudo := startup.Commands(it, uid)
		fmt.Fprintf(w, "   %s\n", ui.Bold(ui.Safe(it.Label)))
		for _, c := range off {
			prefix := "$ "
			if sudo {
				prefix = "$ sudo "
			}
			fmt.Fprintf(w, "     %s%s\n", prefix, ui.ShellJoin(c))
		}
		fmt.Fprintf(w, "     %s %s\n", ui.Dim(i18n.T("startup.undo")), ui.Dim(ui.Safe(undo)))
	}
	fmt.Fprintln(w)
	if !ask(in, w, fmt.Sprintf(i18n.T("startup.confirm"), len(chosen)), true) {
		fmt.Fprintf(w, " %s\n\n", i18n.T("cancelled"))
		return ErrCancelled
	}
	logPath := filepath.Join(stateDir(), "history.jsonl")
	for _, it := range chosen {
		off, undo, sudo := startup.Commands(it, uid)
		var err error
		if sudo {
			err = ui.RunSudo(in, w, ui.SudoStep{
				Action: it.Label, Reason: i18n.T("sudo.reason.daemon"),
				Command: []string{"sh", "-c", ui.ShellJoin(off[0]) + "; " + ui.ShellJoin(off[1])}, Undo: undo,
			}, ui.SudoOptions{NoSudo: noSudo, Pick: true})
		} else {
			for _, c := range off {
				if e := runVisible(c); e != nil && err == nil {
					err = e // bootout fails when the job is not loaded; disable still applies
				}
			}
			if _, e := startup.Real("launchctl", "print-disabled", fmt.Sprintf("gui/%d", uid)); e == nil {
				err = nil
			}
		}
		switch {
		case errors.Is(err, ui.ErrSkipped):
			fmt.Fprintf(w, "   %s %s\n", ui.Dim("–"), ui.Safe(it.Label))
		case err != nil:
			fmt.Fprintf(w, "   %s %s: %s\n", ui.Red("✗"), ui.Safe(it.Label), ui.Safe(err.Error()))
		default:
			fmt.Fprintf(w, "   %s %s\n", ui.Green("✓"), ui.Safe(it.Label))
			logStartup(logPath, it, undo)
		}
	}
	fmt.Fprintln(w)
	return nil
}

func logStartup(path string, it startup.Item, undo string) {
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	ok := true
	json.NewEncoder(f).Encode(state.Record{Time: time.Now(), Startup: it.Label, Path: it.File, Undo: undo, OK: &ok})
}
