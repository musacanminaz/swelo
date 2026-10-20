package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/state"
)

func TestStatusAndHistory(t *testing.T) {
	i18n.Set("en")
	home, dir := t.TempDir(), t.TempDir()
	var out bytes.Buffer
	runStatus(&out, home, dir, time.Now())
	if !strings.Contains(out.String(), "No cleanups yet") {
		t.Fatalf("empty status:\n%s", out.String())
	}
	state.RecordSnapshot(dir, time.Now().Add(-48*time.Hour), 1e9, 5e9)
	os.WriteFile(filepath.Join(dir, "history.jsonl"), []byte(
		`{"time":"`+time.Now().Format(time.RFC3339)+`","path":"`+home+`/p/node_modules","bytes":2000000000,"rule":"node"}`+"\n"+
			`{"time":"`+time.Now().Format(time.RFC3339)+`","uninstall":"brew:x","bytes":5,"ok":false}`+"\n"), 0o644)
	out.Reset()
	runStatus(&out, home, dir, time.Now())
	for _, want := range []string{"2 days ago: 1.0 GB free", "Recent cleanups", "2.0 GB", "1 items"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("status missing %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	runHistory(&out, home, dir, 10)
	if !strings.Contains(out.String(), "~/p/node_modules") || !strings.Contains(out.String(), "brew:x") {
		t.Errorf("history:\n%s", out.String())
	}
}
