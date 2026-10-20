package uninstall

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Outcome of removing one package.
type Outcome struct {
	Package Package
	Err     error // nil = removed; ErrSkipped from the sudo dialog = not run
}

// Remove runs each package's uninstall command. Sudo packages go through
// sudo (which shows the command and asks on its own); the rest through run.
// Every attempt is appended to logPath.
func Remove(pkgs []Package, run func([]string) error, sudo func(Package) error, logPath string) []Outcome {
	var out []Outcome
	for _, p := range pkgs {
		var err error
		switch {
		case len(p.Command) == 0:
			err = errors.New("no uninstall command")
		case p.Sudo:
			err = sudo(p)
		default:
			err = run(p.Command)
		}
		out = append(out, Outcome{p, err})
		logRemoval(logPath, p, err)
	}
	return out
}

func logRemoval(path string, p Package, err error) {
	if path == "" {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, ferr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if ferr != nil {
		return
	}
	defer f.Close()
	rec := map[string]any{"time": time.Now().Format(time.RFC3339), "uninstall": p.ID, "command": p.Command, "bytes": p.Size, "ok": err == nil}
	if err != nil {
		rec["error"] = err.Error()
	}
	json.NewEncoder(f).Encode(rec)
}
