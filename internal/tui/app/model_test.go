package app

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/superposition/kotoba-line/internal/content"
	"github.com/superposition/kotoba-line/internal/game"
	kanainput "github.com/superposition/kotoba-line/internal/kana"
	statestore "github.com/superposition/kotoba-line/internal/state"
	"github.com/superposition/kotoba-line/internal/tui/atoms"
)

func TestViewShowsInitialScreen(t *testing.T) {
	view := New(Options{Username: "player", Library: testLibrary(), DisableEventLog: true}).View()
	plain := atoms.StripANSI(view)

	if !strings.Contains(view, "\x1b[") {
		t.Fatalf("View() should include seaside ANSI color styling:\n%s", view)
	}
	for _, want := range []string{"KOTOBA BEACH", "goal    catch the kana wave", "target", "meaning", "sound   [", "[ keys _", "hint    ? for kana/romaji", "feedback ready to surf"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("View() missing %q:\n%s", want, view)
		}
	}
	for _, bad := range []string{"points", "tide", "streak", "STREAK", "before it hits shore", "SHELVES", "DOCUMENT", "TREE", "SQLite Lesson", "SKILL TREE", "DAG", "flowchart TD", "-->", ".->", "[\"", "study map", "exercise:", "Q: type kana reading", "queued:", "next key", "press   ["} {
		if strings.Contains(plain, bad) {
			t.Fatalf("View() should not render old card/sidebar marker %q:\n%s", bad, view)
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
	if !strings.Contains(got.View(), "KOTOBA BEACH") {
		t.Fatalf("View() did not include HUD screen:\n%s", got.View())
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

func TestViewFitsStandardSSHViewport(t *testing.T) {
	model, _ := New(Options{Library: testLibrary(), DisableEventLog: true}).Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	plain := atoms.StripANSI(model.(Model).View())

	for _, want := range []string{"KOTOBA BEACH", "goal    catch the kana wave", "target", "meaning", "[ keys _", "feedback"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("80x24 view missing %q:\n%s", want, plain)
		}
	}
	for _, bad := range []string{"SHELVES", "DOCUMENT", "TREE", "SQLite Lesson"} {
		if strings.Contains(plain, bad) {
			t.Fatalf("80x24 gameplay view should not show %q:\n%s", bad, plain)
		}
	}
	if lines := strings.Split(plain, "\n"); len(lines) > 24 {
		t.Fatalf("80x24 view has %d lines, want <= 24:\n%s", len(lines), plain)
	}
}

func TestPromptKeepsTargetSoundAndKeysTogether(t *testing.T) {
	view := atoms.StripANSI(New(Options{Username: "player", Library: testLibrary(), DisableEventLog: true}).View())
	targetIndex := strings.Index(view, "target")
	soundIndex := strings.Index(view, "sound")
	keysIndex := strings.Index(view, "[ keys")
	meaningIndex := strings.Index(view, "meaning")
	if targetIndex < 0 || soundIndex < 0 || keysIndex < 0 || meaningIndex < 0 {
		t.Fatalf("prompt missing core lines:\n%s", view)
	}
	if !(targetIndex < soundIndex && soundIndex < keysIndex && keysIndex < meaningIndex) {
		t.Fatalf("target/sound/keys should be clustered before meaning:\n%s", view)
	}

	betweenTargetAndKeys := view[targetIndex:keysIndex]
	if strings.Contains(betweenTargetAndKeys, "meaning") || strings.Count(betweenTargetAndKeys, "\n") > 3 {
		t.Fatalf("target/sound/keys cluster has too much separation:\n%s", view)
	}
}

func TestKatakanaTargetProgressUsesKatakanaPreview(t *testing.T) {
	if got := targetSoundProgress("カ", "ka"); got != "カ" {
		t.Fatalf("targetSoundProgress(katakana ka) = %q, want カ", got)
	}
	if got := targetProgressText("k + a / か", "カ", "ka"); got != "[カ]" {
		t.Fatalf("targetProgressText(katakana ka) = %q, want [カ]", got)
	}
	if got := targetSoundProgress("カ", "か"); got != "" {
		t.Fatalf("wrong-script exact input should not preview progress, got %q", got)
	}
}

func TestNewReplaysSavedProgressOnStartup(t *testing.T) {
	store := statestore.NewSQLiteEventStore(filepath.Join(t.TempDir(), "kotoba.sqlite"), "player")
	if err := store.Append(statestore.CardMastered("lesson-one-a")); err != nil {
		t.Fatalf("append saved mastery: %v", err)
	}

	model := New(Options{Username: "player", Library: resumeTestLibrary(), EventStore: store})
	if model.levelID != "lesson-one" {
		t.Fatalf("levelID = %q, want lesson-one", model.levelID)
	}
	if view := atoms.StripANSI(model.View()); !strings.Contains(view, "target") || strings.Contains(view, "set     ") {
		t.Fatalf("startup view did not replay saved progress:\n%s", view)
	}
}

func TestNewTargetsUnlearnedCardsAfterReplay(t *testing.T) {
	store := statestore.NewSQLiteEventStore(filepath.Join(t.TempDir(), "kotoba.sqlite"), "player")
	if err := store.Append(statestore.CardMastered("lesson-one-a")); err != nil {
		t.Fatalf("append saved mastery: %v", err)
	}

	model := New(Options{Username: "player", Library: resumeTestLibrary(), EventStore: store})
	target, ok := model.drill.Target()
	if !ok {
		t.Fatal("target missing")
	}
	if target.CardID != "lesson-one-b" {
		t.Fatalf("target card = %q, want unlearned lesson-one-b", target.CardID)
	}
}

func TestNewResumesNextLessonAfterSavedCompletion(t *testing.T) {
	store := statestore.NewSQLiteEventStore(filepath.Join(t.TempDir(), "kotoba.sqlite"), "player")
	for _, cardID := range []string{"lesson-one-a", "lesson-one-b"} {
		if err := store.Append(statestore.CardMastered(cardID)); err != nil {
			t.Fatalf("append saved mastery %s: %v", cardID, err)
		}
	}

	model := New(Options{Username: "player", Library: resumeTestLibrary(), EventStore: store})
	if model.levelID != "lesson-two" {
		t.Fatalf("levelID = %q, want lesson-two", model.levelID)
	}
	if view := atoms.StripANSI(model.View()); !strings.Contains(view, "target  二") || strings.Contains(view, "set     ") {
		t.Fatalf("startup view did not resume next lesson:\n%s", view)
	}
}

func TestNewResumesBuiltInLessonAfterSavedCompletion(t *testing.T) {
	library, report := statestore.DefaultLessonLibrary()
	if report.HasErrors() {
		t.Fatalf("default lesson library invalid: %#v", report)
	}
	store := statestore.NewSQLiteEventStore(filepath.Join(t.TempDir(), "kotoba.sqlite"), "player")
	for _, levelID := range []string{"lesson-hi-readings", "lesson-hi-words"} {
		for _, card := range levelCards(library, levelID) {
			if err := store.Append(statestore.CardMastered(card.ID)); err != nil {
				t.Fatalf("append saved mastery %s: %v", card.ID, err)
			}
		}
	}

	model := New(Options{Username: "player", Library: library, EventStore: store})
	if model.levelID != "lesson-hi-sentences" {
		t.Fatalf("levelID = %q, want lesson-hi-sentences", model.levelID)
	}
	if view := atoms.StripANSI(model.View()); !strings.Contains(view, "target") || !strings.Contains(view, "meaning") {
		t.Fatalf("startup view did not resume built-in lesson 3:\n%s", view)
	}
}

func TestNewBuiltInLibraryStartsOnKanaFoundation(t *testing.T) {
	library, report := statestore.DefaultLessonLibrary()
	if report.HasErrors() {
		t.Fatalf("default lesson library invalid: %#v", report)
	}

	model := New(Options{Username: "player", Library: library, DisableEventLog: true})
	if model.levelID != "lesson-kana-hiragana-early" {
		t.Fatalf("levelID = %q, want kana foundation start", model.levelID)
	}
	if model.level != "Kana 1 - Hiragana A/K/S/T/N" {
		t.Fatalf("level = %q, want kana foundation title", model.level)
	}
	view := atoms.StripANSI(model.View())
	for _, want := range []string{"goal    catch the kana wave", "target", "meaning"} {
		if !strings.Contains(view, want) {
			t.Fatalf("kana foundation startup missing %q:\n%s", want, view)
		}
	}
}

func TestExistingBuiltInProgressSkipsNewKanaFoundationOnResume(t *testing.T) {
	library, report := statestore.DefaultLessonLibrary()
	if report.HasErrors() {
		t.Fatalf("default lesson library invalid: %#v", report)
	}
	store := statestore.NewSQLiteEventStore(filepath.Join(t.TempDir(), "kotoba.sqlite"), "player")
	if err := store.Append(statestore.CardMastered("lesson-hi-hi")); err != nil {
		t.Fatalf("append saved mastery: %v", err)
	}

	model := New(Options{Username: "player", Library: library, EventStore: store})
	if model.levelID != "lesson-hi-readings" {
		t.Fatalf("levelID = %q, want existing progress to resume 日 readings", model.levelID)
	}
	view := atoms.StripANSI(model.openStations().View())
	for _, want := range []string{"continue  Kana 1 - Hiragana A/K/S/T/N", "> 01 OPEN   Kana 1 - Hiragana A/K/S/T/N"} {
		if !strings.Contains(view, want) {
			t.Fatalf("route map should focus optional kana practice, missing %q:\n%s", want, view)
		}
	}
}

func TestPointGateControlsExpandedLanguagePack(t *testing.T) {
	library, report := statestore.DefaultLessonLibrary()
	if report.HasErrors() {
		t.Fatalf("default lesson library invalid: %#v", report)
	}
	store := statestore.NewSQLiteEventStore(filepath.Join(t.TempDir(), "kotoba.sqlite"), "player")
	for _, levelID := range []string{"lesson-hi-readings", "lesson-hi-words", "lesson-hi-sentences"} {
		for _, card := range levelCards(library, levelID) {
			if err := store.Append(statestore.CardMastered(card.ID)); err != nil {
				t.Fatalf("append saved mastery %s: %v", card.ID, err)
			}
		}
	}

	model := New(Options{Username: "player", Library: library, EventStore: store}).openStations()
	view := atoms.StripANSI(model.View())
	if !strings.Contains(view, "LOCKED Lesson 4 - Particles And Glue") || !strings.Contains(view, "1200 more points") {
		t.Fatalf("point-gated pack should stay locked without points:\n%s", view)
	}

	if err := store.Append(statestore.Points(1200, "test points")); err != nil {
		t.Fatalf("append points: %v", err)
	}
	model = New(Options{Username: "player", Library: library, EventStore: store})
	if model.levelID != "lesson-hi-particles" {
		t.Fatalf("levelID = %q, want lesson-hi-particles", model.levelID)
	}
}

func TestCompletedBuiltInLessonFiveResumesN5VerbPack(t *testing.T) {
	library, report := statestore.DefaultLessonLibrary()
	if report.HasErrors() {
		t.Fatalf("default lesson library invalid: %#v", report)
	}
	store := statestore.NewSQLiteEventStore(filepath.Join(t.TempDir(), "kotoba.sqlite"), "player")
	for _, levelID := range []string{
		"lesson-hi-readings",
		"lesson-hi-words",
		"lesson-hi-sentences",
		"lesson-hi-particles",
		"lesson-hi-patterns",
	} {
		for _, card := range levelCards(library, levelID) {
			if err := store.Append(statestore.CardMastered(card.ID)); err != nil {
				t.Fatalf("append saved mastery %s: %v", card.ID, err)
			}
		}
	}
	if err := store.Append(statestore.Points(8675, "test points")); err != nil {
		t.Fatalf("append points: %v", err)
	}

	model := New(Options{Username: "player", Library: library, EventStore: store})
	if model.levelID != "lesson-hi-n5-verbs" {
		t.Fatalf("levelID = %q, want lesson-hi-n5-verbs", model.levelID)
	}
	if model.level != "Lesson 6 - N5 Action Verbs" {
		t.Fatalf("level = %q, want Lesson 6 - N5 Action Verbs", model.level)
	}
	view := atoms.StripANSI(model.View())
	for _, want := range []string{"KOTOBA BEACH", "target", "meaning"} {
		if !strings.Contains(view, want) {
			t.Fatalf("completed lesson five should resume lesson six, missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "DOCUMENTS") || strings.Contains(view, "all visible waves done") {
		t.Fatalf("completed lesson five should not land on route-map dead end:\n%s", view)
	}
}

func TestCompletedBuiltInLessonEightResumesBeginner200Core(t *testing.T) {
	library, report := statestore.DefaultLessonLibrary()
	if report.HasErrors() {
		t.Fatalf("default lesson library invalid: %#v", report)
	}
	store := statestore.NewSQLiteEventStore(filepath.Join(t.TempDir(), "kotoba.sqlite"), "player")
	for _, levelID := range []string{
		"lesson-hi-readings",
		"lesson-hi-words",
		"lesson-hi-sentences",
		"lesson-hi-particles",
		"lesson-hi-patterns",
		"lesson-hi-n5-verbs",
		"lesson-hi-n5-anchors",
		"lesson-hi-n5-sentence-trees",
	} {
		for _, card := range levelCards(library, levelID) {
			if err := store.Append(statestore.CardMastered(card.ID)); err != nil {
				t.Fatalf("append saved mastery %s: %v", card.ID, err)
			}
		}
	}
	if err := store.Append(statestore.Points(6000, "test points")); err != nil {
		t.Fatalf("append points: %v", err)
	}

	model := New(Options{Username: "player", Library: library, EventStore: store})
	if model.levelID != "lesson-b200-g01-core" {
		t.Fatalf("levelID = %q, want lesson-b200-g01-core", model.levelID)
	}
	if model.level != "Beginner 200.01 - Numbers And Time Core" {
		t.Fatalf("level = %q", model.level)
	}
	view := atoms.StripANSI(model.View())
	for _, want := range []string{"KOTOBA BEACH", "target", "meaning"} {
		if !strings.Contains(view, want) {
			t.Fatalf("completed lesson eight should resume beginner 200 core, missing %q:\n%s", want, view)
		}
	}
}

func TestCompletedFirstFiftyBeginnerKanjiResumesGroupSix(t *testing.T) {
	library, report := statestore.DefaultLessonLibrary()
	if report.HasErrors() {
		t.Fatalf("default lesson library invalid: %#v", report)
	}
	store := statestore.NewSQLiteEventStore(filepath.Join(t.TempDir(), "kotoba.sqlite"), "player")
	for _, level := range library.Levels {
		for _, card := range levelCards(library, level.ID) {
			if err := store.Append(statestore.CardMastered(card.ID)); err != nil {
				t.Fatalf("append saved mastery %s: %v", card.ID, err)
			}
		}
		if level.ID == "lesson-b200-g05-sentences" {
			break
		}
	}
	if err := store.Append(statestore.Points(20000, "test points")); err != nil {
		t.Fatalf("append points: %v", err)
	}

	model := New(Options{Username: "player", Library: library, EventStore: store})
	if model.levelID != "lesson-b200-g06-core" {
		t.Fatalf("levelID = %q, want lesson-b200-g06-core", model.levelID)
	}
	target, ok := model.drill.Target()
	if !ok {
		t.Fatal("group six target missing")
	}
	if target.CardID < "lesson-b200-051-" || target.CardID > "lesson-b200-060-word" {
		t.Fatalf("target card = %q, want group six card", target.CardID)
	}
	view := atoms.StripANSI(model.View())
	for _, want := range []string{"KOTOBA BEACH", "target", "meaning"} {
		if !strings.Contains(view, want) {
			t.Fatalf("completed first fifty should resume group six, missing %q:\n%s", want, view)
		}
	}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updated.(Model).openStations()
	view = atoms.StripANSI(model.View())
	for _, want := range []string{"continue  Beginner 200.06 - Language And Reading Core", "progress 050/200 lit", "showing 041-070 of 200", "051-060"} {
		if !strings.Contains(view, want) {
			t.Fatalf("route map should point to group six, missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "all visible waves done") {
		t.Fatalf("first fifty complete should not be route-clear:\n%s", view)
	}
	if lines := strings.Split(view, "\n"); len(lines) > 24 {
		t.Fatalf("group six route map has %d lines, want <= 24:\n%s", len(lines), view)
	}
}

func TestHitRefreshesDrillToUnlearnedCards(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	model := New(Options{Username: "player", Library: resumeTestLibrary(), EventLogPath: path})

	model = masterCardClean(t, model, "lesson-one-a")
	target, ok := model.drill.Target()
	if !ok {
		t.Fatal("target missing after first hit")
	}
	if target.CardID != "lesson-one-b" {
		t.Fatalf("target card after first hit = %q, want unlearned lesson-one-b", target.CardID)
	}
}

func TestLessonCompletionAutoAdvancesToNextLesson(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	model := New(Options{Username: "player", Library: resumeTestLibrary(), EventLogPath: path})

	model = masterCardClean(t, model, "lesson-one-a")
	if model.levelID != "lesson-one" {
		t.Fatalf("levelID after first card = %q, want lesson-one", model.levelID)
	}

	model = masterCardClean(t, model, "lesson-one-b")
	if model.levelID != "lesson-two" {
		t.Fatalf("levelID after lesson completion = %q, want lesson-two", model.levelID)
	}
	view := atoms.StripANSI(model.View())
	if !strings.Contains(view, "feedback lesson clear -> Lesson Two") || strings.Contains(view, "WAVE CLEAR") {
		t.Fatalf("view did not advance to next lesson:\n%s", view)
	}
}

func TestFinalLessonCompletionReturnsToRouteMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	model := New(Options{Username: "player", Library: singleLessonLibrary(), EventLogPath: path})

	model = masterCardClean(t, model, "single-a")
	if model.mode != modeStations {
		t.Fatalf("mode after final completion = %q, want stations", model.mode)
	}
	if target, ok := model.drill.Target(); ok {
		t.Fatalf("final completed lesson respawned target %q", target.CardID)
	}
	view := atoms.StripANSI(model.View())
	for _, want := range []string{"DOCUMENTS", "lesson clear -> route map"} {
		if !strings.Contains(view, want) {
			t.Fatalf("final clear route map missing %q:\n%s", want, view)
		}
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("Update(enter) returned command, want nil")
	}
	model = updated.(Model)
	if model.mode != modeStations {
		t.Fatalf("mode after dismissing final clear = %q, want stations", model.mode)
	}
	view = atoms.StripANSI(model.View())
	for _, want := range []string{"DOCUMENTS", "DONE   Only Wave", "route clear"} {
		if !strings.Contains(view, want) {
			t.Fatalf("route map after final clear missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "SURF RUN") || strings.Contains(view, "target  波") {
		t.Fatalf("finished lesson should not reopen the completed drill:\n%s", view)
	}
}

func TestNewStartsOnRouteMapWhenSavedPackIsComplete(t *testing.T) {
	store := statestore.NewSQLiteEventStore(filepath.Join(t.TempDir(), "kotoba.sqlite"), "player")
	if err := store.Append(statestore.CardMastered("single-a")); err != nil {
		t.Fatalf("append saved mastery: %v", err)
	}

	model := New(Options{Username: "player", Library: singleLessonLibrary(), EventStore: store})
	if model.mode != modeStations {
		t.Fatalf("mode after loading completed pack = %q, want stations", model.mode)
	}
	view := atoms.StripANSI(model.View())
	for _, want := range []string{"DOCUMENTS", "DONE   Only Wave", "ROUTE MAP"} {
		if !strings.Contains(view, want) {
			t.Fatalf("completed saved pack view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "SURF RUN") || strings.Contains(view, "target  波") {
		t.Fatalf("completed saved pack should not reopen the drill:\n%s", view)
	}
}

func TestCompletedRouteMapEscapeDoesNotRenderEmptySurfRun(t *testing.T) {
	store := statestore.NewSQLiteEventStore(filepath.Join(t.TempDir(), "kotoba.sqlite"), "player")
	if err := store.Append(statestore.CardMastered("single-a")); err != nil {
		t.Fatalf("append saved mastery: %v", err)
	}
	model := New(Options{Username: "player", Library: singleLessonLibrary(), EventStore: store})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatalf("Update(esc) returned command, want nil")
	}
	model = updated.(Model)
	if model.mode != modeStations {
		t.Fatalf("mode after esc from completed map = %q, want stations", model.mode)
	}
	view := atoms.StripANSI(model.View())
	for _, want := range []string{"DOCUMENTS", "DONE   Only Wave", "all visible waves done"} {
		if !strings.Contains(view, want) {
			t.Fatalf("completed map after esc missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "SURF RUN") || strings.Contains(view, "waiting for the next wave") {
		t.Fatalf("completed map should not render empty surf run after esc:\n%s", view)
	}
}

func TestEmptyDrillRenderNormalizesToRouteMap(t *testing.T) {
	store := statestore.NewSQLiteEventStore(filepath.Join(t.TempDir(), "kotoba.sqlite"), "player")
	if err := store.Append(statestore.CardMastered("single-a")); err != nil {
		t.Fatalf("append saved mastery: %v", err)
	}
	model := New(Options{Username: "player", Library: singleLessonLibrary(), EventStore: store})
	model.mode = modeDrill

	view := atoms.StripANSI(model.View())
	for _, want := range []string{"DOCUMENTS", "DONE   Only Wave", "all visible waves done"} {
		if !strings.Contains(view, want) {
			t.Fatalf("normalized empty drill view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "SURF RUN") || strings.Contains(view, "waiting for the next wave") {
		t.Fatalf("empty drill render should normalize to route map:\n%s", view)
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
	target, ok := model.drill.Target()
	if !ok {
		t.Fatalf("test drill has no target")
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if cmd != nil {
		t.Fatalf("Update(?) returned command, want nil")
	}
	model = updated.(Model)
	if model.drill.Hints() != 1 || model.drill.Hits() != 0 || model.drill.Misses() != 0 {
		t.Fatalf("hint counts = hits %d misses %d hints %d, want 0/0/1", model.drill.Hits(), model.drill.Misses(), model.drill.Hints())
	}
	wantHint := "hint    " + target.Text + " = " + target.Kana
	if !strings.Contains(atoms.StripANSI(model.View()), wantHint) {
		t.Fatalf("hint not shown in view:\n%s", model.View())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(target.RomajiHint)})
	model = updated.(Model)

	if model.drill.Hits() != 1 || model.drill.Misses() != 0 {
		t.Fatalf("typing counts = hits %d misses %d, want 1/0", model.drill.Hits(), model.drill.Misses())
	}
	if !strings.Contains(atoms.StripANSI(model.View()), "feedback saved "+target.Text) {
		t.Fatalf("hit not shown in view:\n%s", model.View())
	}
	if strings.Contains(atoms.StripANSI(model.View()), "BLAST") {
		t.Fatalf("hit should not add an interrupt card:\n%s", model.View())
	}
}

func TestKeyboardInputShowsKanaPreviewAndHits(t *testing.T) {
	model := New(Options{Library: testLibrary(), DisableEventLog: true})
	card := mustLibraryCard(t, model.library, "nihon")
	model.drill = game.NewDrillFromCards([]content.Card{card}, game.Config{SpawnEvery: 6, MaxEnemies: 1}).Start()

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("nihon")})
	if cmd != nil {
		t.Fatalf("Update(nihon) returned command, want nil")
	}
	model = updated.(Model)
	if model.drill.Hits() != 1 || model.drill.Misses() != 0 {
		t.Fatalf("typing counts = hits %d misses %d, want 1/0", model.drill.Hits(), model.drill.Misses())
	}
	if !strings.Contains(atoms.StripANSI(model.View()), "feedback saved 日本") {
		t.Fatalf("automatic keyboard hit not shown in view:\n%s", model.View())
	}
}

func TestWrongKeyIsRejectedUntilCorrected(t *testing.T) {
	model := New(Options{Library: testLibrary(), DisableEventLog: true})
	card := mustLibraryCard(t, model.library, "nihon")
	model.drill = game.NewDrillFromCards([]content.Card{card}, game.Config{SpawnEvery: 6, MaxEnemies: 1}).Start()

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd != nil {
		t.Fatalf("Update(x) returned command, want nil")
	}
	model = updated.(Model)
	view := atoms.StripANSI(model.View())
	if !strings.Contains(view, "[ keys _") || !strings.Contains(view, "feedback wipeout X -5  [N]") {
		t.Fatalf("wrong key feedback missing:\n%s", view)
	}
	if model.drill.Misses() != 0 {
		t.Fatalf("wrong key should not count whole-card miss: %d", model.drill.Misses())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = updated.(Model)
	view = atoms.StripANSI(model.View())
	if !strings.Contains(view, "target  [に]ほん") || !strings.Contains(view, "sound   に [I]") || !strings.Contains(view, "[ keys n_") {
		t.Fatalf("correct key should advance sound cue:\n%s", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	model = updated.(Model)
	view = atoms.StripANSI(model.View())
	if strings.Contains(view, "wipeout") || !strings.Contains(view, "[ keys _") {
		t.Fatalf("backspace should clear accepted key and feedback:\n%s", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("nihon")})
	model = updated.(Model)
	if model.input != "" || model.drill.Hits() != 1 {
		t.Fatalf("correct key sequence should auto-save and clear input: input=%q hits=%d", model.input, model.drill.Hits())
	}
}

func TestSpaceKeyWorksInRomajiAnswers(t *testing.T) {
	model := New(Options{Library: testLibrary(), DisableEventLog: true})
	card := mustLibraryCard(t, model.library, "phrase-hi-ga-kureru")
	model.drill = game.NewDrillFromCards([]content.Card{card}, game.Config{SpawnEvery: 6, MaxEnemies: 1}).Start()

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	model = updated.(Model)
	if model.input != "hi " {
		t.Fatalf("space key input = %q, want %q", model.input, "hi ")
	}
	if strings.Contains(atoms.StripANSI(model.View()), "wrong") {
		t.Fatalf("space after valid prefix should not be wrong:\n%s", model.View())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ga kureru")})
	model = updated.(Model)
	if model.drill.Hits() != 1 {
		t.Fatalf("spaced romaji answer hits = %d, want 1", model.drill.Hits())
	}
}

func TestDrillViewShowsTargetAndQueueWithoutRows(t *testing.T) {
	model := New(Options{Library: testLibrary(), DisableEventLog: true})
	model.drill = model.drill.Tick()
	model.drill = model.drill.Tick()
	model.drill = model.drill.Tick()
	model.drill, _ = model.drill.Spawn()

	view := atoms.StripANSI(model.View())
	for _, want := range []string{"KOTOBA BEACH", "goal    catch the kana wave", "target", "meaning", "sound", "[ keys _", "feedback"} {
		if !strings.Contains(view, want) {
			t.Fatalf("drill view missing %q:\n%s", want, view)
		}
	}
	for _, bad := range []string{"row ", "SHELVES", "DOCUMENT", "TREE", "SQLite Lesson"} {
		if strings.Contains(view, bad) {
			t.Fatalf("drill view should not expose noisy marker %q:\n%s", bad, view)
		}
	}
}

func TestFullWidthQuestionMarkRevealsHint(t *testing.T) {
	model := New(Options{Library: testLibrary(), DisableEventLog: true})
	target, ok := model.drill.Target()
	if !ok {
		t.Fatalf("test drill has no target")
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'？'}})
	if cmd != nil {
		t.Fatalf("Update(？) returned command, want nil")
	}
	model = updated.(Model)
	if !strings.Contains(atoms.StripANSI(model.View()), "hint    "+target.Text+" = "+target.Kana) {
		t.Fatalf("full-width hint not shown in view:\n%s", model.View())
	}
}

func TestLongHintAndFeedbackWrapInsideHUD(t *testing.T) {
	library, report := statestore.DefaultLessonLibrary()
	if report.HasErrors() {
		t.Fatalf("default lesson library invalid: %#v", report)
	}
	model := New(Options{Library: library, DisableEventLog: true})
	model.levelID = "lesson-hi-sentences"
	model.level = "Lesson 3 - 日 Sentences"
	card := mustLibraryCard(t, model.library, "lesson-hi-honjitsu-thanks")
	model.drill = game.NewDrillFromCards([]content.Card{card}, game.Config{SpawnEvery: 6, MaxEnemies: 1}).Start()
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 28})
	model = updated.(Model)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if cmd != nil {
		t.Fatalf("Update(?) returned command, want nil")
	}
	model = updated.(Model)
	view := atoms.StripANSI(model.View())
	for _, want := range []string{
		"goal    catch the kana wave",
		"target  本日は誠にありがとうございました",
		"meaning Thank you very much today.",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("center prompt missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "tree time") || strings.Contains(view, "tree topic") {
		t.Fatalf("drill prompt should not render sentence tree rows:\n%s", view)
	}
	for _, want := range []string{"hint    本日は誠にありがとうございました =", "ほんじつはまことにありがとうございました", "arigatou"} {
		if !strings.Contains(view, want) {
			t.Fatalf("wrapped hint missing %q:\n%s", want, view)
		}
	}
	assertViewLineWidths(t, view, 80)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("honjitsuwaar")})
	model = updated.(Model)
	view = atoms.StripANSI(model.View())
	for _, want := range []string{"target  [ほんじつ]はまことにありがとうございました", "sound   ほんじつ [H]", "[ keys honjitsu _", "feedback wipeout W -5  [H]"} {
		if !strings.Contains(view, want) {
			t.Fatalf("wrapped feedback missing %q:\n%s", want, view)
		}
	}
	assertViewLineWidths(t, view, 80)
}

func TestDrillLettersStayInInputInsteadOfSwitchingModes(t *testing.T) {
	model := New(Options{DisableEventLog: true})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bcjs")})
	if cmd != nil {
		t.Fatalf("Update(bcjs) returned command, want nil")
	}
	model = updated.(Model)

	view := atoms.StripANSI(model.View())
	for _, want := range []string{
		"KOTOBA BEACH",
		"goal    catch the kana wave",
		"[ keys _",
		"feedback wipeout",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("drill input view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Wave: Constitution Gate") || strings.Contains(view, "BOSS") {
		t.Fatalf("letter input should not switch modes:\n%s", view)
	}
}

func TestEscapeFromDrillOpensDocumentsWhenInputEmpty(t *testing.T) {
	model := New(Options{Library: testLibrary(), DisableEventLog: true})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatalf("Update(esc) returned command, want nil")
	}
	model = updated.(Model)
	view := atoms.StripANSI(model.View())
	if model.mode != modeStations || !strings.Contains(view, "DOCUMENTS") || !strings.Contains(view, "KANJI GRID") {
		t.Fatalf("esc from empty drill should open documents:\n%s", view)
	}
}

func TestEscapeFromDrillClearsInputBeforeDocuments(t *testing.T) {
	model := New(Options{Library: testLibrary(), DisableEventLog: true})
	model.input = "ni"

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.mode != modeDrill || model.input != "" {
		t.Fatalf("esc with input should only clear input: mode=%q input=%q", model.mode, model.input)
	}
}

func TestStationSelectorShowsOpenAndLockedLevels(t *testing.T) {
	model := New(Options{DisableEventLog: true}).openStations()

	view := atoms.StripANSI(model.View())
	for _, want := range []string{
		"DOCUMENTS",
		"Tide Gate",
		"Calendar Breakers",
		"Today And Around It",
		"locked",
		"ROUTE MAP",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("station selector missing %q:\n%s", want, view)
		}
	}
}

func TestStationSelectorShowsBeginner200KanjiGrid(t *testing.T) {
	library, report := statestore.DefaultLessonLibrary()
	if report.HasErrors() {
		t.Fatalf("default lesson library invalid: %#v", report)
	}
	model := New(Options{Library: library, DisableEventLog: true})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 80})
	model = updated.(Model).openStations()

	view := model.View()
	plain := atoms.StripANSI(view)
	for _, want := range []string{
		"DOCUMENTS",
		"KANA MATRIX",
		"vowel  a あ/ア",
		"KANJI GRID",
		"mastered",
		"current",
		"open",
		"locked",
		"gate",
		"001-010",
		"一 二 三 四 五 六 七 八 九 十",
		"191-200",
		"楽 歌 写 真 旅 病 院 薬 医 者",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("kanji grid missing %q:\n%s", want, plain)
		}
	}
	if !strings.Contains(view, string(atoms.DeepNavy)) {
		t.Fatalf("kanji grid should color locked cells:\n%s", view)
	}
	assertViewLineWidths(t, view, 120)
}

func TestStationSelectorFitsStandardRouteMapViewport(t *testing.T) {
	library, report := statestore.DefaultLessonLibrary()
	if report.HasErrors() {
		t.Fatalf("default lesson library invalid: %#v", report)
	}
	model := New(Options{Library: library, DisableEventLog: true})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = updated.(Model).openStations()

	view := model.View()
	plain := atoms.StripANSI(view)
	for _, want := range []string{
		"continue  Kana 1 - Hiragana A/K/S/T/N",
		"KANA MATRIX",
		"vowel  a あ/ア",
		"showing 001-080 of 200",
		"051-060",
		"... 52 later documents",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("route map missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "191-200") {
		t.Fatalf("standard route map viewport should window the grid:\n%s", plain)
	}
	if lines := strings.Split(plain, "\n"); len(lines) > 40 {
		t.Fatalf("route map view has %d lines, want <= 40:\n%s", len(lines), plain)
	}
	assertViewLineWidths(t, view, 120)
}

func TestCompactKanjiGridShowsCurrentRows(t *testing.T) {
	library, report := statestore.DefaultLessonLibrary()
	if report.HasErrors() {
		t.Fatalf("default lesson library invalid: %#v", report)
	}
	store := statestore.NewSQLiteEventStore(filepath.Join(t.TempDir(), "kotoba.sqlite"), "player")
	for _, levelID := range []string{
		"lesson-hi-readings",
		"lesson-hi-words",
		"lesson-hi-sentences",
		"lesson-hi-particles",
		"lesson-hi-patterns",
		"lesson-hi-n5-verbs",
		"lesson-hi-n5-anchors",
		"lesson-hi-n5-sentence-trees",
	} {
		for _, card := range levelCards(library, levelID) {
			if err := store.Append(statestore.CardMastered(card.ID)); err != nil {
				t.Fatalf("append saved mastery %s: %v", card.ID, err)
			}
		}
	}
	if err := store.Append(statestore.Points(6000, "test points")); err != nil {
		t.Fatalf("append points: %v", err)
	}
	model := New(Options{Library: library, EventStore: store})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updated.(Model).openStations()

	view := model.View()
	plain := atoms.StripANSI(view)
	for _, want := range []string{"showing 001-030 of 200", "001-010", "021-030"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("compact grid missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "041-050") {
		t.Fatalf("compact grid should not show far rows:\n%s", plain)
	}
	if !strings.Contains(view, string(atoms.Yellow)) {
		t.Fatalf("current row should include yellow styling:\n%s", view)
	}
	assertViewLineWidths(t, view, 80)
}

func TestNoteDrivenSentenceTreeLines(t *testing.T) {
	library, report := statestore.DefaultLessonLibrary()
	if report.HasErrors() {
		t.Fatalf("default lesson library invalid: %#v", report)
	}
	card := mustLibraryCard(t, library, "lesson-b200-g01-s01")
	lines := strings.Join(sentenceTreeLines(card), "\n")
	for _, want := range []string{
		"tree time     一月一日 / いちがつついたち = January first",
		"tree topic    は = marks the day as the frame",
		"tree state    休みです / やすみです = is a rest day",
	} {
		if !strings.Contains(lines, want) {
			t.Fatalf("note-driven tree missing %q:\n%s", want, lines)
		}
	}
}

func TestRequirementLinesCapsLongLists(t *testing.T) {
	cards := []content.Card{
		{Text: "一", Reading: content.Reading{Kana: "いち"}},
		{Text: "二", Reading: content.Reading{Kana: "に"}},
		{Text: "三", Reading: content.Reading{Kana: "さん"}},
		{Text: "四", Reading: content.Reading{Kana: "よん"}},
		{Text: "五", Reading: content.Reading{Kana: "ご"}},
		{Text: "六", Reading: content.Reading{Kana: "ろく"}},
		{Text: "七", Reading: content.Reading{Kana: "なな"}},
		{Text: "八", Reading: content.Reading{Kana: "はち"}},
	}
	lines := requirementLines(cards)
	if len(lines) != 7 {
		t.Fatalf("requirement line count = %d, want 7: %#v", len(lines), lines)
	}
	if lines[6] != "+2 more locks" {
		t.Fatalf("capped requirement suffix = %q", lines[6])
	}
}

func TestStationSelectorSwitchesSelectedOpenLevel(t *testing.T) {
	model := New(Options{DisableEventLog: true}).openStations()

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jjjj")})
	if cmd != nil {
		t.Fatalf("Update(jjjj) returned command, want nil")
	}
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	view := atoms.StripANSI(model.View())
	for _, want := range []string{
		"Constitution Gate: Preamble 1",
		"target",
		"feedback wave Constitution Gate",
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
		"DOCUMENTS",
		"LOCKED Article 1: Symbol Of The State",
		"needs:",
		"日本国民は/にほんこくみんは",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("locked level view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Wave: Article 1: Symbol Of The State") {
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
		"KOTOBA BEACH",
		"target",
		"feedback wave Article 1",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("unlocked level view missing %q:\n%s", want, view)
		}
	}
}

func TestKanaActionsAppendStateEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	model := New(Options{Library: testLibrary(), EventLogPath: path})
	target, ok := model.drill.Target()
	if !ok {
		t.Fatalf("test drill has no target")
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(kanainput.TypingSequence(target.Kana, target.RomajiHint))})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	events, err := statestore.NewEventLog(path).ReadAll()
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("event count = %d, want 4: %#v", len(events), events)
	}
	if events[0].Type != statestore.EventHintRevealed || events[0].CardID != target.CardID {
		t.Fatalf("first event = %#v, want hint for %s", events[0], target.CardID)
	}
	if events[1].Type != statestore.EventPoints || events[1].Points != -10 {
		t.Fatalf("second event = %#v, want hint point tax", events[1])
	}
	if events[2].Type != statestore.EventEnemyHit || events[2].CardID != target.CardID {
		t.Fatalf("third event = %#v, want hit for %s", events[2], target.CardID)
	}
	if events[2].Clean == nil || *events[2].Clean {
		t.Fatalf("hinted drill hit should be unclean: %#v", events[2])
	}
	if events[3].Type != statestore.EventPoints || events[3].Points <= 0 {
		t.Fatalf("fourth event = %#v, want positive hit points", events[3])
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

func TestHintedPrerequisiteHitsUnlockGatedArticleLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	model := New(Options{EventLogPath: path}).switchLevel(constitutionLevelID)

	for _, cardID := range articleOnePrerequisiteCardIDs() {
		model = hitCardWithHint(t, model, cardID)
	}

	progress, err := statestore.NewEventLog(path).Replay()
	if err != nil {
		t.Fatalf("replay event log: %v", err)
	}
	if !progress.UnlockedLevels["constitution-article-1"] {
		t.Fatalf("article level should unlock from hinted correct hits: %#v", progress.UnlockedLevels)
	}
	for _, cardID := range articleOnePrerequisiteCardIDs() {
		card := progress.Cards[cardID]
		if !card.Mastered {
			t.Fatalf("card %s should be mastered from hinted correct hit: %#v", cardID, card)
		}
	}

	events, err := statestore.NewEventLog(path).ReadAll()
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	for _, event := range events {
		if event.Type == statestore.EventLevelUnlocked && event.LevelID == "constitution-article-1" {
			return
		}
	}
	t.Fatalf("missing article unlock event: %#v", events)
}

func TestBossModeDamagesBossAndRendersTransition(t *testing.T) {
	model := New(Options{Library: testLibrary(), DisableEventLog: true}).enterBoss()
	startHP := model.boss.HP()

	view := atoms.StripANSI(model.View())
	for _, want := range []string{"BOSS", "boss    Tide Gate", "target  日が暮れる"} {
		if !strings.Contains(view, want) {
			t.Fatalf("boss view missing %q:\n%s", want, view)
		}
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ひがくれる")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if model.boss.HP() != startHP-1 {
		t.Fatalf("boss HP = %d, want %d", model.boss.HP(), startHP-1)
	}
	view = atoms.StripANSI(model.View())
	for _, want := range []string{"boss    Tide Gate", "burst 日が暮れる -1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("boss hit view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "CRACK") {
		t.Fatalf("boss hit should not add an interrupt card:\n%s", view)
	}
}

func TestBossModeAppendsReplayableStateEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	model := New(Options{Library: testLibrary(), EventLogPath: path}).enterBoss()

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
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
	if len(events) != 13 {
		t.Fatalf("event count = %d, want 13: %#v", len(events), events)
	}
	if events[0].Type != statestore.EventBossIntro || events[0].BossID != "gate-journal-2026-06-22-key-readings" {
		t.Fatalf("first event = %#v, want boss intro", events[0])
	}
	if events[1].Type != statestore.EventHintRevealed || events[1].CardID != "phrase-hi-ga-kureru" {
		t.Fatalf("second event = %#v, want boss hint", events[1])
	}
	if events[2].Type != statestore.EventPoints || events[2].Points != -10 {
		t.Fatalf("third event = %#v, want boss hint point tax", events[2])
	}
	if events[3].Type != statestore.EventEnemyHit || events[3].Clean == nil || *events[3].Clean {
		t.Fatalf("first boss hit should be unclean after hint: %#v", events[3])
	}
	if events[len(events)-1].Type != statestore.EventBossCleared {
		t.Fatalf("last event = %#v, want boss cleared", events[len(events)-1])
	}
	if !strings.Contains(atoms.StripANSI(model.View()), "clear Tide Gate") {
		t.Fatalf("clear view missing status:\n%s", model.View())
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
	input := kana
	if target, ok := model.drill.Target(); ok && target.Kana == kana && target.RomajiHint != "" {
		input = kanainput.TypingSequence(target.Kana, target.RomajiHint)
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(input)})
	if cmd != nil {
		t.Fatalf("Update(%q) returned command, want nil", input)
	}
	model = updated.(Model)
	if model.input != "" {
		updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil {
			t.Fatalf("Update(enter) returned command, want nil")
		}
		return updated.(Model)
	}
	return model
}

func assertViewLineWidths(t *testing.T, view string, width int) {
	t.Helper()
	for i, line := range strings.Split(view, "\n") {
		if got := atoms.DisplayWidth(line); got > width {
			t.Fatalf("line %d width = %d, want <= %d: %q\n%s", i+1, got, width, line, view)
		}
	}
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

func resumeTestLibrary() *content.Library {
	return &content.Library{
		Cards: []content.Card{
			{
				ID:       "lesson-one-a",
				Text:     "一",
				Reading:  content.Reading{Kana: "いち", RomajiHint: "ichi"},
				Meaning:  "one",
				Type:     content.CardTypeWord,
				Playable: true,
			},
			{
				ID:       "lesson-one-b",
				Text:     "日",
				Reading:  content.Reading{Kana: "ひ", RomajiHint: "hi"},
				Meaning:  "sun",
				Type:     content.CardTypeWord,
				Playable: true,
			},
			{
				ID:       "lesson-two-a",
				Text:     "二",
				Reading:  content.Reading{Kana: "に", RomajiHint: "ni"},
				Meaning:  "two",
				Type:     content.CardTypeWord,
				Playable: true,
			},
		},
		Levels: []content.Level{
			{ID: "lesson-one", Title: "Lesson One", CardIDs: []string{"lesson-one-a", "lesson-one-b"}},
			{ID: "lesson-two", Title: "Lesson Two", CardIDs: []string{"lesson-two-a"}, RequiredCardIDs: []string{"lesson-one-a", "lesson-one-b"}},
		},
		Campaigns: []content.Campaign{{
			ID:           "resume-test",
			Title:        "Resume Test",
			LevelIDs:     []string{"lesson-one", "lesson-two"},
			StartLevelID: "lesson-one",
		}},
	}
}

func singleLessonLibrary() *content.Library {
	return &content.Library{
		Cards: []content.Card{
			{
				ID:       "single-a",
				Text:     "波",
				Reading:  content.Reading{Kana: "なみ", RomajiHint: "nami"},
				Meaning:  "wave",
				Type:     content.CardTypeWord,
				Playable: true,
			},
		},
		Levels: []content.Level{
			{ID: "only-wave", Title: "Only Wave", CardIDs: []string{"single-a"}},
		},
		Campaigns: []content.Campaign{{
			ID:           "single-test",
			Title:        "Single Test",
			LevelIDs:     []string{"only-wave"},
			StartLevelID: "only-wave",
		}},
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
