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
type Picker struct {
	Title     string
	Items     []PickItem
	Selected  []bool
	Cancelled bool
	Single    bool
	cursor    int
	top       int
	height    int
	done      bool
}

// NewPicker builds a picker with nothing selected.
func NewPicker(title string, items []PickItem) *Picker {
	return &Picker{Title: title, Items: items, Selected: make([]bool, len(items)), height: 20}
}

func (p *Picker) Init() tea.Cmd { return nil }

func (p *Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.height = max(3, m.Height-6)
	case tea.KeyMsg:
		switch m.String() {
		case "q", "esc", "ctrl+c":
			p.Cancelled, p.done = true, true
			return p, tea.Quit
		case "enter":
			if p.Single && len(p.Items) > 0 {
				p.Selected[p.cursor] = true
			}
			p.done = true
			return p, tea.Quit
		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
			}
		case "down", "j":
			if p.cursor < len(p.Items)-1 {
				p.cursor++
			}
		case " ", "x":
			if len(p.Items) > 0 && !p.Single {
				p.Selected[p.cursor] = !p.Selected[p.cursor]
			}
		}
	}
	if p.cursor < p.top {
		p.top = p.cursor
	}
	if p.cursor >= p.top+p.height {
		p.top = p.cursor - p.height + 1
	}
	return p, nil
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
	fmt.Fprintf(&s, " %s  %s\n %s\n\n", Bold(p.Title), Dim(fmt.Sprintf(i18n.T("pick.count"), n)), Dim(i18n.T("pick.help")))
	width := 0
	for _, it := range p.Items {
		width = max(width, utf8.RuneCountInString(it.Label))
	}
	width = min(width, 56)
	end := min(len(p.Items), p.top+p.height)
	for i := p.top; i < end; i++ {
		it := p.Items[i]
		if i == 0 || it.Group != p.Items[i-1].Group || i == p.top {
			fmt.Fprintf(&s, " %s\n", Green(it.Group))
		}
		box := "[ ]"
		if p.Selected[i] {
			box = Green("[x]")
		}
		line := fmt.Sprintf("%s %s %s", box, Pad(it.Label, width), Dim(it.Detail))
		if i == p.cursor {
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
		if !p.Cancelled && len(p.Items) > 0 {
			answer = Green(p.Items[p.cursor].Label)
		}
		fmt.Fprintf(&s, " %s %s\n", Bold(p.Title), answer)
		return s.String()
	}
	fmt.Fprintf(&s, " %s\n %s\n\n", Bold(p.Title), Dim(i18n.T("menu.help")))
	end := min(len(p.Items), p.top+p.height)
	for i := p.top; i < end; i++ {
		it := p.Items[i]
		if it.Group != "" && (i == 0 || it.Group != p.Items[i-1].Group || i == p.top) {
			fmt.Fprintf(&s, " %s\n", Dim(it.Group))
		}
		line := fmt.Sprintf("%s %s", Pad(it.Label, 18), Dim(it.Detail))
		if i == p.cursor {
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
