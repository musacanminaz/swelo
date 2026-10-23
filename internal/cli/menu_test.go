package cli

import (
	"testing"
	"time"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/state"
)

func TestLastCleanup(t *testing.T) {
	i18n.Set("en")
	if got := lastCleanup(nil); got != "no cleanup yet" {
		t.Fatalf("empty: %q", got)
	}
	ok, failed := true, false
	now := time.Now()
	old := now.AddDate(0, 0, -10)
	hist := []state.Record{
		{Time: old, Bytes: 100, OK: &ok},
		{Time: now.Add(-2 * time.Hour), Bytes: 5, OK: &ok},
		{Time: now.Add(-time.Hour), Bytes: 7, OK: &failed}, // failed: not counted
		{Time: now.Add(-time.Hour), Bytes: 3, OK: &ok},
	}
	if got := lastCleanup(hist); got != "last cleanup today · 8 B" {
		t.Fatalf("got %q", got)
	}
}
