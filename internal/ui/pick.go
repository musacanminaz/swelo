package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/musacanminaz/swelo/internal/i18n"
)

// PickItem is one row of a multi-select list.
type PickItem struct {
	Group, Label, Detail string
}

// Picker is a bubbletea multi-select list. After Run, Selected says what was
// ticked; Cancelled is true if the user quit without confirming.
type Picker struct {
	Title     string
	Items     []PickItem
	Selected  []bool
	Cancelled bool
	cursor    int
	top       int
	height    int
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
			p.Cancelled = true
			return p, tea.Quit
		case "enter":
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
			if len(p.Items) > 0 {
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
	n := 0
	for _, v := range p.Selected {
		if v {
			n++
		}
	}
	fmt.Fprintf(&s, " %s  %s\n %s\n\n", Bold(p.Title), Dim(fmt.Sprintf(i18n.T("pick.count"), n)), Dim(i18n.T("pick.help")))
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
		line := fmt.Sprintf("%s %s %s", box, Pad(it.Label, 56), Dim(it.Detail))
		if i == p.cursor {
			line = "❯ " + Bold(line)
		} else {
			line = "  " + line
		}
		s.WriteString(" " + line + "\n")
	}
	return s.String()
}
