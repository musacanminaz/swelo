package cli

import (
	"testing"

	"github.com/musacanminaz/swelo/internal/clean"
	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/rules"
)

func TestCleanRows(t *testing.T) {
	i18n.Set("en")
	recent := "project changed in the last 7 days"
	results := []clean.Result{
		{Rule: rules.Rule{ID: "node", Kind: "artifact", Risk: rules.Safe}, Size: 10,
			Items: []clean.Item{{Path: "/p/a/node_modules", Size: 10}},
			Skips: []clean.Item{{Path: "/p/b/node_modules", Project: "/p/b", Size: 5, Skip: recent},
				{Path: "/p/c/node_modules", Project: "/p/c", Size: 7, Skip: "tracked by git"}}},
		{Rule: rules.Rule{ID: "pub", Kind: "path", Risk: rules.Risky}, Size: 3, Items: []clean.Item{{Path: "/x", Size: 3}}},
		{Rule: rules.Rule{ID: "report", Kind: "path", Risk: rules.Report}, Size: 9, Items: []clean.Item{{Path: "/y", Size: 9}}},
	}
	items, selected, resolve := cleanRows(results, 7)
	if len(items) != 3 || !selected[0] || selected[1] || selected[2] {
		t.Fatalf("rows %v selected %v (want node ticked, pub and recent node not, no report row)", items, selected)
	}
	chosen, total, rec := resolve(selected)
	if len(chosen) != 1 || total != 10 || rec || len(chosen[0].Items) != 1 {
		t.Fatalf("default: %v %d %v", chosen, total, rec)
	}
	// only the recent row: just the recent project, never the git-tracked one
	chosen, total, rec = resolve([]bool{false, false, true})
	if len(chosen) != 1 || total != 5 || !rec || len(chosen[0].Items) != 1 || chosen[0].Items[0].Path != "/p/b/node_modules" || chosen[0].Items[0].Skip != "" {
		t.Fatalf("recent only: %+v %d %v", chosen, total, rec)
	}
	chosen, total, _ = resolve([]bool{true, true, true})
	if len(chosen) != 2 || total != 18 || len(chosen[0].Items) != 2 {
		t.Fatalf("all: %+v %d", chosen, total)
	}
	if len(results[0].Items) != 1 {
		t.Fatal("resolve changed the scan results")
	}
}
