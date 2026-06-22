package app

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/superposition/kotoba-line/internal/content"
	"github.com/superposition/kotoba-line/internal/game"
	statestore "github.com/superposition/kotoba-line/internal/state"
	"github.com/superposition/kotoba-line/internal/tui/atoms"
)

func TestViewShowsInitialScreen(t *testing.T) {
	view := New(Options{Username: "player", Library: testLibrary(), DisableEventLog: true}).View()
	plain := atoms.StripANSI(view)

	for _, want := range []string{"Kotoba Line", "Player: player", "Station: Tide Gate", "DRILL", "日"} {
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
	if !strings.Contains(atoms.StripANSI(model.View()), "hint: 日 = ひ (hi)") {
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

func TestDrillViewShowsTargetAndQueueWithoutRows(t *testing.T) {
	model := New(Options{Library: testLibrary(), DisableEventLog: true})
	model.drill = model.drill.Tick()
	model.drill = model.drill.Tick()
	model.drill = model.drill.Tick()
	model.drill, _ = model.drill.Spawn()

	view := atoms.StripANSI(model.View())
	for _, want := range []string{"target  日", "note    sun; day", "queue", "日本  Japan"} {
		if !strings.Contains(view, want) {
			t.Fatalf("drill view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "row ") {
		t.Fatalf("drill view should not expose internal row counters:\n%s", view)
	}
}

func TestFullWidthQuestionMarkRevealsHint(t *testing.T) {
	model := New(Options{Library: testLibrary(), DisableEventLog: true})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'？'}})
	if cmd != nil {
		t.Fatalf("Update(？) returned command, want nil")
	}
	model = updated.(Model)
	if !strings.Contains(atoms.StripANSI(model.View()), "hint: 日 = ひ (hi)") {
		t.Fatalf("full-width hint not shown in view:\n%s", model.View())
	}
}

func TestSwitchesToConstitutionLevelFromLoadedContent(t *testing.T) {
	model := New(Options{DisableEventLog: true})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd != nil {
		t.Fatalf("Update(c) returned command, want nil")
	}
	model = updated.(Model)

	view := atoms.StripANSI(model.View())
	for _, want := range []string{
		"Station: Constitution Gate: Preamble 1",
		"STATION ARRIVAL",
		"target  日本国民は",
		"note    the Japanese people",
		"LEVEL Constitution Gate: Preamble 1",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("constitution view missing %q:\n%s", want, view)
		}
	}
}

func TestStationSelectorShowsOpenAndLockedLevels(t *testing.T) {
	model := New(Options{DisableEventLog: true})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd != nil {
		t.Fatalf("Update(s) returned command, want nil")
	}
	model = updated.(Model)

	view := atoms.StripANSI(model.View())
	for _, want := range []string{
		"STATIONS",
		"Tide Gate",
		"Constitution Gate",
		"Emperor Symbol",
		"LOCKED",
		"needs:",
		"日本国民は/にほんこくみんは",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("station selector missing %q:\n%s", want, view)
		}
	}
}

func TestStationSelectorSwitchesSelectedOpenLevel(t *testing.T) {
	model := New(Options{DisableEventLog: true}).openStations()

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jjjj")})
	if cmd != nil {
		t.Fatalf("Update(jjjj) returned command, want nil")
	}
	model = updated.(Model)

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("Update(enter) returned command, want nil")
	}
	model = updated.(Model)

	view := atoms.StripANSI(model.View())
	for _, want := range []string{
		"Station: Constitution Gate: Preamble 1",
		"target  日本国民は",
		"LEVEL Constitution Gate: Preamble 1",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("selected level view missing %q:\n%s", want, view)
		}
	}
}

func TestStationSelectorKeepsLockedLevelVisible(t *testing.T) {
	model := New(Options{DisableEventLog: true}).openStations()
	model.cursor = indexLevelOption(model.levelOptions(), "constitution-article-1")

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("Update(enter) returned command, want nil")
	}
	model = updated.(Model)

	view := atoms.StripANSI(model.View())
	for _, want := range []string{
		"STATIONS",
		"LOCKED Article 1: Symbol Of The State",
		"needs:",
		"日本国民は/にほんこくみんは",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("locked level view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Station: Article 1: Symbol Of The State") {
		t.Fatalf("locked level should not switch drill station:\n%s", view)
	}
}

func TestStationSelectorSwitchesUnlockedLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	log := statestore.NewEventLog(path)
	for _, cardID := range []string{
		"constitution-preamble-nihon-kokumin-wa",
		"constitution-preamble-shuken",
		"constitution-preamble-kenpou",
	} {
		if err := log.Append(statestore.CardMastered(cardID)); err != nil {
			t.Fatalf("append mastery event: %v", err)
		}
	}

	model := New(Options{EventLogPath: path}).openStations()
	model.cursor = indexLevelOption(model.levelOptions(), "constitution-article-1")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	view := atoms.StripANSI(model.View())
	for _, want := range []string{
		"Station: Article 1: Symbol Of The State",
		"target  第一条",
		"LEVEL Article 1: Symbol Of The State",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("unlocked level view missing %q:\n%s", want, view)
		}
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
	if events[1].Clean == nil || *events[1].Clean {
		t.Fatalf("hinted drill hit should be unclean: %#v", events[1])
	}
}

func TestCleanPrerequisiteHitsUnlockGatedArticleLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	model := New(Options{EventLogPath: path}).switchLevel(constitutionLevelID)

	for _, cardID := range articleOnePrerequisiteCardIDs() {
		model = masterCardClean(t, model, cardID)
	}

	events, err := statestore.NewEventLog(path).ReadAll()
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	unlockCount := 0
	for _, event := range events {
		if event.Type == statestore.EventLevelUnlocked && event.LevelID == "constitution-article-1" {
			unlockCount++
		}
	}
	if unlockCount != 1 {
		t.Fatalf("level unlock count = %d, want 1: %#v", unlockCount, events)
	}

	progress, err := statestore.NewEventLog(path).Replay()
	if err != nil {
		t.Fatalf("replay event log: %v", err)
	}
	if !progress.UnlockedLevels["constitution-article-1"] {
		t.Fatalf("article level was not durably unlocked: %#v", progress.UnlockedLevels)
	}

	model = New(Options{EventLogPath: path}).openStations()
	option, ok := model.levelOption("constitution-article-1")
	if !ok {
		t.Fatalf("article level option missing")
	}
	if option.Locked {
		t.Fatalf("article level should be open after prerequisite mastery: %#v", option.Missing)
	}
	if !strings.Contains(atoms.StripANSI(model.View()), "OPEN   Emperor Symbol") {
		t.Fatalf("station selector should show Emperor Symbol open:\n%s", model.View())
	}
}

func TestHintedPrerequisiteHitsDoNotUnlockGatedArticleLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	model := New(Options{EventLogPath: path}).switchLevel(constitutionLevelID)

	for _, cardID := range articleOnePrerequisiteCardIDs() {
		model = hitCardWithHint(t, model, cardID)
		model = hitCardWithHint(t, model, cardID)
		model = hitCardWithHint(t, model, cardID)
	}

	progress, err := statestore.NewEventLog(path).Replay()
	if err != nil {
		t.Fatalf("replay event log: %v", err)
	}
	if progress.UnlockedLevels["constitution-article-1"] {
		t.Fatalf("article level should not unlock from hinted hits: %#v", progress.UnlockedLevels)
	}
	for _, cardID := range articleOnePrerequisiteCardIDs() {
		card := progress.Cards[cardID]
		if card.Mastered {
			t.Fatalf("card %s should not be mastered from hinted hits: %#v", cardID, card)
		}
	}

	events, err := statestore.NewEventLog(path).ReadAll()
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	for _, event := range events {
		if event.Type == statestore.EventLevelUnlocked && event.LevelID == "constitution-article-1" {
			t.Fatalf("unexpected article unlock event: %#v", events)
		}
	}
}

func TestBossModeDamagesBossAndRendersTransition(t *testing.T) {
	model := New(Options{Library: testLibrary(), DisableEventLog: true})
	startHP := model.boss.HP()

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if cmd != nil {
		t.Fatalf("Update(b) returned command, want nil")
	}
	model = updated.(Model)
	view := atoms.StripANSI(model.View())
	for _, want := range []string{"BOSS", "BOSS INTRO", "target 日が暮れる"} {
		if !strings.Contains(view, want) {
			t.Fatalf("boss view missing %q:\n%s", want, view)
		}
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ひがくれる")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if model.boss.HP() != startHP-1 {
		t.Fatalf("boss HP = %d, want %d", model.boss.HP(), startHP-1)
	}
	view = atoms.StripANSI(model.View())
	for _, want := range []string{"BOSS CRACK", "BOSS HIT 日が暮れる -1 HP"} {
		if !strings.Contains(view, want) {
			t.Fatalf("boss hit view missing %q:\n%s", want, view)
		}
	}
}

func TestBossModeAppendsReplayableStateEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	model := New(Options{Library: testLibrary(), EventLogPath: path})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model = updated.(Model)
	for !model.boss.Cleared() {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ひがくれる")})
		model = updated.(Model)
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model = updated.(Model)
	}

	events, err := statestore.NewEventLog(path).ReadAll()
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	if len(events) != 9 {
		t.Fatalf("event count = %d, want 9: %#v", len(events), events)
	}
	if events[0].Type != statestore.EventBossIntro || events[0].BossID != "journal-2026-06-22-boss" {
		t.Fatalf("first event = %#v, want boss intro", events[0])
	}
	if events[1].Type != statestore.EventHintRevealed || events[1].CardID != "phrase-hi-ga-kureru" {
		t.Fatalf("second event = %#v, want boss hint", events[1])
	}
	if events[2].Type != statestore.EventEnemyHit || events[2].Clean == nil || *events[2].Clean {
		t.Fatalf("first boss hit should be unclean after hint: %#v", events[2])
	}
	if events[len(events)-1].Type != statestore.EventBossCleared {
		t.Fatalf("last event = %#v, want boss cleared", events[len(events)-1])
	}
	if !strings.Contains(atoms.StripANSI(model.View()), "LEVEL CLEAR") {
		t.Fatalf("clear view missing LEVEL CLEAR:\n%s", model.View())
	}
}

func articleOnePrerequisiteCardIDs() []string {
	return []string{
		"constitution-preamble-nihon-kokumin-wa",
		"constitution-preamble-shuken",
		"constitution-preamble-kenpou",
	}
}

func masterCardClean(t *testing.T, model Model, cardID string) Model {
	t.Helper()
	card := mustLibraryCard(t, model.library, cardID)
	model.drill = game.NewDrillFromCards([]content.Card{card}, game.Config{SpawnEvery: 6, MaxEnemies: 1}).Start()
	for i := 0; i < statestore.MasteryCleanHitStreak; i++ {
		model = submitKana(t, model, card.Reading.Kana)
	}
	return model
}

func hitCardWithHint(t *testing.T, model Model, cardID string) Model {
	t.Helper()
	card := mustLibraryCard(t, model.library, cardID)
	model.drill = game.NewDrillFromCards([]content.Card{card}, game.Config{SpawnEvery: 6, MaxEnemies: 1}).Start()

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if cmd != nil {
		t.Fatalf("Update(?) returned command, want nil")
	}
	model = updated.(Model)
	return submitKana(t, model, card.Reading.Kana)
}

func submitKana(t *testing.T, model Model, kana string) Model {
	t.Helper()
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(kana)})
	if cmd != nil {
		t.Fatalf("Update(kana) returned command, want nil")
	}
	model = updated.(Model)
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("Update(enter) returned command, want nil")
	}
	return updated.(Model)
}

func mustLibraryCard(t *testing.T, library *content.Library, cardID string) content.Card {
	t.Helper()
	if library == nil {
		t.Fatalf("library is nil")
	}
	for _, card := range library.Cards {
		if card.ID == cardID {
			return card
		}
	}
	t.Fatalf("card %q missing", cardID)
	return content.Card{}
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
			{
				ID:       "phrase-hi-ga-kureru",
				Text:     "日が暮れる",
				Reading:  content.Reading{Kana: "ひがくれる", RomajiHint: "hi ga kureru"},
				Meaning:  "the sun sets",
				Type:     content.CardTypePhrase,
				Playable: true,
			},
		},
	}
}
