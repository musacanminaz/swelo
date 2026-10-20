package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/musacanminaz/swelo/internal/clean"
	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/ui"
)

func TestWatchScanStepLines(t *testing.T) {
	i18n.Set("en")
	var buf bytes.Buffer
	st := ui.NewStatus(&buf)
	eng := &clean.Engine{}
	done := watchScan(eng, st, "/home")
	for _, p := range []clean.Progress{
		{Stage: "rule", Detail: "brew", N: 1, Total: 3},
		{Stage: "rule", Detail: "uv", N: 2, Total: 3},
		{Stage: "walk", Detail: "/home/p", N: 40, Found: 2},
		{Stage: "measure", N: 2, Total: 2},
		{Stage: "check", N: 2, Total: 2},
	} {
		eng.OnProgress(p)
	}
	done()
	st.Stop()
	out := buf.String()
	for _, want := range []string{"2 cache and tool rules", "40 folders read, 2 build artifacts", "sizes measured: 2", "safety checks done: 2"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\r") {
		t.Error("live line drawn off a terminal")
	}
}
