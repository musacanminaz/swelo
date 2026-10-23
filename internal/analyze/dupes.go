package analyze

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
)

// Group is a set of identical files (same size and SHA-256).
type Group struct {
	Size  int64    `json:"bytes"` // of one copy
	Files []string `json:"files"` // every copy, sorted
}

// Wasted is what the extra copies take.
func (g Group) Wasted() int64 { return g.Size * int64(len(g.Files)-1) }

// DupeOptions for FindDupes.
type DupeOptions struct {
	MinSize  int64                 // files smaller than this are ignored
	Skip     func(dir string) bool // folders not to enter
	Progress func(files int, hashing int)
	Workers  int
}

// FindDupes walks root for files of at least MinSize and returns the groups
// of identical ones, largest waste first. Same-size files are compared by a
// hash of their first and last 4 MB, and only matches are hashed in full.
// Hard links to one file count once.
func FindDupes(root string, o DupeOptions) []Group {
	if o.Workers == 0 {
		o.Workers = 4
	}
	bySize := map[int64][]string{}
	seen := map[uint64]bool{}
	files := 0
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && o.Skip != nil && o.Skip(p) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 || !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() < o.MinSize {
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Nlink > 1 {
			if seen[st.Ino] {
				return nil
			}
			seen[st.Ino] = true
		}
		files++
		if o.Progress != nil && files%200 == 0 {
			o.Progress(files, 0)
		}
		bySize[info.Size()] = append(bySize[info.Size()], p)
		return nil
	})
	// candidates: sizes with more than one file
	type job struct {
		size int64
		path string
	}
	var jobs []job
	for size, ps := range bySize {
		if len(ps) > 1 {
			for _, p := range ps {
				jobs = append(jobs, job{size, p})
			}
		}
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].path < jobs[j].path })
	var mu sync.Mutex
	byKey := map[string][]string{} // size+quick hash (+ full hash) -> files
	var wg sync.WaitGroup
	sem := make(chan struct{}, o.Workers)
	hashed := 0
	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j job) {
			defer wg.Done()
			defer func() { <-sem }()
			quick, err := hashFile(j.path, j.size, 4<<20)
			if err != nil {
				return
			}
			mu.Lock()
			hashed++
			n := hashed
			byKey[quick] = append(byKey[quick], j.path)
			mu.Unlock()
			if o.Progress != nil {
				o.Progress(files, n)
			}
		}(j)
	}
	wg.Wait()
	// full hash where the quick hash collides
	full := map[string][]string{}
	for _, ps := range byKey {
		if len(ps) < 2 {
			continue
		}
		for _, p := range ps {
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			h, err := hashFile(p, info.Size(), 0)
			if err != nil {
				continue
			}
			full[h] = append(full[h], p)
		}
	}
	var groups []Group
	for _, ps := range full {
		if len(ps) < 2 {
			continue
		}
		sort.Strings(ps)
		info, err := os.Stat(ps[0])
		if err != nil {
			continue
		}
		groups = append(groups, Group{Size: info.Size(), Files: ps})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Wasted() != groups[j].Wasted() {
			return groups[i].Wasted() > groups[j].Wasted()
		}
		return groups[i].Files[0] < groups[j].Files[0]
	})
	return groups
}

// hashFile hashes the size plus the first and last `edge` bytes of the
// file; edge 0 hashes the whole file.
func hashFile(path string, size, edge int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	var buf [8]byte
	for i := 0; i < 8; i++ {
		buf[i] = byte(size >> (8 * i))
	}
	h.Write(buf[:])
	if edge == 0 || size <= 2*edge {
		if _, err := io.Copy(h, f); err != nil {
			return "", err
		}
	} else {
		if _, err := io.CopyN(h, f, edge); err != nil {
			return "", err
		}
		if _, err := f.Seek(-edge, io.SeekEnd); err != nil {
			return "", err
		}
		if _, err := io.CopyN(h, f, edge); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
