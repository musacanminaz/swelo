package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"syscall"
	"time"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/perm"
	"github.com/musacanminaz/swelo/internal/ui"
)

var menuIDs = []string{"clean", "analyze", "tree", "uninstall", "optimize", "status", "history", "permissions", "lang", "quit"}

// runMenu is what `swelo` opens on a terminal: pick a command with the arrow
// keys, it runs in its interactive form (pick lists, menu confirmations), then
// the menu comes back.
func runMenu(in *bufio.Reader, w io.Writer, home string) error {
	introGate(in, w, true)
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
			printPermissions(w)
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
	fmt.Fprintf(w, "\n %s %s — %s\n", ui.Bold("swelo"), Version, i18n.T("tagline"))
	fmt.Fprintf(w, " %s %s  %s / %s · %s %s · %s\n\n", i18n.T("disk"), bar, ui.Bytes(total-free), ui.Bytes(total),
		ui.Bold(ui.Bytes(free)), i18n.T("menu.free"), fda)
}
