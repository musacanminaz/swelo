package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/musacanminaz/swelo/internal/i18n"
)

// PickItem is one row of a multi-select list.
type PickItem struct {
	Group, Label, Detail string
}

// Picker is a bubbletea multi-select list. After Run, Selected says what was
// ticked; Cancelled is true if the user quit without confirming. With Single,
// Enter picks the row under the cursor (a menu).
//
// Keys: ↑↓/jk move, PgUp/PgDn/Home/End jump, space ticks, a ticks every
// visible row, n unticks them, / filters rows as you type (Enter keeps the
// filter, Esc clears it), Enter confirms, q cancels.
type Picker struct {
	Title     string
	Items     []PickItem
	Selected  []bool
	Cancelled bool
	Single    bool
	cursor    int   // index into visible
	visible   []int // rows that match the filter, as indices into Items
	filter    string
	typing    bool // keys edit the filter
	top       int
	height    int
	done      bool
}

// NewPicker builds a picker with nothing selected.
func NewPicker(title string, items []PickItem) *Picker {
	p := &Picker{Title: title, Items: items, Selected: make([]bool, len(items)), height: 20}
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
				p.Selected[p.visible[p.cursor]] = true
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
		case " ", "x":
			if len(p.visible) > 0 && !p.Single {
				i := p.visible[p.cursor]
				p.Selected[i] = !p.Selected[i]
			}
		case "a", "n":
			if !p.Single {
				for _, i := range p.visible {
					p.Selected[i] = m.String() == "a"
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

// refilter recomputes the visible rows and keeps the cursor on a valid one.
func (p *Picker) refilter() {
	p.visible = p.visible[:0]
	f := strings.ToLower(p.filter)
	for i, it := range p.Items {
		if f == "" || strings.Contains(strings.ToLower(it.Label+" "+it.Detail+" "+it.Group), f) {
			p.visible = append(p.visible, i)
		}
	}
	p.cursor = max(0, min(p.cursor, len(p.visible)-1))
	p.top = 0
}

func (p *Picker) View() string {
	var s strings.Builder
	if p.Single {
		return p.menuView()
	}
	n := 0
	for _, v := range p.Selected {
		if v {
			n++
		}
	}
	help := i18n.T("pick.help")
	if p.typing {
		help = i18n.T("pick.help.filter")
	}
	fmt.Fprintf(&s, " %s  %s\n %s\n", Bold(p.Title), Dim(fmt.Sprintf(i18n.T("pick.count"), n)), Dim(help))
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
	for _, i := range p.visible {
		width = max(width, utf8.RuneCountInString(p.Items[i].Label))
	}
	width = min(width, 56)
	end := min(len(p.visible), p.top+p.height)
	for v := p.top; v < end; v++ {
		i := p.visible[v]
		it := p.Items[i]
		if v == p.top || it.Group != p.Items[p.visible[v-1]].Group {
			fmt.Fprintf(&s, " %s\n", Green(it.Group))
		}
		box := "[ ]"
		if p.Selected[i] {
			box = Green("[x]")
		}
		line := fmt.Sprintf("%s %s %s", box, Pad(it.Label, width), Dim(it.Detail))
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
			answer = Green(p.Items[p.visible[p.cursor]].Label)
		}
		fmt.Fprintf(&s, " %s %s\n", Bold(p.Title), answer)
		return s.String()
	}
	fmt.Fprintf(&s, " %s\n %s\n\n", Bold(p.Title), Dim(i18n.T("menu.help")))
	end := min(len(p.visible), p.top+p.height)
	for v := p.top; v < end; v++ {
		i := p.visible[v]
		it := p.Items[i]
		if it.Group != "" && (v == p.top || it.Group != p.Items[p.visible[v-1]].Group) {
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
