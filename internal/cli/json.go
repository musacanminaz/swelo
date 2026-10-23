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
	DockerImage *struct {
		AllocatedBytes int64 `json:"allocated_bytes"`
		MaxBytes       int64 `json:"max_bytes"`
	} `json:"docker_image,omitempty"`
	Trend *struct {
		Since         time.Time `json:"since"`
		FreeBytesThen int64     `json:"free_bytes_then"`
		DeltaBytes    int64     `json:"delta_bytes"`
	} `json:"trend,omitempty"`
	Cleanups []struct {
		Day   string `json:"day"`
		Bytes int64  `json:"bytes"`
		Items int    `json:"items"`
	} `json:"cleanups"`
}
