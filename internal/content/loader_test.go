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

func indexCardsByTextKana(cards []Card) map[string]Card {
	index := make(map[string]Card, len(cards))
	for _, card := range cards {
		index[card.Text+"\x00"+card.Reading.Kana] = card
	}
	return index
}
