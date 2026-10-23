package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"syscall"
	"time"

	"os"
	"os/exec"
	"os/signal"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/perm"
	"github.com/musacanminaz/swelo/internal/state"
	"github.com/musacanminaz/swelo/internal/ui"
)

var menuIDs = []string{"quick", "clean", "analyze", "tree", "uninstall", "optimize", "status", "history", "permissions", "lang", "quit"}

// runMenu is what `swelo` opens on a terminal: pick a command with the arrow
// keys, it runs in its interactive form (pick lists, menu confirmations), then
// the menu comes back.
func runMenu(in *bufio.Reader, w io.Writer, home string) error {
	introGate(in, w, true)
	// a sudo session granted from the menu never outlives swelo
	defer endSudo()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	go func() {
		if _, ok := <-sig; ok {
			endSudo()
			os.Exit(130)
		}
	}()
	for {
		menuHeader(w, home)
		items := make([]ui.PickItem, len(menuIDs))
		for i, id := range menuIDs {
			items[i] = ui.PickItem{Label: i18n.T("menu." + id), Detail: i18n.T("menu." + id + ".desc")}
		}
		choice := ui.Choose(i18n.T("menu.title"), items)
		if choice < 0 || menuIDs[choice] == "quit" {
			return nil
		}
		var err error
		switch menuIDs[choice] {
		case "quick":
			err = runClean(in, w, home, cleanOpts{apply: true, quick: true, recentDays: -1}, true)
		case "clean":
			err = runClean(in, w, home, cleanOpts{pick: true, recentDays: -1}, true)
		case "analyze":
			err = runAnalyze(in, w, home, false, 10, true)
		case "tree":
			err = runTree(in, w, home, home, false)
		case "uninstall":
			err = runUninstall(in, w, home, uninstallOpts{pick: true})
		case "optimize":
			err = runOptimize(in, w, home, false, false, true, nil)
		case "status":
			runStatus(w, home, stateDir(), time.Now())
		case "history":
			runHistory(w, home, stateDir(), 30)
		case "permissions":
			permissionsMenu(in, w, home)
			continue
		case "lang":
			if i18n.Lang() == "tr" {
				i18n.Set("en")
			} else {
				i18n.Set("tr")
			}
			continue
		}
		if err != nil {
			fmt.Fprintf(w, " %s %s\n", ui.Red("✗"), ui.Safe(err.Error()))
		}
		ui.WaitEnter(in, w, i18n.T("menu.back"))
	}
}

// menuHeader: version, a disk usage bar and the permissions swelo has.
func menuHeader(w io.Writer, home string) {
	var st syscall.Statfs_t
	syscall.Statfs(home, &st)
	total, free := int64(st.Blocks)*int64(st.Bsize), int64(st.Bavail)*int64(st.Bsize)
	bar := ""
	if total > 0 {
		n := int(20 * (total - free) / total)
		bar = strings.Repeat("█", n) + ui.Dim(strings.Repeat("░", 20-n))
	}
	fda := ui.Yellow(i18n.T("menu.fda.no"))
	if perm.FullDiskAccess(home) {
		fda = ui.Green(i18n.T("menu.fda.yes"))
	}
	if ui.SudoGranted() {
		fda += " · " + ui.Green(i18n.T("menu.sudo.yes"))
	}
	fmt.Fprintf(w, "\n %s %s — %s\n", ui.Bold("swelo"), Version, i18n.T("tagline"))
	fmt.Fprintf(w, " %s %s  %s / %s · %s %s · %s\n", i18n.T("disk"), bar, ui.Bytes(total-free), ui.Bytes(total),
		ui.Bold(ui.Bytes(free)), i18n.T("menu.free"), fda)
	fmt.Fprintf(w, " %s\n\n", ui.Dim(lastCleanup(state.History(stateDir()))))
}

// lastCleanup: when swelo last deleted something and how much that day.
func lastCleanup(hist []state.Record) string {
	var last time.Time
	var bytes int64
	for _, r := range hist {
		if !r.Succeeded() {
			continue
		}
		if day := r.Time.Local().Format("2006-01-02"); day != last.Local().Format("2006-01-02") {
			if r.Time.Before(last) {
				continue
			}
			bytes = 0
		}
		if !r.Time.Before(last) {
			last = r.Time
		}
		bytes += r.Bytes
	}
	if last.IsZero() {
		return i18n.T("menu.last.none")
	}
	return fmt.Sprintf(i18n.T("menu.last"), ago(last), ui.Bytes(bytes))
}

func endSudo() {
	if ui.SudoGranted() {
		ui.EndSudo()
	}
}

// permissionsMenu grants or drops the sudo session, opens the Full Disk
// Access settings, or explains every permission.
func permissionsMenu(in *bufio.Reader, w io.Writer, home string) {
	for {
		sudoLabel, sudoDesc := i18n.T("perm.menu.sudo"), i18n.T("perm.menu.sudo.desc")
		if ui.SudoGranted() {
			sudoLabel, sudoDesc = i18n.T("perm.menu.sudo.end"), i18n.T("perm.menu.sudo.end.desc")
		}
		fdaDesc := i18n.T("perm.menu.fda.desc")
		if perm.FullDiskAccess(home) {
			fdaDesc = i18n.T("perm.fda.ok")
		}
		items := []ui.PickItem{
			{Label: sudoLabel, Detail: sudoDesc},
			{Label: i18n.T("perm.menu.fda"), Detail: fdaDesc},
			{Label: i18n.T("perm.menu.info"), Detail: i18n.T("perm.menu.info.desc")},
			{Label: i18n.T("perm.menu.intro"), Detail: i18n.T("perm.menu.intro.desc")},
			{Label: i18n.T("perm.menu.back")},
		}
		fmt.Fprintln(w)
		switch ui.Choose(i18n.T("menu.permissions"), items) {
		case 0:
			switch {
			case ui.SudoGranted():
				ui.EndSudo()
				fmt.Fprintf(w, " %s %s\n", ui.Green("✓"), i18n.T("perm.sudo.ended"))
			case !perm.IsAdmin():
				fmt.Fprintf(w, " %s %s\n", ui.Yellow("!"), i18n.T("perm.sudo.notadmin"))
			default:
				fmt.Fprintf(w, "\n%s\n\n", ui.Indent(i18n.T("perm.sudo.explain"), " "))
				if n := perm.TerminalNeighbours(); len(n) > 0 {
					fmt.Fprintf(w, " %s %s\n", ui.Yellow("!"), i18n.T("perm.sudo.neighbours"))
					for _, p := range n {
						fmt.Fprintf(w, "     %s\n", ui.Safe(p))
					}
					if ui.Choose(i18n.T("perm.sudo.neighbours.ask"), []ui.PickItem{{Label: i18n.T("choice.no")}, {Label: i18n.T("choice.yes")}}) != 1 {
						continue
					}
				}
				if err := ui.GrantSudo(); err != nil {
					fmt.Fprintf(w, " %s %s\n", ui.Red("✗"), i18n.T("perm.sudo.failed"))
				} else {
					fmt.Fprintf(w, " %s %s\n", ui.Green("✓"), i18n.T("perm.sudo.granted"))
				}
			}
		case 1:
			fmt.Fprintf(w, "\n%s\n\n", ui.Indent(fmt.Sprintf(i18n.T("perm.fda.steps"), terminalApp()), " "))
			exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles").Run()
			ui.WaitEnter(in, w, i18n.T("menu.back"))
		case 2:
			printPermissions(w)
			ui.WaitEnter(in, w, i18n.T("menu.back"))
		case 3:
			welcome(w)
			ui.WaitEnter(in, w, i18n.T("menu.back"))
		default:
			return
		}
	}
}

// terminalApp names the app that needs Full Disk Access: the terminal swelo runs in.
func terminalApp() string {
	switch os.Getenv("TERM_PROGRAM") {
	case "Apple_Terminal":
		return "Terminal"
	case "iTerm.app":
		return "iTerm"
	case "vscode":
		return "Visual Studio Code / Cursor"
	case "":
		return i18n.T("perm.fda.yourterm")
	default:
		return ui.Safe(os.Getenv("TERM_PROGRAM"))
	}
}
