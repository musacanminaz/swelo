package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotsAndHistory(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	RecordSnapshot(dir, now.Add(-2*time.Hour), 100, 1000)
	RecordSnapshot(dir, now.Add(-90*time.Minute), 110, 1000) // < 1h after the last: skipped
	RecordSnapshot(dir, now, 200, 1000)
	if s := Snapshots(dir); len(s) != 2 || s[1].Free != 200 {
		t.Fatalf("snapshots = %+v", s)
	}
	os.WriteFile(filepath.Join(dir, "history.jsonl"), []byte(
		`{"time":"2026-10-20T10:00:00Z","path":"/a","bytes":5,"rule":"node"}`+"\n"+
			`not json`+"\n"+
			`{"time":"2026-10-19T10:00:00Z","uninstall":"brew:x","bytes":7,"ok":false}`+"\n"), 0o644)
	h := History(dir)
	if len(h) != 2 || h[0].Uninstall != "brew:x" || h[0].Succeeded() || !h[1].Succeeded() {
		t.Fatalf("history = %+v", h)
	}
}
