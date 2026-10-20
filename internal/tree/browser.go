package tree

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/ui"
)

// Browser is the interactive tree view (bubbletea model).
type Browser struct {
	Root      *Node
	Home      string
	Protected []string
	TrashFn   func(path string) error // nil = Trash(path, Home)

	cur     *Node
	cursor  int
	top     int
	height  int
	confirm *Node
	msg     string
	Freed   int64
}

// NewBrowser starts at root.
func NewBrowser(root *Node, home string, protected []string) *Browser {
	return &Browser{Root: root, Home: home, Protected: protected, cur: root, height: 20}
}

func (b *Browser) Init() tea.Cmd { return nil }

func (b *Browser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		b.height = m.Height - 7
		if b.height < 3 {
			b.height = 3
		}
	case tea.KeyMsg:
		key := m.String()
		if b.confirm != nil {
			if key == "y" || key == "e" {
				b.trash(b.confirm)
			} else {
				b.msg = i18n.T("tree.cancelled")
			}
			b.confirm = nil
			return b, nil
		}
		b.msg = ""
		n := len(b.cur.Children)
		switch key {
		case "q", "ctrl+c", "esc":
			return b, tea.Quit
		case "up", "k":
			if b.cursor > 0 {
				b.cursor--
			}
		case "down", "j":
			if b.cursor < n-1 {
				b.cursor++
			}
		case "enter", "right", "l":
			if n > 0 && b.cur.Children[b.cursor].Dir && !b.cur.Children[b.cursor].Denied {
				b.cur, b.cursor, b.top = b.cur.Children[b.cursor], 0, 0
			}
		case "left", "h", "backspace":
			if b.cur.Parent != nil {
				prev := b.cur
				b.cur, b.top = b.cur.Parent, 0
				b.cursor = indexOf(b.cur.Children, prev)
			}
		case "d", "delete":
			if n == 0 {
				break
			}
			sel := b.cur.Children[b.cursor]
			if why := CanTrash(sel.Path, b.Root.Path, b.Home, b.Protected); why != "" {
				b.msg = ui.Yellow(fmt.Sprintf(i18n.T("tree.refused"), why))
				break
			}
			b.confirm = sel
		}
	}
	if b.cursor < b.top {
		b.top = b.cursor
	}
	if b.cursor >= b.top+b.height {
		b.top = b.cursor - b.height + 1
	}
	return b, nil
}

func (b *Browser) trash(n *Node) {
	fn := b.TrashFn
	if fn == nil {
		fn = func(p string) error { return Trash(p, b.Home) }
	}
	if err := fn(n.Path); err != nil {
		b.msg = ui.Red(err.Error())
		return
	}
	n.Detach()
	b.Freed += n.Size
	b.msg = ui.Green(fmt.Sprintf(i18n.T("tree.trashed"), n.Name, ui.Bytes(n.Size)))
	if b.cursor >= len(b.cur.Children) && b.cursor > 0 {
		b.cursor--
	}
}

func (b *Browser) View() string {
	var s strings.Builder
	fmt.Fprintf(&s, " %s  %s\n", ui.Bold(short(b.cur.Path, b.Home)), ui.Bold(ui.Bytes(b.cur.Size)))
	fmt.Fprintf(&s, " %s\n\n", ui.Dim(i18n.T("tree.help")))
	end := b.top + b.height
	if end > len(b.cur.Children) {
		end = len(b.cur.Children)
	}
	for i := b.top; i < end; i++ {
		c := b.cur.Children[i]
		name := c.Name
		if c.Dir {
			name += "/"
		}
		if c.Denied {
			name += " " + ui.Dim(i18n.T("tree.denied"))
		}
		line := fmt.Sprintf("%9s %s %s", ui.Bytes(c.Size), bar(c.Size, b.cur.Size), name)
		if i == b.cursor {
			line = "❯ " + ui.Bold(line)
		} else {
			line = "  " + line
		}
		s.WriteString(line + "\n")
	}
	if b.cur.Small > 0 {
		fmt.Fprintf(&s, "  %9s %s %s\n", ui.Bytes(b.cur.SmallSz), bar(b.cur.SmallSz, b.cur.Size),
			ui.Dim(fmt.Sprintf(i18n.T("tree.small"), b.cur.Small)))
	}
	s.WriteString("\n")
	if b.confirm != nil {
		fmt.Fprintf(&s, " %s\n", ui.Yellow(fmt.Sprintf(i18n.T("tree.confirm"), short(b.confirm.Path, b.Home), ui.Bytes(b.confirm.Size))))
	} else if b.msg != "" {
		fmt.Fprintf(&s, " %s\n", b.msg)
	}
	return s.String()
}

func bar(n, total int64) string {
	const width = 12
	k := 0
	if total > 0 {
		k = int(float64(n) / float64(total) * width)
	}
	return ui.Green(strings.Repeat("█", k)) + ui.Dim(strings.Repeat("░", width-k))
}

func indexOf(list []*Node, n *Node) int {
	for i, c := range list {
		if c == n {
			return i
		}
	}
	return 0
}

func short(p, home string) string {
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}
