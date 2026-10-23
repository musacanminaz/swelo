package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/musacanminaz/swelo/internal/i18n"
)

// PickItem is one row of a multi-select list. An item with Children expands
// (→) into them so single children can be unticked; its own tick then means
// "all children". Size, when set anywhere in the list, is shown as a column
// and summed in the header.
type PickItem struct {
	Group, Label, Detail string
	Size                 int64
	Children             []PickItem
}

// Picker is a bubbletea multi-select list. After Run, Selected says what was
// ticked (for an item with children: at least one child) and Sub which
// children; Cancelled is true if the user quit without confirming. With
// Single, Enter picks the row under the cursor (a menu).
//
// Keys: ↑↓/jk move, PgUp/PgDn/Home/End jump, → ← expand/collapse, space
// ticks, a ticks every visible row, n unticks them, / filters rows as you
// type (Enter keeps the filter, Esc clears it), Enter confirms, q cancels.
type Picker struct {
	Title     string
	Items     []PickItem
	Selected  []bool
	Sub       [][]bool // Sub[i][j]: child j of item i is ticked (nil without children)
	Cancelled bool
	Single    bool
	cursor    int   // index into visible
	visible   []row // rows that match the filter
	expanded  []bool
	filter    string
	typing    bool // keys edit the filter
	top       int
	height    int
	done      bool
}

// row is a visible line: an item, or child of an item (child ≥ 0).
type row struct{ item, child int }

// NewPicker builds a picker with nothing selected.
func NewPicker(title string, items []PickItem) *Picker {
	p := &Picker{Title: title, Items: items, Selected: make([]bool, len(items)),
		Sub: make([][]bool, len(items)), expanded: make([]bool, len(items)), height: 20}
	for i, it := range items {
		if len(it.Children) > 0 {
			p.Sub[i] = make([]bool, len(it.Children))
		}
	}
	p.refilter()
	return p
}

func (p *Picker) Init() tea.Cmd { return nil }

func (p *Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.height = max(3, m.Height-6)
	case tea.KeyMsg:
		if p.typing && p.filterKey(m) {
			break
		}
		switch m.String() {
		case "q", "esc", "ctrl+c":
			p.Cancelled, p.done = true, true
			return p, tea.Quit
		case "enter":
			if p.Single && len(p.visible) > 0 {
				p.Selected[p.visible[p.cursor].item] = true
			}
			p.done = true
			return p, tea.Quit
		case "up", "k":
			p.cursor--
		case "down", "j":
			p.cursor++
		case "pgup":
			p.cursor -= p.height
		case "pgdown":
			p.cursor += p.height
		case "home":
			p.cursor = 0
		case "end":
			p.cursor = len(p.visible) - 1
		case "right", "l", "left", "h":
			if len(p.visible) > 0 && !p.Single {
				r := p.visible[p.cursor]
				if open := m.String() == "right" || m.String() == "l"; len(p.Items[r.item].Children) > 0 && p.expanded[r.item] != open {
					p.expanded[r.item] = open
					p.refilter()
					for v, vr := range p.visible { // stay on the item
						if vr.item == r.item && vr.child < 0 {
							p.cursor = v
						}
					}
				}
			}
		case " ", "x":
			if len(p.visible) > 0 && !p.Single {
				r := p.visible[p.cursor]
				p.set(r, !p.ticked(r))
			}
		case "a", "n":
			if !p.Single {
				for _, r := range p.visible {
					p.set(r, m.String() == "a")
				}
			}
		case "/":
			if !p.Single {
				p.typing = true
			}
		}
	}
	p.cursor = max(0, min(p.cursor, len(p.visible)-1))
	if p.cursor < p.top {
		p.top = p.cursor
	}
	if p.cursor >= p.top+p.height {
		p.top = p.cursor - p.height + 1
	}
	return p, nil
}

// ticked: a child's own tick; an item with children counts as ticked when
// every child is.
func (p *Picker) ticked(r row) bool {
	if r.child >= 0 {
		return p.Sub[r.item][r.child]
	}
	if len(p.Sub[r.item]) > 0 {
		for _, v := range p.Sub[r.item] {
			if !v {
				return false
			}
		}
		return true
	}
	return p.Selected[r.item]
}

// set ticks or unticks a row; on an item with children, all of them.
func (p *Picker) set(r row, v bool) {
	switch {
	case r.child >= 0:
		p.Sub[r.item][r.child] = v
	case len(p.Sub[r.item]) > 0:
		for j := range p.Sub[r.item] {
			p.Sub[r.item][j] = v
		}
	default:
		p.Selected[r.item] = v
		return
	}
	any := false
	for _, c := range p.Sub[r.item] {
		any = any || c
	}
	p.Selected[r.item] = any
}

// filterKey handles a key while the filter is being typed; false hands the
// key to the normal bindings (navigation keys still work).
func (p *Picker) filterKey(m tea.KeyMsg) bool {
	switch m.Type {
	case tea.KeyEsc:
		p.filter, p.typing = "", false
	case tea.KeyEnter:
		p.typing = false
	case tea.KeyBackspace:
		if p.filter != "" {
			_, n := utf8.DecodeLastRuneInString(p.filter)
			p.filter = p.filter[:len(p.filter)-n]
		}
	case tea.KeyRunes:
		p.filter += string(m.Runes)
	case tea.KeySpace:
		p.filter += " "
	default:
		return false
	}
	p.refilter()
	return true
}

// refilter recomputes the visible rows (expanded items are followed by their
// children) and keeps the cursor on a valid one.
func (p *Picker) refilter() {
	p.visible = p.visible[:0]
	f := strings.ToLower(p.filter)
	for i, it := range p.Items {
		text := it.Label + " " + it.Detail + " " + it.Group
		for _, c := range it.Children {
			text += " " + c.Label
		}
		if f != "" && !strings.Contains(strings.ToLower(text), f) {
			continue
		}
		p.visible = append(p.visible, row{i, -1})
		if p.expanded[i] {
			for j := range it.Children {
				p.visible = append(p.visible, row{i, j})
			}
		}
	}
	p.cursor = max(0, min(p.cursor, len(p.visible)-1))
	p.top = 0
}

// size of an item: its own, or the sum of its children's.
func (p *Picker) size(i int) int64 {
	it := p.Items[i]
	if len(it.Children) == 0 {
		return it.Size
	}
	var n int64
	for _, c := range it.Children {
		n += c.Size
	}
	return n
}

func (p *Picker) View() string {
	var s strings.Builder
	if p.Single {
		return p.menuView()
	}
	n, sizes, expandable := 0, false, false
	var total int64
	for i, it := range p.Items {
		sizes = sizes || p.size(i) > 0
		expandable = expandable || len(it.Children) > 0
		switch {
		case len(p.Sub[i]) > 0:
			for j, v := range p.Sub[i] {
				if v {
					n++
					total += it.Children[j].Size
				}
			}
		case p.Selected[i]:
			n++
			total += it.Size
		}
	}
	count := fmt.Sprintf(i18n.T("pick.count"), n)
	if sizes {
		count += " · " + Bytes(total)
	}
	help := i18n.T("pick.help")
	switch {
	case p.typing:
		help = i18n.T("pick.help.filter")
	case expandable:
		help = i18n.T("pick.help.expand") + " · " + help
	}
	fmt.Fprintf(&s, " %s  %s\n %s\n", Bold(p.Title), Dim(count), Dim(help))
	if p.typing || p.filter != "" {
		cur := ""
		if p.typing {
			cur = "_"
		}
		fmt.Fprintf(&s, " / %s%s  %s\n", Safe(p.filter), cur, Dim(fmt.Sprintf(i18n.T("pick.matches"), len(p.visible), len(p.Items))))
	} else {
		s.WriteString("\n")
	}
	width := 0
	for _, r := range p.visible {
		if r.child < 0 {
			width = max(width, utf8.RuneCountInString(p.Items[r.item].Label))
		}
	}
	width = min(width, 56)
	end := min(len(p.visible), p.top+p.height)
	for v := p.top; v < end; v++ {
		r := p.visible[v]
		it := p.Items[r.item]
		if r.child < 0 && (v == p.top || it.Group != p.Items[p.visible[v-1].item].Group) {
			fmt.Fprintf(&s, " %s\n", Green(it.Group))
		}
		var line string
		if r.child >= 0 {
			c := it.Children[r.child]
			box := "[ ]"
			if p.Sub[r.item][r.child] {
				box = Green("[x]")
			}
			line = "    " + box + " "
			if sizes {
				line += fmt.Sprintf("%9s  ", Bytes(c.Size))
			}
			line += c.Label
			if c.Detail != "" {
				line += "  " + Dim(c.Detail)
			}
		} else {
			box := "[ ]"
			switch {
			case p.ticked(r):
				box = Green("[x]")
			case p.Selected[r.item]: // some children
				box = Yellow("[-]")
			}
			arrow := "  "
			if len(it.Children) > 0 {
				arrow = "▸ "
				if p.expanded[r.item] {
					arrow = "▾ "
				}
			}
			line = box + " "
			if sizes {
				line += fmt.Sprintf("%9s  ", Bytes(p.size(r.item)))
			}
			if expandable {
				line += arrow
			}
			line += fmt.Sprintf("%s %s", Pad(it.Label, width), Dim(it.Detail))
		}
		if v == p.cursor {
			line = "❯ " + Bold(line)
		} else {
			line = "  " + line
		}
		s.WriteString(" " + line + "\n")
	}
	return s.String()
}

func (p *Picker) menuView() string {
	var s strings.Builder
	if p.done { // what stays on screen once the menu closes
		answer := Dim("–")
		if !p.Cancelled && len(p.visible) > 0 {
			answer = Green(p.Items[p.visible[p.cursor].item].Label)
		}
		fmt.Fprintf(&s, " %s %s\n", Bold(p.Title), answer)
		return s.String()
	}
	fmt.Fprintf(&s, " %s\n %s\n\n", Bold(p.Title), Dim(i18n.T("menu.help")))
	end := min(len(p.visible), p.top+p.height)
	for v := p.top; v < end; v++ {
		i := p.visible[v].item
		it := p.Items[i]
		if it.Group != "" && (v == p.top || it.Group != p.Items[p.visible[v-1].item].Group) {
			fmt.Fprintf(&s, " %s\n", Dim(it.Group))
		}
		line := fmt.Sprintf("%s %s", Pad(it.Label, 18), Dim(it.Detail))
		if v == p.cursor {
			line = Green("❯ ") + Bold(line)
		} else {
			line = "  " + line
		}
		s.WriteString(" " + line + "\n")
	}
	return s.String()
}

// Choose shows a menu and returns the chosen index, or -1 if it was cancelled
// or could not run.
func Choose(title string, items []PickItem) int {
	p := NewPicker(title, items)
	p.Single = true
	if _, err := tea.NewProgram(p).Run(); err != nil || p.Cancelled {
		return -1
	}
	for i, v := range p.Selected {
		if v {
			return i
		}
	}
	return -1
}
