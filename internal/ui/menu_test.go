package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSingleEnterPicksCursorRow(t *testing.T) {
	p := NewPicker("Menu", []PickItem{{Label: "Clean"}, {Label: "Analyze"}, {Label: "Quit"}})
	p.Single = true
	p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p.Update(tea.KeyMsg{Type: tea.KeySpace}) // space does not tick in a menu
	p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !p.Selected[1] || p.Selected[0] || p.Selected[2] || p.Cancelled {
		t.Fatalf("selected %v cancelled %v", p.Selected, p.Cancelled)
	}
	if v := p.View(); !strings.Contains(v, "Analyze") || strings.Contains(v, "Clean") {
		t.Fatalf("closed menu should show only the answer: %q", v)
	}
}
