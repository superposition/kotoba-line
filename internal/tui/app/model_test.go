package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestViewShowsInitialScreen(t *testing.T) {
	view := New(Options{Username: "player"}).View()

	for _, want := range []string{"Kotoba Line", "Player: player", "Station 01"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q:\n%s", want, view)
		}
	}
}

func TestUpdateStoresWindowSize(t *testing.T) {
	model, cmd := New(Options{}).Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	if cmd != nil {
		t.Fatalf("Update(WindowSizeMsg) returned command, want nil")
	}

	got := model.(Model)
	if got.width != 100 || got.height != 32 {
		t.Fatalf("window size = %dx%d, want 100x32", got.width, got.height)
	}
	if !strings.Contains(got.View(), "Terminal: 100x32") {
		t.Fatalf("View() did not include terminal size:\n%s", got.View())
	}
}

func TestUpdateQuitKeys(t *testing.T) {
	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyCtrlC},
	} {
		_, cmd := New(Options{}).Update(msg)
		if cmd == nil {
			t.Fatalf("Update(%v) returned nil command, want quit command", msg)
		}
	}
}
