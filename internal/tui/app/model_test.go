package app

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/superposition/kotoba-line/internal/content"
	statestore "github.com/superposition/kotoba-line/internal/state"
	"github.com/superposition/kotoba-line/internal/tui/atoms"
)

func TestViewShowsInitialScreen(t *testing.T) {
	view := New(Options{Username: "player", Library: testLibrary(), DisableEventLog: true}).View()
	plain := atoms.StripANSI(view)

	for _, want := range []string{"Kotoba Line", "Player: player", "Station 01", "DRILL", "日"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("View() missing %q:\n%s", want, view)
		}
	}
}

func TestUpdateStoresWindowSize(t *testing.T) {
	model, cmd := New(Options{Library: testLibrary(), DisableEventLog: true}).Update(tea.WindowSizeMsg{Width: 100, Height: 32})
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

func TestViewRendersAtCommonTerminalSizes(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{
		{Width: 80, Height: 24},
		{Width: 120, Height: 40},
	} {
		model, _ := New(Options{Library: testLibrary(), DisableEventLog: true}).Update(size)
		view := model.(Model).View()

		for i, line := range strings.Split(view, "\n") {
			if got := atoms.DisplayWidth(line); got > size.Width {
				t.Fatalf("%dx%d line %d width = %d, want <= %d: %q", size.Width, size.Height, i+1, got, size.Width, atoms.StripANSI(line))
			}
		}
	}
}

func TestUpdateQuitKeys(t *testing.T) {
	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyCtrlC},
	} {
		_, cmd := New(Options{Library: testLibrary(), DisableEventLog: true}).Update(msg)
		if cmd == nil {
			t.Fatalf("Update(%v) returned nil command, want quit command", msg)
		}
	}
}

func TestKanaInputHitAndHintActions(t *testing.T) {
	model := New(Options{Library: testLibrary(), DisableEventLog: true})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if cmd != nil {
		t.Fatalf("Update(?) returned command, want nil")
	}
	model = updated.(Model)
	if model.drill.Hints() != 1 || model.drill.Hits() != 0 || model.drill.Misses() != 0 {
		t.Fatalf("hint counts = hits %d misses %d hints %d, want 0/0/1", model.drill.Hits(), model.drill.Misses(), model.drill.Hints())
	}
	if !strings.Contains(atoms.StripANSI(model.View()), "hint: 日 = hi") {
		t.Fatalf("hint not shown in view:\n%s", model.View())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'ひ'}})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if model.drill.Hits() != 1 || model.drill.Misses() != 0 {
		t.Fatalf("submit counts = hits %d misses %d, want 1/0", model.drill.Hits(), model.drill.Misses())
	}
	if !strings.Contains(atoms.StripANSI(model.View()), "HIT 日 -> ひ") {
		t.Fatalf("hit not shown in view:\n%s", model.View())
	}
}

func TestKanaActionsAppendStateEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	model := New(Options{Library: testLibrary(), EventLogPath: path})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'ひ'}})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	events, err := statestore.NewEventLog(path).ReadAll()
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("event count = %d, want 2: %#v", len(events), events)
	}
	if events[0].Type != statestore.EventHintRevealed || events[0].CardID != "hi" {
		t.Fatalf("first event = %#v, want hint for hi", events[0])
	}
	if events[1].Type != statestore.EventEnemyHit || events[1].CardID != "hi" {
		t.Fatalf("second event = %#v, want hit for hi", events[1])
	}
}

func testLibrary() *content.Library {
	return &content.Library{
		Cards: []content.Card{
			{
				ID:       "hi",
				Text:     "日",
				Reading:  content.Reading{Kana: "ひ", RomajiHint: "hi"},
				Meaning:  "sun; day",
				Type:     content.CardTypeKanjiReading,
				Playable: true,
			},
			{
				ID:       "nihon",
				Text:     "日本",
				Reading:  content.Reading{Kana: "にほん", RomajiHint: "nihon"},
				Meaning:  "Japan",
				Type:     content.CardTypeWord,
				Playable: true,
			},
		},
	}
}
