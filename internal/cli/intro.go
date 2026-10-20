package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/ui"
)

func introMarker() string { return filepath.Join(configDir(), ".intro-seen") }

// introGate shows the full intro once, before the first command that scans
// the disk, and waits for Enter when the input is a terminal.
func introGate(in *bufio.Reader, w io.Writer, interactive bool) {
	if _, err := os.Stat(introMarker()); err == nil {
		return
	}
	fmt.Fprintf(w, "\n %s\n", ui.Bold(i18n.T("intro.first")))
	welcome(w)
	fmt.Fprintf(w, " %s: %s\n\n", i18n.T("intro.config"), filepath.Join(configDir(), "config.yaml"))
	if interactive {
		ui.WaitEnter(in, w, i18n.T("intro.enter"))
	}
	if os.MkdirAll(configDir(), 0o755) == nil {
		os.WriteFile(introMarker(), nil, 0o644)
	}
}

func stdinIsTerminal() bool { return ui.IsTerminal(os.Stdin) }
