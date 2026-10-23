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
			Items: []clean.Item{{Path: "/p/a/node_modules", Size: 6}, {Path: "/p/d/node_modules", Size: 4}},
			Skips: []clean.Item{{Path: "/p/b/node_modules", Project: "/p/b", Size: 5, Skip: recent},
				{Path: "/p/c/node_modules", Project: "/p/c", Size: 7, Skip: "tracked by git"}}},
		{Rule: rules.Rule{ID: "pub", Kind: "path", Risk: rules.Risky}, Size: 3, Items: []clean.Item{{Path: "/x", Size: 3}}},
		{Rule: rules.Rule{ID: "brew", Kind: "command", Risk: rules.Safe}, Size: 2},
		{Rule: rules.Rule{ID: "report", Kind: "path", Risk: rules.Report}, Size: 9, Items: []clean.Item{{Path: "/y", Size: 9}}},
	}
	items, sub, resolve := cleanRows(results, 7, "/home")
	// rows: node (2 children), brew (no children), pub (1 child), recent node (1 child); no report row
	if len(items) != 4 || len(items[0].Children) != 2 || len(items[1].Children) != 0 || len(items[3].Children) != 1 {
		t.Fatalf("rows %+v", items)
	}
	want := [][]bool{{true, true}, {true}, {false}, {false}}
	for i := range want {
		for j := range want[i] {
			if sub[i][j] != want[i][j] {
				t.Fatalf("initial ticks %v, want %v", sub, want)
			}
		}
	}
	chosen, total, rec := resolve(sub)
	if len(chosen) != 2 || total != 12 || rec || len(chosen[0].Items) != 2 || chosen[1].Rule.ID != "brew" || chosen[1].Size != 2 {
		t.Fatalf("default: %+v %d %v", chosen, total, rec)
	}
	// one node child unticked, the recent project ticked: never the git-tracked one
	chosen, total, rec = resolve([][]bool{{false, true}, {false}, {false}, {true}})
	if len(chosen) != 1 || total != 9 || !rec || len(chosen[0].Items) != 2 ||
		chosen[0].Items[0].Path != "/p/d/node_modules" || chosen[0].Items[1].Path != "/p/b/node_modules" || chosen[0].Items[1].Skip != "" {
		t.Fatalf("partial: %+v %d %v", chosen, total, rec)
	}
	if len(results[0].Items) != 2 || results[0].Size != 10 {
		t.Fatal("resolve changed the scan results")
	}
}
