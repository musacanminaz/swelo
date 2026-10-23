package cli

import (
	"encoding/json"
	"errors"
	"io"
	"time"
)

// ErrCancelled is returned when the user declined a confirmation or quit a
// pick list; main exits with status 2 and prints nothing more.
var ErrCancelled = errors.New("cancelled")

// jsonFlag (--json) switches a command to machine-readable output: one JSON
// document on stdout, no plan box, progress or prompts.
var jsonFlag bool

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// JSON shapes. Sizes are bytes; times RFC 3339.

type jsonItem struct {
	Path    string `json:"path"`
	Project string `json:"project,omitempty"`
	Bytes   int64  `json:"bytes"`
	Skipped string `json:"skipped,omitempty"` // why it will not be deleted
}

type jsonRule struct {
	ID          string     `json:"id"`
	Group       string     `json:"group"`
	Risk        string     `json:"risk"`
	Kind        string     `json:"kind"`
	Description string     `json:"description"`
	Bytes       int64      `json:"bytes"`
	Note        string     `json:"note,omitempty"`
	Output      string     `json:"output,omitempty"` // what a command rule would act on
	Items       []jsonItem `json:"items"`
	Skipped     []jsonItem `json:"skipped,omitempty"`
}

type jsonDeleted struct {
	Rule  string `json:"rule"`
	Path  string `json:"path,omitempty"`
	Bytes int64  `json:"bytes,omitempty"`
	Error string `json:"error,omitempty"`
}

type cleanJSON struct {
	DryRun     bool          `json:"dry_run"`
	FreeBytes  int64         `json:"free_bytes"`
	SafeBytes  int64         `json:"safe_bytes"`
	RiskyBytes int64         `json:"risky_bytes"`
	Rules      []jsonRule    `json:"rules"`
	Deleted    []jsonDeleted `json:"deleted,omitempty"`
	FreedBytes int64         `json:"freed_bytes,omitempty"`
	FreeAfter  int64         `json:"free_bytes_after,omitempty"`
}

type statusJSON struct {
	Disk struct {
		TotalBytes  int64   `json:"total_bytes"`
		FreeBytes   int64   `json:"free_bytes"`
		UsedPercent float64 `json:"used_percent"`
	} `json:"disk"`
	DockerImage *jsonDocker `json:"docker_image,omitempty"`
	Trend       *struct {
		Since         time.Time `json:"since"`
		FreeBytesThen int64     `json:"free_bytes_then"`
		DeltaBytes    int64     `json:"delta_bytes"`
	} `json:"trend,omitempty"`
	Cleanups []struct {
		Day   string `json:"day"`
		Bytes int64  `json:"bytes"`
		Items int    `json:"items"`
	} `json:"cleanups"`
	Snapshots []string `json:"apfs_snapshots"` // local APFS snapshot names
}

type jsonCategory struct {
	ID            string `json:"id"`
	Bytes         int64  `json:"bytes"`
	FreeableBytes int64  `json:"freeable_bytes"`
}

type jsonFile struct {
	Path  string    `json:"path"`
	Bytes int64     `json:"bytes"`
	Used  time.Time `json:"last_used"` // later of access and change
	Stale bool      `json:"stale"`     // untouched for 180 days, outside backup folders
}

type jsonProject struct {
	Path             string    `json:"path"`
	RegenerableBytes int64     `json:"regenerable_bytes"`
	Changed          time.Time `json:"changed"`
}

type jsonDocker struct {
	AllocatedBytes int64 `json:"allocated_bytes"`
	MaxBytes       int64 `json:"max_bytes"`
}

type analyzeJSON struct {
	Disk struct {
		TotalBytes int64 `json:"total_bytes"`
		FreeBytes  int64 `json:"free_bytes"`
	} `json:"disk"`
	HomeBytes        int64          `json:"home_bytes"`
	Categories       []jsonCategory `json:"categories"`
	DockerImage      *jsonDocker    `json:"docker_image,omitempty"`
	BigFiles         []jsonFile     `json:"big_files"`
	Projects         []jsonProject  `json:"projects"`
	SkippedProtected []string       `json:"skipped_protected"`
	Unreadable       int            `json:"unreadable_folders"`
}

type jsonPackage struct {
	ID        string     `json:"id"`
	Provider  string     `json:"provider"`
	Name      string     `json:"name"`
	Bytes     int64      `json:"bytes"`
	Installed *time.Time `json:"installed,omitempty"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	UsedBy    []string   `json:"used_by,omitempty"`
	Note      string     `json:"note,omitempty"`
	Command   []string   `json:"command,omitempty"`
	Sudo      bool       `json:"sudo"`
}

type uninstallJSON struct {
	Packages []jsonPackage `json:"packages"`
}
