package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/musacanminaz/swelo/internal/analyze"
	"github.com/musacanminaz/swelo/internal/clean"
	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/perm"
	"github.com/musacanminaz/swelo/internal/rules"
	"github.com/musacanminaz/swelo/internal/ui"
	"github.com/spf13/cobra"
)

func analyzeCmd() *cobra.Command {
	var full, browse, dupes bool
	var top int
	c := &cobra.Command{
		Use:   "analyze [folder]",
		Short: i18n.T("cmd.analyze"),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := home()
			if len(args) == 1 {
				dir = args[0]
			}
			if dupes {
				if jsonFlag {
					return runDupes(bufio.NewReader(os.Stdin), io.Discard, home(), dir, full, false, cmd.OutOrStdout())
				}
				return runDupes(bufio.NewReader(os.Stdin), cmd.OutOrStdout(), home(), dir, full, stdinIsTerminal(), nil)
			}
			if browse || len(args) == 1 {
				return runTree(bufio.NewReader(os.Stdin), cmd.OutOrStdout(), home(), dir, full)
			}
			if jsonFlag {
				return runAnalyze(bufio.NewReader(os.Stdin), io.Discard, home(), full, top, false, cmd.OutOrStdout())
			}
			return runAnalyze(bufio.NewReader(os.Stdin), cmd.OutOrStdout(), home(), full, top, stdinIsTerminal(), nil)
		},
	}
	c.Flags().BoolVar(&full, "full", false, i18n.T("flag.analyze.full"))
	c.Flags().BoolVar(&browse, "tree", false, i18n.T("flag.analyze.tree"))
	c.Flags().BoolVar(&dupes, "dupes", false, i18n.T("flag.analyze.dupes"))
	c.Flags().IntVar(&top, "top", 10, i18n.T("flag.analyze.top"))
	return c
}

// runAnalyze prints the report to w, or writes it as JSON to jsonOut when set.
func runAnalyze(in *bufio.Reader, w io.Writer, home string, full bool, top int, interactive bool, jsonOut io.Writer) error {
	introGate(in, w, interactive)
	rs, cfg, err := rules.Load(home)
	if err != nil {
		return err
	}
	fda := perm.FullDiskAccess(home)
	permText := i18n.T("plan.perm.analyze")
	if full {
		permText = i18n.T("plan.perm.full")
		if !fda {
			fmt.Fprintf(w, "\n %s %s\n", ui.Yellow("!"), i18n.T("analyze.nofda"))
			full = false
		}
	}
	ui.ShowPlan(w, ui.Plan{
		Title:    "swelo analyze",
		Scan:     []string{"~"},
		Read:     i18n.T("plan.read.analyze"),
		Delete:   i18n.T("plan.delete.none"),
		Perm:     permText,
		Duration: i18n.T("plan.time.analyze"),
	})
	fmt.Fprintf(w, " %s\n", ui.Dim(i18n.T("scanning")))

	eng := clean.New(home, rs, cfg)
	eng.FDA = fda
	roots := rules.ProjectRoots(cfg, home)
	cats := analyze.Categories(home, roots)

	done := make(chan []clean.Result)
	go func() { done <- eng.Scan() }()
	start := time.Now()
	prog := ui.NewStatus(w)
	res := analyze.Scan(analyze.Options{
		Home: home, Categories: cats, Protected: perm.ProtectedDirs(home), Full: full, BigFile: 1e9,
		Progress: watchDirs(prog, home),
	})
	var n int64
	for _, v := range res.Sizes {
		n += v
	}
	prog.Step(i18n.T("step.home"), ui.Bytes(n), len(res.BigFiles), res.Unreadable)
	prog.Set("%s", i18n.T("prog.freeable"))
	results := <-done
	prog.Step("%s", i18n.T("step.freeable"))
	prog.Stop()

	// what clean could free, per category
	freeable := map[string]int64{}
	for _, r := range results {
		if !clean.Selected(r, false) {
			continue
		}
		for _, it := range r.Items {
			freeable[analyze.Classify(it.Path, cats)] += it.Size
		}
		if r.Rule.Kind == "command" && r.Size > 0 {
			id := analyze.Classify(r.Show, cats)
			if r.Rule.Group == "docker" {
				id = "docker"
			}
			freeable[id] += r.Size
		}
	}

	var st syscall.Statfs_t
	syscall.Statfs(home, &st)
	total, free := int64(st.Blocks)*int64(st.Bsize), int64(st.Bavail)*int64(st.Bsize)
	if jsonOut != nil {
		doc := analyzeDoc(res, results, freeable, rs, home, fda || full)
		doc.Disk.TotalBytes, doc.Disk.FreeBytes = total, free
		return writeJSON(jsonOut, doc)
	}
	fmt.Fprintf(w, "\n %s %s / %s — %s %s  %s\n\n", ui.Bold(i18n.T("disk")), ui.Bytes(total-free), ui.Bytes(total),
		i18n.T("disk.free"), ui.Bold(ui.Bytes(free)), ui.Dim(fmt.Sprintf("(%.0fs)", time.Since(start).Seconds())))

	ids := make([]string, 0, len(res.Sizes))
	var homeTotal int64
	for id, n := range res.Sizes {
		ids = append(ids, id)
		homeTotal += n
	}
	sort.Slice(ids, func(i, j int) bool { return res.Sizes[ids[i]] > res.Sizes[ids[j]] })
	fmt.Fprintf(w, "   %s %10s  %s\n", ui.Pad(ui.Bold(i18n.T("col.category")), 40), i18n.T("col.size"), i18n.T("col.freeable"))
	for _, id := range ids {
		if res.Sizes[id] < 1e8 && freeable[id] == 0 {
			continue // hide categories under 100 MB
		}
		f := ""
		if freeable[id] > 0 {
			f = ui.Green(ui.Bytes(freeable[id]))
		}
		fmt.Fprintf(w, "   %s %10s  %s\n", ui.Pad(i18n.T("cat."+id), 40), ui.Bytes(res.Sizes[id]), f)
	}
	fmt.Fprintf(w, "   %s %10s\n", ui.Pad(ui.Dim(i18n.T("col.total")), 40), ui.Dim(ui.Bytes(homeTotal)))
	if dr := dockerImage(home, fda || full); dr != "" {
		fmt.Fprintf(w, "   %s\n", ui.Dim(dr))
	}
	if len(res.Skipped) > 0 {
		fmt.Fprintf(w, "\n   %s\n", ui.Dim(fmt.Sprintf(i18n.T("analyze.skipped"), len(res.Skipped))))
	}

	if len(res.BigFiles) > 0 {
		fmt.Fprintf(w, "\n %s\n", ui.Bold(i18n.T("analyze.bigfiles")))
		for i, f := range res.BigFiles {
			if i == top {
				fmt.Fprintf(w, "   %s\n", ui.Dim(fmt.Sprintf(i18n.T("more.top"), len(res.BigFiles)-top)))
				break
			}
			fmt.Fprintf(w, "   %9s  %s\n", ui.Bytes(f.Size), short(f.Path, home))
		}
		printStaleFiles(w, res.BigFiles, top, home)
	}

	printProjects(w, projectRows(results, rs), top, home)
	fmt.Fprintf(w, "\n %s\n\n", ui.Dim(i18n.T("analyze.next")))
	return nil
}

// staleAge: a big file untouched this long is worth a look (report only).
const staleAge = 180 * 24 * time.Hour

// printStaleFiles lists the big files nobody opened or changed for half a
// year, backup folders excluded; nothing is deleted.
func printStaleFiles(w io.Writer, files []analyze.File, top int, home string) {
	now := time.Now()
	var stale []analyze.File
	for _, f := range files {
		if f.Stale(now, staleAge) {
			stale = append(stale, f)
		}
	}
	if len(stale) == 0 {
		return
	}
	fmt.Fprintf(w, "\n %s %s\n", ui.Bold(i18n.T("analyze.stale")), ui.Dim(i18n.T("analyze.stale.note")))
	for i, f := range stale {
		if i == top {
			fmt.Fprintf(w, "   %s\n", ui.Dim(fmt.Sprintf(i18n.T("more.top"), len(stale)-top)))
			break
		}
		fmt.Fprintf(w, "   %9s  %s %s\n", ui.Bytes(f.Size), ui.Pad(short(f.Path, home), 60), ui.Dim(fmt.Sprintf(i18n.T("days.ago"), int(now.Sub(f.Used).Hours()/24))))
	}
}

// analyzeDoc is the --json form of the report (every category, no top cut-off).
func analyzeDoc(res analyze.Result, results []clean.Result, freeable map[string]int64, rs []rules.Rule, home string, docker bool) analyzeJSON {
	var doc analyzeJSON
	doc.Categories, doc.BigFiles, doc.Projects = []jsonCategory{}, []jsonFile{}, []jsonProject{}
	for id, n := range res.Sizes {
		doc.HomeBytes += n
		doc.Categories = append(doc.Categories, jsonCategory{ID: id, Bytes: n, FreeableBytes: freeable[id]})
	}
	sort.Slice(doc.Categories, func(i, j int) bool { return doc.Categories[i].Bytes > doc.Categories[j].Bytes })
	if alloc, maxSize, ok := dockerRaw(home, docker); ok {
		doc.DockerImage = &jsonDocker{alloc, maxSize}
	}
	now := time.Now()
	for _, f := range res.BigFiles {
		doc.BigFiles = append(doc.BigFiles, jsonFile{f.Path, f.Size, f.Used, f.Stale(now, staleAge)})
	}
	for _, r := range projectRows(results, rs) {
		doc.Projects = append(doc.Projects, jsonProject{r.dir, r.size, r.changed})
	}
	doc.SkippedProtected = append([]string{}, res.Skipped...)
	doc.Unreadable = res.Unreadable
	return doc
}

// dockerImage describes Docker Desktop's disk image: allocated vs. maximum size.
func dockerImage(home string, allowed bool) string {
	alloc, maxSize, ok := dockerRaw(home, allowed)
	if !ok {
		return ""
	}
	return fmt.Sprintf(i18n.T("analyze.docker"), ui.Bytes(alloc), ui.Bytes(maxSize))
}

// dockerRaw: allocated and maximum size of Docker Desktop's disk image.
func dockerRaw(home string, allowed bool) (alloc, maxSize int64, ok bool) {
	if !allowed {
		return 0, 0, false
	}
	p := filepath.Join(home, "Library", "Containers", "com.docker.docker", "Data", "vms", "0", "data", "Docker.raw")
	info, err := os.Stat(p)
	if err != nil {
		return 0, 0, false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return st.Blocks * 512, info.Size(), true
}

type projectRow struct {
	dir     string
	size    int64
	changed time.Time
}

// printProjects lists projects by regenerable size, with their last change.
// projectRows sums each project's regenerable data (found or skipped) and
// finds its last change; projects under 10 MB are left out, largest first.
func projectRows(results []clean.Result, rs []rules.Rule) []*projectRow {
	skip := map[string]bool{}
	for _, r := range rs {
		for _, n := range r.Names {
			skip[n] = true
		}
	}
	byDir := map[string]*projectRow{}
	for _, r := range results {
		if r.Rule.Kind != "artifact" {
			continue
		}
		for _, it := range append(append([]clean.Item{}, r.Items...), r.Skips...) {
			row := byDir[it.Project]
			if row == nil {
				row = &projectRow{dir: it.Project, changed: lastChange(it.Project, skip)}
				byDir[it.Project] = row
			}
			row.size += it.Size
		}
	}
	var rows []*projectRow
	for _, r := range byDir {
		if r.size >= 1e7 {
			rows = append(rows, r)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].size > rows[j].size })
	return rows
}

func printProjects(w io.Writer, rows []*projectRow, top int, home string) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(w, "\n %s\n", ui.Bold(i18n.T("analyze.projects")))
	for i, r := range rows {
		if i == top {
			fmt.Fprintf(w, "   %s\n", ui.Dim(fmt.Sprintf(i18n.T("more.top"), len(rows)-top)))
			break
		}
		days := int(time.Since(r.changed).Hours() / 24)
		age := fmt.Sprintf(i18n.T("analyze.daysago"), days)
		if days == 0 {
			age = i18n.T("analyze.today")
		}
		if days > 90 {
			age = ui.Yellow(age)
		}
		fmt.Fprintf(w, "   %9s  %s %s\n", ui.Bytes(r.size), ui.Pad(short(r.dir, home), 60), age)
	}
}

// lastChange is the newest modification time of a project's top-level entries,
// ignoring regenerable folders.
func lastChange(dir string, skip map[string]bool) time.Time {
	var newest time.Time
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if skip[e.Name()] {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}
