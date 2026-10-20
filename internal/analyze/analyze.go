// Package analyze measures where disk space in the home folder went, in one
// parallel walk, without deleting anything.
package analyze

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

// Category groups paths under one name. The longest matching prefix wins.
type Category struct {
	ID    string
	Paths []string // absolute
}

// Result of a scan.
type Result struct {
	Sizes      map[string]int64 // category id -> allocated bytes
	BigFiles   []File           // files >= BigFile bytes, largest first
	Skipped    []string         // TCC-protected folders not read
	Unreadable int              // folders that could not be read
}

// File is one large file.
type File struct {
	Path string
	Size int64
}

// Options for Scan.
type Options struct {
	Home       string
	Categories []Category
	Protected  []string // read only when Full is set (Full Disk Access folders)
	Full       bool
	BigFile    int64
	Workers    int
	// Progress, if set, is called from scan goroutines as each directory is
	// read, with running totals; it must be safe for concurrent use.
	Progress func(dir string, dirs, files, bytes int64)
}

type scanner struct {
	o        Options
	prefixes []prefix
	mu       sync.Mutex
	res      Result
	sem      chan struct{}
	wg       sync.WaitGroup
	seen     sync.Map // inode -> struct{} for hard links

	dirs, files, bytes atomic.Int64
}

type prefix struct{ path, id string }

// Scan walks o.Home. Everything not claimed by a category is counted as "other".
func Scan(o Options) Result {
	if o.Workers == 0 {
		o.Workers = 16
	}
	s := &scanner{o: o, sem: make(chan struct{}, o.Workers)}
	s.res.Sizes = map[string]int64{}
	for _, c := range o.Categories {
		for _, p := range c.Paths {
			s.prefixes = append(s.prefixes, prefix{filepath.Clean(p), c.ID})
		}
	}
	sort.Slice(s.prefixes, func(i, j int) bool { return len(s.prefixes[i].path) > len(s.prefixes[j].path) })
	s.wg.Add(1)
	s.walk(o.Home, s.category(o.Home, "other"))
	s.wg.Wait()
	sort.Slice(s.res.BigFiles, func(i, j int) bool { return s.res.BigFiles[i].Size > s.res.BigFiles[j].Size })
	sort.Strings(s.res.Skipped)
	return s.res
}

// category returns the category of dir: its own prefix match, else inherited.
func (s *scanner) category(dir, inherited string) string {
	for _, p := range s.prefixes {
		if dir == p.path {
			return p.id
		}
	}
	return inherited
}

func (s *scanner) protected(dir string) bool {
	for _, p := range s.o.Protected {
		if dir == p {
			return true
		}
	}
	return false
}

func (s *scanner) walk(dir, cat string) {
	defer s.wg.Done()
	f, err := os.Open(dir)
	if err != nil {
		s.mu.Lock()
		s.res.Unreadable++
		s.mu.Unlock()
		return
	}
	entries, _ := f.Readdir(-1)
	f.Close()
	var local, nfiles int64
	for _, info := range entries {
		p := filepath.Join(dir, info.Name())
		mode := info.Mode()
		switch {
		case mode&os.ModeSymlink != 0:
			continue
		case mode.IsDir():
			if !s.o.Full && s.protected(p) {
				s.mu.Lock()
				s.res.Skipped = append(s.res.Skipped, p)
				s.mu.Unlock()
				continue
			}
			sub := s.category(p, cat)
			s.wg.Add(1)
			select {
			case s.sem <- struct{}{}:
				go func() { s.walk(p, sub); <-s.sem }()
			default:
				s.walk(p, sub)
			}
		default:
			st, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				continue
			}
			if st.Nlink > 1 {
				if _, dup := s.seen.LoadOrStore(st.Ino, struct{}{}); dup {
					continue
				}
			}
			size := st.Blocks * 512
			local += size
			nfiles++
			if s.o.BigFile > 0 && size >= s.o.BigFile {
				s.mu.Lock()
				s.res.BigFiles = append(s.res.BigFiles, File{p, size})
				s.mu.Unlock()
			}
		}
	}
	s.mu.Lock()
	s.res.Sizes[cat] += local
	s.mu.Unlock()
	if s.o.Progress != nil {
		s.o.Progress(dir, s.dirs.Add(1), s.files.Add(nfiles), s.bytes.Add(local))
	}
}

// Classify returns the category id for an arbitrary path (longest prefix).
func Classify(path string, cats []Category) string {
	best, bestLen := "other", -1
	for _, c := range cats {
		for _, p := range c.Paths {
			if (path == p || strings.HasPrefix(path, p+"/")) && len(p) > bestLen {
				best, bestLen = c.ID, len(p)
			}
		}
	}
	return best
}
