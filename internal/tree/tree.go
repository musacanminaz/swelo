// Package tree builds a size tree of a folder for the interactive browser.
package tree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Node is a folder or a file. Files under MinFile are folded into Small.
type Node struct {
	Name     string
	Path     string
	Size     int64
	Dir      bool
	Children []*Node
	Parent   *Node
	Small    int   // small files folded into this folder's size
	SmallSz  int64 // their total size
	Denied   bool  // could not be read or skipped (protected)
}

// MinFile is the smallest file listed on its own.
const MinFile = 1 << 20

// Build scans root in parallel. Folders in skip are listed but not read.
func Build(root string, skip []string) *Node {
	n := &Node{Name: root, Path: root, Dir: true}
	sem := make(chan struct{}, 16)
	var wg sync.WaitGroup
	var seen sync.Map
	skipSet := map[string]bool{}
	for _, s := range skip {
		skipSet[s] = true
	}
	var walk func(*Node)
	walk = func(d *Node) {
		defer wg.Done()
		f, err := os.Open(d.Path)
		if err != nil {
			d.Denied = true
			return
		}
		entries, _ := f.Readdir(-1)
		f.Close()
		for _, info := range entries {
			p := filepath.Join(d.Path, info.Name())
			if info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			if info.IsDir() {
				c := &Node{Name: info.Name(), Path: p, Dir: true, Parent: d}
				d.Children = append(d.Children, c)
				if skipSet[p] {
					c.Denied = true
					continue
				}
				wg.Add(1)
				select {
				case sem <- struct{}{}:
					go func() { walk(c); <-sem }()
				default:
					walk(c)
				}
				continue
			}
			st, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				continue
			}
			if st.Nlink > 1 {
				if _, dup := seen.LoadOrStore(st.Ino, true); dup {
					continue
				}
			}
			size := st.Blocks * 512
			if size < MinFile {
				d.Small++
				d.SmallSz += size
				continue
			}
			d.Children = append(d.Children, &Node{Name: info.Name(), Path: p, Size: size, Parent: d})
		}
	}
	wg.Add(1)
	walk(n)
	wg.Wait()
	n.sum()
	return n
}

// sum fills folder sizes bottom-up and sorts children largest first.
func (n *Node) sum() int64 {
	if !n.Dir {
		return n.Size
	}
	n.Size = n.SmallSz
	for _, c := range n.Children {
		n.Size += c.sum()
	}
	sort.Slice(n.Children, func(i, j int) bool { return n.Children[i].Size > n.Children[j].Size })
	return n.Size
}

// Detach removes n from its parent and subtracts its size from every ancestor.
func (n *Node) Detach() {
	p := n.Parent
	if p == nil {
		return
	}
	for i, c := range p.Children {
		if c == n {
			p.Children = append(p.Children[:i], p.Children[i+1:]...)
			break
		}
	}
	for a := p; a != nil; a = a.Parent {
		a.Size -= n.Size
	}
}

// CanTrash reports why path may not be moved to the Trash ("" = allowed).
// Recoverable, but the browser still refuses home, the scan root, anything outside
// home, ~/Library and its direct children, and macOS-protected folders.
func CanTrash(path, root, home string, protected []string) string {
	switch {
	case path == root || path == home:
		return "the folder you are browsing"
	case !strings.HasPrefix(path, home+"/"):
		return "outside your home folder"
	}
	lib := filepath.Join(home, "Library")
	if path == lib || filepath.Dir(path) == lib {
		return "a system part of ~/Library"
	}
	for _, p := range protected {
		if path == p || strings.HasPrefix(path, p+"/") {
			return "protected by macOS"
		}
	}
	return ""
}

// Trash moves path to the user's Trash with macOS's own `trash` tool
// (macOS 15+), or by renaming it into ~/.Trash on older systems.
func Trash(path, home string) error {
	if _, err := exec.LookPath("/usr/bin/trash"); err == nil {
		// path is absolute, so trash cannot read it as a flag
		if out, err := exec.Command("/usr/bin/trash", path).CombinedOutput(); err != nil {
			return fmt.Errorf("trash: %v %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	dst := filepath.Join(home, ".Trash", filepath.Base(path))
	if _, err := os.Lstat(dst); err == nil {
		dst += " " + time.Now().Format("15.04.05")
	}
	return os.Rename(path, dst)
}
