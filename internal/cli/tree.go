package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/perm"
	"github.com/musacanminaz/swelo/internal/tree"
	"github.com/musacanminaz/swelo/internal/ui"
)

func runTree(in *bufio.Reader, w io.Writer, home, dir string, full bool) error {
	if !stdinIsTerminal() {
		return errors.New(i18n.T("tree.needtty"))
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	introGate(in, w, true)
	protected := perm.ProtectedDirs(home)
	skip := protected
	permText := i18n.T("plan.perm.analyze")
	if full && perm.FullDiskAccess(home) {
		skip, permText = nil, i18n.T("plan.perm.full")
	}
	ui.ShowPlan(w, ui.Plan{
		Title:    "swelo analyze --tree",
		Scan:     []string{short(abs, home)},
		Read:     i18n.T("plan.read.analyze"),
		Delete:   i18n.T("plan.delete.tree"),
		Perm:     permText,
		Duration: i18n.T("plan.time.analyze"),
	})
	fmt.Fprintf(w, " %s\n", ui.Dim(i18n.T("scanning")))
	root := tree.Build(abs, skip)
	b := tree.NewBrowser(root, home, protected)
	if _, err := tea.NewProgram(b, tea.WithAltScreen()).Run(); err != nil {
		return err
	}
	if b.Freed > 0 {
		fmt.Fprintf(w, " %s\n\n", fmt.Sprintf(i18n.T("tree.summary"), ui.Bytes(b.Freed)))
	}
	return nil
}
