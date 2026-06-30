package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/superposition/kotoba-line/internal/content"
)

func TestSQLiteEventStoreTracksUsersAndIsolatesProgress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kotoba.sqlite")
	eric := NewSQLiteEventStore(path, "eric")
	guest := NewSQLiteEventStore(path, "guest")

	if err := eric.TouchUser(); err != nil {
		t.Fatalf("touch eric: %v", err)
	}
	if err := guest.TouchUser(); err != nil {
		t.Fatalf("touch guest: %v", err)
	}
	for _, event := range []Event{
		EnemyHit("card-a"),
		EnemyHit("card-a"),
		EnemyHit("card-a"),
		LevelUnlocked("level-a"),
	} {
		if err := eric.Append(event); err != nil {
			t.Fatalf("append eric event %s: %v", event.Type, err)
		}
	}
	if err := guest.Append(HintRevealed("card-a", "romaji")); err != nil {
		t.Fatalf("append guest hint: %v", err)
	}

	ericUser, err := eric.UserRecord()
	if err != nil {
		t.Fatalf("eric user record: %v", err)
	}
	if ericUser.Username != "eric" || ericUser.CreatedAt == "" || ericUser.LastSeenAt == "" {
		t.Fatalf("bad eric user record: %#v", ericUser)
	}

	ericProgress, err := eric.Replay()
	if err != nil {
		t.Fatalf("replay eric: %v", err)
	}
	if !ericProgress.Cards["card-a"].Mastered || !ericProgress.UnlockedLevels["level-a"] {
		t.Fatalf("eric progress did not replay from sqlite: %#v", ericProgress)
	}

	guestProgress, err := guest.Replay()
	if err != nil {
		t.Fatalf("replay guest: %v", err)
	}
	if guestProgress.Cards["card-a"].Mastered || guestProgress.UnlockedLevels["level-a"] {
		t.Fatalf("guest should not inherit eric progress: %#v", guestProgress)
	}
	if guestProgress.Cards["card-a"].HintsUsed != 1 {
		t.Fatalf("guest hint count = %#v, want one hint", guestProgress.Cards["card-a"])
	}
}

func TestSQLiteEventStorePreservesEventOrderAndCleanFlags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kotoba.sqlite")
	store := NewSQLiteEventStore(path, "player")
	events := []Event{
		HintRevealed("card-a", "romaji"),
		Points(-10, "hint"),
		EnemyHitWithClean("card-a", false),
		Points(40, "hinted hit"),
		EnemyHit("card-a"),
		Points(120, "clean hit"),
		BossIntro("boss-a"),
		BossDamaged("boss-a", "card-a"),
		BossCleared("boss-a"),
	}

	for _, event := range events {
		if err := store.Append(event); err != nil {
			t.Fatalf("append %s: %v", event.Type, err)
		}
	}

	readEvents, err := store.ReadAll()
	if err != nil {
		t.Fatalf("read sqlite events: %v", err)
	}
	if !reflect.DeepEqual(readEvents, events) {
		t.Fatalf("sqlite events differed:\nread: %#v\nwant: %#v", readEvents, events)
	}
	progress, err := store.Replay()
	if err != nil {
		t.Fatalf("replay sqlite points: %v", err)
	}
	if progress.Points != 160 {
		t.Fatalf("sqlite replay points = %d, want 160", progress.Points)
	}
}

func TestSeedSQLiteFromEventLogIfEmpty(t *testing.T) {
	dir := t.TempDir()
	log := NewEventLog(filepath.Join(dir, "events.jsonl"))
	for _, event := range []Event{EnemyHit("card-a"), EnemyHit("card-a"), EnemyHit("card-a")} {
		if err := log.Append(event); err != nil {
			t.Fatalf("append legacy event: %v", err)
		}
	}

	store := NewSQLiteEventStore(filepath.Join(dir, "kotoba.sqlite"), "player")
	seeded, err := SeedSQLiteFromEventLogIfEmpty(store, log)
	if err != nil {
		t.Fatalf("seed sqlite: %v", err)
	}
	if seeded != 3 {
		t.Fatalf("seeded = %d, want 3", seeded)
	}
	seededAgain, err := SeedSQLiteFromEventLogIfEmpty(store, log)
	if err != nil {
		t.Fatalf("seed sqlite again: %v", err)
	}
	if seededAgain != 0 {
		t.Fatalf("second seed = %d, want 0", seededAgain)
	}
	events, err := store.ReadAll()
	if err != nil {
		t.Fatalf("read seeded events: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("seeded event count = %d, want 3", len(events))
	}
}

func TestSeedDefaultLessonsStoresRealLessonLibrary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kotoba.sqlite")
	seeded, err := SeedDefaultLessons(path)
	if err != nil {
		t.Fatalf("seed default lessons: %v", err)
	}
	if seeded == 0 {
		t.Fatalf("seeded = 0, want lesson card rows")
	}

	library, report, err := LoadLessonLibrary(path)
	if err != nil {
		t.Fatalf("load lesson library: %v", err)
	}
	if report.HasErrors() {
		t.Fatalf("lesson library validation errors: %#v", report.Issues)
	}
	if len(library.Levels) != 60 {
		t.Fatalf("level count = %d, want 60", len(library.Levels))
	}
	if got := library.Campaigns[0].StartLevelID; got != "lesson-kana-hiragana-early" {
		t.Fatalf("start level = %q, want lesson-kana-hiragana-early", got)
	}

	cards := map[string]string{}
	cardsByID := map[string]string{}
	for _, card := range library.Cards {
		cards[card.Text] = card.Reading.Kana
		cardsByID[card.ID] = card.Reading.Kana
	}
	for cardID, kana := range map[string]string{
		"lesson-kana-hira-ka":                 "か",
		"lesson-kana-kata-ka":                 "カ",
		"lesson-kana-compare-kata-shi-vs-tsu": "シ",
		"lesson-hi-ka":                        "か",
		"lesson-hi-ikimasu":                   "いきます",
		"lesson-hi-gakkou":                    "がっこう",
	} {
		if cardsByID[cardID] != kana {
			t.Fatalf("card %s kana = %q, want %q", cardID, cardsByID[cardID], kana)
		}
	}
	for text, kana := range map[string]string{
		"日本":          "にっぽん",
		"毎日":          "まいにち",
		"日暮里で毎日日が暮れる": "にっぽりでまいにちひがくれる",
		"今日は学校へ行きます":  "きょうはがっこうへいきます",
		"一月一日は休みです":   "いちがつついたちはやすみです",
	} {
		if cards[text] != kana {
			t.Fatalf("card %s kana = %q, want %q", text, cards[text], kana)
		}
	}
	if len(findLevel(t, library.Levels, "lesson-kana-hiragana-late").RequiredCardIDs) == 0 ||
		len(findLevel(t, library.Levels, "lesson-kana-katakana-early").RequiredCardIDs) == 0 {
		t.Fatalf("kana foundation lessons should require prior lesson cards: %#v", library.Levels)
	}
	if got := len(findLevel(t, library.Levels, "lesson-hi-readings").RequiredCardIDs); got != 14 {
		t.Fatalf("日 readings requirements = %d, want comparison gate cards", got)
	}
	if got := findLevel(t, library.Levels, "lesson-hi-particles").RequiredPoints; got != 1200 {
		t.Fatalf("particles required points = %d, want 1200", got)
	}
	if got := findLevel(t, library.Levels, "lesson-hi-n5-verbs").RequiredPoints; got != 3200 {
		t.Fatalf("n5 verbs required points = %d, want 3200", got)
	}
	if got := findLevel(t, library.Levels, "lesson-b200-g01-core").RequiredPoints; got != 6000 {
		t.Fatalf("beginner group 1 required points = %d, want 6000", got)
	}
	if got := findLevel(t, library.Levels, "lesson-b200-g01-sentences").RequiredCardIDs; len(got) != 20 {
		t.Fatalf("beginner group 1 sentence gate requirements = %d, want 20", len(got))
	}
	if got := findLevel(t, library.Levels, "lesson-b200-g02-core").RequiredCardIDs; len(got) != 5 {
		t.Fatalf("beginner group 2 core requirements = %d, want previous sentence gate cards", len(got))
	}
	boost := findLevel(t, library.Levels, "lesson-anki-n5-verbs-01")
	if len(boost.CardIDs) != 10 {
		t.Fatalf("first Anki boost card count = %d, want 10", len(boost.CardIDs))
	}
	if len(boost.RequiredCardIDs) != 5 {
		t.Fatalf("first Anki boost requirements = %d, want previous sentence gate", len(boost.RequiredCardIDs))
	}
	cardsByID["lesson-anki-n5-v001"] = ""
	for _, card := range library.Cards {
		if card.ID == "lesson-anki-n5-v001" {
			if card.Text != "読みます" || card.Reading.Kana != "よみます" || card.Meaning != "read" {
				t.Fatalf("bad Anki card import: %#v", card)
			}
			cardsByID[card.ID] = card.Reading.Kana
		}
	}
	if cardsByID["lesson-anki-n5-v001"] != "よみます" {
		t.Fatal("missing imported Anki card lesson-anki-n5-v001")
	}
	if got := findLevel(t, library.Levels, "lesson-b200-g20-core").ID; got != "lesson-b200-g20-core" {
		t.Fatalf("final beginner core level = %q, want lesson-b200-g20-core", got)
	}
	if got := library.Levels[len(library.Levels)-1].ID; got != "lesson-anki-n5-verbs-07" {
		t.Fatalf("final beginner level = %q, want final Anki boost", got)
	}
	if got := findLevel(t, library.Levels, "lesson-b200-g20-sentences").ID; got != "lesson-b200-g20-sentences" {
		t.Fatalf("final beginner sentence level = %q, want lesson-b200-g20-sentences", got)
	}
}

func TestDefaultLessonLibraryDoesNotRequireSQLite(t *testing.T) {
	library, report := DefaultLessonLibrary()
	if report.HasErrors() {
		t.Fatalf("default lesson library validation errors: %#v", report.Issues)
	}
	if len(library.Levels) != 60 {
		t.Fatalf("level count = %d, want 60", len(library.Levels))
	}
	if got := library.Campaigns[0].StartLevelID; got != "lesson-kana-hiragana-early" {
		t.Fatalf("start level = %q, want lesson-kana-hiragana-early", got)
	}
	if got := library.Documents[0].Title; got != "Kana And 日 Foundation" {
		t.Fatalf("document title = %q, want storage-neutral title", got)
	}
}

func TestSQLitePasswordUsers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kotoba.sqlite")
	store := NewSQLiteEventStore(path, "system")

	if err := store.CreatePasswordUser("hikari2", "123"); err != nil {
		t.Fatalf("create password user: %v", err)
	}
	if err := store.CreatePasswordUser("hikari2", "123"); err != ErrCredentialExists {
		t.Fatalf("duplicate create error = %v, want ErrCredentialExists", err)
	}
	ok, err := store.AuthenticatePasswordUser("hikari2", "123")
	if err != nil {
		t.Fatalf("authenticate password user: %v", err)
	}
	if !ok {
		t.Fatal("password user did not authenticate")
	}
	ok, err = store.AuthenticatePasswordUser("hikari2", "wrong")
	if err != nil {
		t.Fatalf("authenticate wrong password: %v", err)
	}
	if ok {
		t.Fatal("wrong password authenticated")
	}
	if err := ValidateSignup("no", "123"); err != ErrInvalidUsername {
		t.Fatalf("short username error = %v, want ErrInvalidUsername", err)
	}
	if err := ValidateSignup("valid-user", "12"); err != ErrInvalidPassword {
		t.Fatalf("short password error = %v, want ErrInvalidPassword", err)
	}
}

func findLevel(t *testing.T, levels []content.Level, id string) content.Level {
	t.Helper()
	for _, level := range levels {
		if level.ID == id {
			return level
		}
	}
	t.Fatalf("level %s not found", id)
	return content.Level{}
}

func TestBeginner200KanjiRowsAreExactUniqueGrid(t *testing.T) {
	rows := Beginner200KanjiRows()
	if len(rows) != 20 {
		t.Fatalf("row count = %d, want 20", len(rows))
	}
	if rows[0] != "一二三四五六七八九十" {
		t.Fatalf("first row = %q", rows[0])
	}
	if rows[19] != "楽歌写真旅病院薬医者" {
		t.Fatalf("last row = %q", rows[19])
	}
	seen := map[rune]bool{}
	count := 0
	for _, row := range rows {
		for _, r := range row {
			count++
			if seen[r] {
				t.Fatalf("duplicate kanji %q", r)
			}
			seen[r] = true
		}
	}
	if count != 200 {
		t.Fatalf("kanji count = %d, want 200", count)
	}
}

func TestDefaultSQLitePath(t *testing.T) {
	empty := func(string) (string, bool) { return "", false }
	if got := DefaultSQLitePathFromLookup(empty); got != filepath.Join("state", "kotoba.sqlite") {
		t.Fatalf("local default sqlite path = %q", got)
	}

	values := map[string]string{"RAILWAY_ENVIRONMENT": "production"}
	if got := DefaultSQLitePathFromLookup(mapLookup(values)); got != filepath.Join(string(os.PathSeparator), "data", "kotoba.sqlite") {
		t.Fatalf("railway sqlite path = %q", got)
	}

	values = map[string]string{"KOTOBA_STATE_DIR": "/tmp/kotoba-state"}
	if got := DefaultSQLitePathFromLookup(mapLookup(values)); got != filepath.Join("/tmp/kotoba-state", "kotoba.sqlite") {
		t.Fatalf("state dir sqlite path = %q", got)
	}

	values = map[string]string{"KOTOBA_STATE_DB": "/tmp/custom.sqlite"}
	if got := DefaultSQLitePathFromLookup(mapLookup(values)); got != "/tmp/custom.sqlite" {
		t.Fatalf("explicit sqlite path = %q", got)
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
