package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestPickerFilterAllNone(t *testing.T) {
	p := NewPicker("Pick", []PickItem{
		{Group: "Apps", Label: "Google Chrome"}, {Group: "Apps", Label: "Firefox"},
		{Group: "Brew", Label: "node", Detail: "chrome-devtools"}, {Group: "Brew", Label: "go"},
	})
	// "a" ticks everything, "n" clears everything
	p.Update(key("a"))
	for i, v := range p.Selected {
		if !v {
			t.Fatalf("row %d not ticked by a", i)
		}
	}
	p.Update(key("n"))
	// "/chrome" keeps the two rows that mention chrome (label or detail)
	p.Update(key("/"))
	for _, r := range "chrome" {
		p.Update(key(string(r)))
	}
	if len(p.visible) != 2 || p.visible[0] != 0 || p.visible[1] != 2 {
		t.Fatalf("visible = %v", p.visible)
	}
	if v := p.View(); !strings.Contains(v, "/ chrome_") || strings.Contains(v, "Firefox") {
		t.Fatalf("filtered view:\n%s", v)
	}
	// space ticks the original row under the cursor; "a" ticks visible rows only
	p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p.Update(tea.KeyMsg{Type: tea.KeySpace}) // typing: space is part of the filter
	if p.filter != "chrome " {
		t.Fatalf("filter = %q", p.filter)
	}
	p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	p.Update(tea.KeyMsg{Type: tea.KeyEnter}) // keep the filter, stop typing
	if p.typing || p.done {
		t.Fatal("enter while typing must only close the filter")
	}
	p.Update(tea.KeyMsg{Type: tea.KeyDown}) // "chrome " had narrowed the list to one row and reset the cursor
	p.Update(key(" "))
	if !p.Selected[2] || p.Selected[0] || p.Selected[1] {
		t.Fatalf("selected after space = %v", p.Selected)
	}
	p.Update(key("a"))
	if p.Selected[1] || p.Selected[3] || !p.Selected[0] {
		t.Fatalf("a with a filter ticked hidden rows: %v", p.Selected)
	}
	// esc while typing clears the filter instead of cancelling
	p.Update(key("/"))
	p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.Cancelled || p.filter != "" || len(p.visible) != 4 {
		t.Fatalf("esc: cancelled=%v filter=%q visible=%v", p.Cancelled, p.filter, p.visible)
	}
	p.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if p.cursor != 3 {
		t.Fatalf("end: cursor = %d", p.cursor)
	}
	p.Update(tea.KeyMsg{Type: tea.KeyHome})
	p.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if p.cursor != 3 {
		t.Fatalf("pgdown past the end: cursor = %d", p.cursor)
	}
}
