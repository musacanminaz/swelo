package cli

import (
	"bufio"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/musacanminaz/swelo/internal/i18n"
)

func TestIntroShownOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	i18n.Set("en")
	in := bufio.NewReader(strings.NewReader("\n"))

	var first bytes.Buffer
	introGate(in, &first, true)
	if !strings.Contains(first.String(), "What swelo never does") || !strings.Contains(first.String(), "Press Enter") {
		t.Fatalf("intro not shown:\n%s", first.String())
	}
	if _, err := os.Stat(introMarker()); err != nil {
		t.Fatal("marker not written")
	}
	var second bytes.Buffer
	introGate(in, &second, true)
	if second.Len() != 0 {
		t.Fatalf("intro shown twice:\n%s", second.String())
	}
}
