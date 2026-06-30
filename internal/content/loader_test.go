package content

import (
	"path/filepath"
	"testing"
)

func TestLoadSeedContent(t *testing.T) {
	library, report, err := LoadFile(filepath.Join("..", "..", "content", "seed-2026-06-22.json"))
	if err != nil {
		t.Fatalf("load seed content: %v", err)
	}
	if report.HasErrors() {
		t.Fatalf("seed content has validation errors: %#v", report.Issues)
	}

	cards := indexCardsByTextKana(library.Cards)
	for _, want := range []struct {
		text string
		kana string
	}{
		{text: "日", kana: "ひ"},
		{text: "日", kana: "にち"},
		{text: "日", kana: "び"},
		{text: "日", kana: "か"},
		{text: "日本", kana: "にほん"},
		{text: "日本", kana: "にっぽん"},
		{text: "本日", kana: "ほんじつ"},
		{text: "毎日", kana: "まいにち"},
		{text: "1日", kana: "ついたち"},
		{text: "20日", kana: "はつか"},
		{text: "今日", kana: "きょう"},
		{text: "明日", kana: "あした"},
	} {
		if _, ok := cards[want.text+"\x00"+want.kana]; !ok {
			t.Fatalf("missing seed card text=%q kana=%q", want.text, want.kana)
		}
	}

	var dateCards, timeWordCards int
	for _, card := range library.Cards {
		if card.Reading.Kana == "" {
			t.Fatalf("seed card %q has no curated kana", card.ID)
		}
		if !card.Playable {
			t.Fatalf("seed card %q should be playable", card.ID)
		}
		if card.NeedsReview {
			t.Fatalf("seed card %q should not need review", card.ID)
		}
		switch card.Type {
		case CardTypeDate:
			dateCards++
		case CardTypeTimeWord:
			timeWordCards++
		}
	}
	if dateCards != 12 {
		t.Fatalf("date card count = %d, want 12", dateCards)
	}
	if timeWordCards != 6 {
		t.Fatalf("time word card count = %d, want 6", timeWordCards)
	}
}

func TestLoadConstitutionPlayableContent(t *testing.T) {
	library, report, err := LoadFile(filepath.Join("..", "..", "content", "constitution-preamble-article1-playable.json"))
	if err != nil {
		t.Fatalf("load constitution content: %v", err)
	}
	if report.HasErrors() {
		t.Fatalf("constitution content has validation errors: %#v", report.Issues)
	}

	if len(library.Documents) != 1 || library.Documents[0].ID != "jp-constitution" {
		t.Fatalf("documents = %#v, want jp-constitution", library.Documents)
	}
	if got := len(library.Levels); got != 2 {
		t.Fatalf("level count = %d, want 2", got)
	}
	if library.Levels[0].ID != "constitution-preamble-1" || library.Levels[1].ID != "constitution-article-1" {
		t.Fatalf("level ids = %#v", library.Levels)
	}

	cards := indexCardsByTextKana(library.Cards)
	for _, want := range []struct {
		text string
		kana string
	}{
		{text: "日本国民は", kana: "にほんこくみんは"},
		{text: "正当に選挙された", kana: "せいとうにせんきょされた"},
		{text: "国会", kana: "こっかい"},
		{text: "主権", kana: "しゅけん"},
		{text: "第一条", kana: "だいいちじょう"},
		{text: "天皇", kana: "てんのう"},
		{text: "象徴", kana: "しょうちょう"},
	} {
		if _, ok := cards[want.text+"\x00"+want.kana]; !ok {
			t.Fatalf("missing constitution card text=%q kana=%q", want.text, want.kana)
		}
	}

	for _, card := range library.Cards {
		if !card.Playable {
			t.Fatalf("constitution card %q should be playable", card.ID)
		}
		if card.Reading.Kana == "" {
			t.Fatalf("constitution card %q missing kana", card.ID)
		}
		if card.SourceRef == "" {
			t.Fatalf("constitution card %q missing source_ref", card.ID)
		}
	}
}

func TestLoadDefaultPlayableLibrary(t *testing.T) {
	library, report, err := LoadDefaultPlayableLibrary()
	if err != nil {
		t.Fatalf("load default playable library: %v", err)
	}
	if report.HasErrors() {
		t.Fatalf("default playable library has validation errors: %#v", report.Issues)
	}

	for _, want := range []string{
		"journal-2026-06-22-key-readings",
		"constitution-preamble-1",
		"constitution-article-1",
	} {
		if !hasLevel(library, want) {
			t.Fatalf("default playable library missing level %q", want)
		}
	}
	if got, want := library.Campaigns[0].StartLevelID, "journal-2026-06-22-key-readings"; got != want {
		t.Fatalf("first campaign start = %q, want %q", got, want)
	}
}

func TestMergeLibrariesPreservesAppendOrder(t *testing.T) {
	merged := MergeLibraries(
		&Library{
			Cards:     []Card{{ID: "a", Text: "一", Reading: Reading{Kana: "いち"}, Meaning: "one", Type: CardTypeWord}},
			Documents: []Document{{ID: "doc-a", Title: "A"}},
			Levels:    []Level{{ID: "level-a", Title: "A", CardIDs: []string{"a"}}},
			Campaigns: []Campaign{{ID: "campaign-a", Title: "A", LevelIDs: []string{"level-a"}, StartLevelID: "level-a"}},
		},
		nil,
		&Library{
			Cards:     []Card{{ID: "b", Text: "二", Reading: Reading{Kana: "に"}, Meaning: "two", Type: CardTypeWord}},
			Documents: []Document{{ID: "doc-b", Title: "B"}},
			Levels:    []Level{{ID: "level-b", Title: "B", CardIDs: []string{"b"}}},
			Campaigns: []Campaign{{ID: "campaign-b", Title: "B", LevelIDs: []string{"level-b"}, StartLevelID: "level-b"}},
		},
	)

	if got := []string{merged.Cards[0].ID, merged.Cards[1].ID}; got[0] != "a" || got[1] != "b" {
		t.Fatalf("merged card order = %#v, want a then b", got)
	}
	if report := ValidateLibrary(merged); report.HasErrors() {
		t.Fatalf("merged library validation errors: %#v", report.Issues)
	}
}

func TestValidateMarksMissingKanaUnplayable(t *testing.T) {
	library := &Library{
		Cards: []Card{{
			ID:       "draft-card",
			Text:     "未整理",
			Kanji:    "未整理",
			Reading:  Reading{RomajiHint: "miseiri"},
			Meaning:  "uncurated draft",
			Type:     CardTypeWord,
			Playable: true,
		}},
	}

	report := ValidateLibrary(library)
	if !report.HasIssue("missing_kana") {
		t.Fatalf("missing_kana issue not reported: %#v", report.Issues)
	}
	if library.Cards[0].Playable {
		t.Fatalf("card with missing kana should be unplayable")
	}
	if !library.Cards[0].NeedsReview {
		t.Fatalf("card with missing kana should need review")
	}
}

func hasLevel(library *Library, levelID string) bool {
	for _, level := range library.Levels {
		if level.ID == levelID {
			return true
		}
	}
	return false
}

func indexCardsByTextKana(cards []Card) map[string]Card {
	index := make(map[string]Card, len(cards))
	for _, card := range cards {
		index[card.Text+"\x00"+card.Reading.Kana] = card
	}
	return index
}
