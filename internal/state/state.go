// Package state reads and writes swelo's files in ~/.local/state/swelo:
// history.jsonl (every deletion/uninstall) and snapshots.jsonl (free space over time).
package state

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Record is one line of history.jsonl.
type Record struct {
	Time      time.Time `json:"time"`
	Path      string    `json:"path,omitempty"`
	Rule      string    `json:"rule,omitempty"`
	Uninstall string    `json:"uninstall,omitempty"`
	Bytes     int64     `json:"bytes"`
	OK        *bool     `json:"ok,omitempty"`
}

// Succeeded reports whether the record was a successful action.
func (r Record) Succeeded() bool { return r.OK == nil || *r.OK }

// Snapshot is one line of snapshots.jsonl.
type Snapshot struct {
	Time  time.Time `json:"time"`
	Free  int64     `json:"free"`
	Total int64     `json:"total"`
}

// History reads history.jsonl, oldest first; a missing file is empty.
func History(dir string) []Record {
	var out []Record
	readLines(filepath.Join(dir, "history.jsonl"), func(b []byte) {
		var r Record
		if json.Unmarshal(b, &r) == nil {
			out = append(out, r)
		}
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}

// Snapshots reads snapshots.jsonl, oldest first.
func Snapshots(dir string) []Snapshot {
	var out []Snapshot
	readLines(filepath.Join(dir, "snapshots.jsonl"), func(b []byte) {
		var s Snapshot
		if json.Unmarshal(b, &s) == nil {
			out = append(out, s)
		}
	})
	return out
}

// Record appends a snapshot unless the last one is younger than an hour.
func RecordSnapshot(dir string, now time.Time, free, total int64) {
	snaps := Snapshots(dir)
	if n := len(snaps); n > 0 && now.Sub(snaps[n-1].Time) < time.Hour {
		return
	}
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "snapshots.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(Snapshot{now, free, total})
}

func readLines(path string, fn func([]byte)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		fn(sc.Bytes())
	}
}
