package analyze

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFindDupes(t *testing.T) {
	root := t.TempDir()
	big := bytes.Repeat([]byte("abcdefgh"), 1<<17) // 1 MB
	write := func(name string, data []byte) string {
		p := filepath.Join(root, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, data, 0o644)
		return p
	}
	a := write("a/movie.mkv", big)
	b := write("b/copy of movie.mkv", big)
	other := append(append([]byte{}, big...), 0)
	other[len(other)/2] ^= 1 // same size as "same" below but different in the middle: the quick hash misses it
	same := append(append([]byte{}, big...), 0)
	c := write("c/x.bin", same)
	d := write("d/y.bin", same)
	write("e/z.bin", other)
	write("small.txt", []byte("tiny"))
	write("skip/movie.mkv", big) // in a folder that is not entered
	os.Link(a, filepath.Join(root, "a", "hardlink.mkv"))
	var progressed bool
	groups := FindDupes(root, DupeOptions{MinSize: 1024, Workers: 2,
		Skip:     func(dir string) bool { return filepath.Base(dir) == "skip" },
		Progress: func(int, int) { progressed = true }})
	if len(groups) != 2 {
		t.Fatalf("groups: %+v", groups)
	}
	hardlink := filepath.Join(root, "a", "hardlink.mkv")
	for _, g := range groups {
		switch g.Files[0] {
		case a, hardlink: // one name of the hard-linked pair, whichever the walk met first
			if len(g.Files) != 2 || g.Files[1] != b || g.Wasted() != int64(len(big)) {
				t.Errorf("movie group: %+v", g)
			}
		case c:
			if len(g.Files) != 2 || g.Files[1] != d {
				t.Errorf("x/y group: %+v", g)
			}
		default:
			t.Errorf("unexpected group %+v", g)
		}
	}
	if !progressed {
		t.Error("no progress reported")
	}
}
