package game

import (
	"path/filepath"
	"testing"

	"github.com/superposition/kotoba-line/internal/content"
)

func TestNewDrillSpawnsPlayableSeedCards(t *testing.T) {
	library, report, err := content.LoadFile(filepath.Join("..", "..", "content", "seed-2026-06-22.json"))
	if err != nil {
		t.Fatalf("load seed content: %v", err)
	}
	if report.HasErrors() {
		t.Fatalf("seed content has validation errors: %#v", report.Issues)
	}

	drill := NewDrill(library, Config{})
	if drill.DeckSize() == 0 {
		t.Fatalf("seed content produced no playable drill cards")
	}

	drill, spawned := drill.Spawn()
	if !spawned {
		t.Fatalf("Spawn() returned false for seed content")
	}

	enemies := drill.Enemies()
	if len(enemies) != 1 {
		t.Fatalf("enemy count = %d, want 1", len(enemies))
	}
	if enemies[0].CardID == "" {
		t.Fatalf("spawned enemy has no card id: %#v", enemies[0])
	}
	if enemies[0].Text == "" || enemies[0].Kana == "" {
		t.Fatalf("enemy did not copy seed card reading: %#v", enemies[0])
	}
}

func TestTickMovesEnemiesAndSpawnsOnCadence(t *testing.T) {
	drill := NewDrillFromCards(testCards(), Config{SpawnEvery: 2, MaxEnemies: 3, Seed: 1}).Start()

	drill = drill.Tick()
	enemies := drill.Enemies()
	if len(enemies) != 1 {
		t.Fatalf("enemy count after first tick = %d, want 1", len(enemies))
	}
	if enemies[0].Row != 1 {
		t.Fatalf("enemy row after first tick = %d, want 1", enemies[0].Row)
	}

	drill = drill.Tick()
	enemies = drill.Enemies()
	if len(enemies) != 2 {
		t.Fatalf("enemy count after spawn tick = %d, want 2", len(enemies))
	}
	if enemies[0].Row != 2 {
		t.Fatalf("first enemy row after second tick = %d, want 2", enemies[0].Row)
	}
	if enemies[1].Row != 0 {
		t.Fatalf("spawned enemy row = %#v, want row 0", enemies[1])
	}
	if enemies[1].CardID == enemies[0].CardID {
		t.Fatalf("spawned enemy repeated active card despite alternatives: %#v", enemies)
	}
}

func TestSubmitKanaDestroysExactMatchingEnemy(t *testing.T) {
	drill := NewDrillFromCards(testCards()[:1], Config{}).Start()

	drill, result := drill.SubmitKana("ひ")
	if result.Status != AnswerHit {
		t.Fatalf("SubmitKana status = %q, want hit", result.Status)
	}
	if result.Enemy.CardID != "hi" {
		t.Fatalf("hit enemy = %q, want hi", result.Enemy.CardID)
	}
	if len(drill.Enemies()) != 0 {
		t.Fatalf("hit should destroy matching enemy: %#v", drill.Enemies())
	}
	if drill.Hits() != 1 || drill.Misses() != 0 {
		t.Fatalf("score = hits %d misses %d, want 1/0", drill.Hits(), drill.Misses())
	}
}

func TestSubmitKeyboardSyllablesDestroysMatchingEnemy(t *testing.T) {
	drill := NewDrillFromCards(testCards()[1:], Config{}).Start()

	drill, result := drill.SubmitKana("nihon")
	if result.Status != AnswerHit {
		t.Fatalf("SubmitKana status = %q, want hit", result.Status)
	}
	if result.Enemy.CardID != "nihon" {
		t.Fatalf("hit enemy = %q, want nihon", result.Enemy.CardID)
	}
	if len(drill.Enemies()) != 0 {
		t.Fatalf("hit should destroy matching enemy: %#v", drill.Enemies())
	}
}

func TestWrongAnswerIsRejectedAndLeavesEnemy(t *testing.T) {
	drill := NewDrillFromCards(testCards()[:1], Config{}).Start()
	before := drill.Enemies()[0]

	drill, result := drill.SubmitKana("wrong")
	if result.Status != AnswerMiss {
		t.Fatalf("SubmitKana status = %q, want miss", result.Status)
	}
	if result.Enemy.CardID != before.CardID {
		t.Fatalf("miss target = %q, want %q", result.Enemy.CardID, before.CardID)
	}

	enemies := drill.Enemies()
	if len(enemies) != 1 || enemies[0] != before {
		t.Fatalf("wrong answer changed enemies: before=%#v after=%#v", before, enemies)
	}
	if drill.Hits() != 0 || drill.Misses() != 1 {
		t.Fatalf("score = hits %d misses %d, want 0/1", drill.Hits(), drill.Misses())
	}
}

func TestSubmitTargetKanaOnlyGradesVisibleTarget(t *testing.T) {
	drill := NewDrillFromCards(testCards(), Config{Seed: 7, MaxEnemies: 2}).Start()
	drill, _ = drill.Spawn()
	before := drill.Enemies()
	if len(before) != 2 {
		t.Fatalf("enemy count = %d, want 2", len(before))
	}

	drill, result := drill.SubmitTargetKana(before[1].Kana)
	if result.Status != AnswerMiss {
		t.Fatalf("SubmitTargetKana queued answer status = %q, want miss", result.Status)
	}
	if result.Enemy.CardID != before[0].CardID {
		t.Fatalf("miss target = %q, want visible target %q", result.Enemy.CardID, before[0].CardID)
	}
	after := drill.Enemies()
	if len(after) != 2 || after[0] != before[0] || after[1] != before[1] {
		t.Fatalf("queued answer should not clear enemies: before=%#v after=%#v", before, after)
	}
}

func TestHintReturnsRomajiWithoutCountingAsAnswer(t *testing.T) {
	drill := NewDrillFromCards(testCards()[:1], Config{}).Start()

	drill, hint := drill.Hint()
	if !hint.Available {
		t.Fatalf("Hint() returned unavailable")
	}
	if hint.Romaji != "hi" {
		t.Fatalf("hint romaji = %q, want hi", hint.Romaji)
	}
	if len(drill.Enemies()) != 1 {
		t.Fatalf("hint should not destroy enemy: %#v", drill.Enemies())
	}
	if drill.Hits() != 0 || drill.Misses() != 0 || drill.Hints() != 1 {
		t.Fatalf("counts after hint = hits %d misses %d hints %d, want 0/0/1", drill.Hits(), drill.Misses(), drill.Hints())
	}
}

func TestSpawnUsesPseudoRandomVarietyInsteadOfFixedDeckWalk(t *testing.T) {
	drill := NewDrillFromCards(testCards(), Config{Seed: 7, MaxEnemies: 2}).Start()
	first := drill.Enemies()[0].CardID

	var spawned bool
	drill, spawned = drill.Spawn()
	if !spawned {
		t.Fatalf("second spawn failed")
	}
	enemies := drill.Enemies()
	if len(enemies) != 2 {
		t.Fatalf("enemy count = %d, want 2", len(enemies))
	}
	if enemies[1].CardID == first {
		t.Fatalf("second spawn repeated first card %q despite another card being available: %#v", first, enemies)
	}
}

func TestUnplayableCardsAreNotSpawned(t *testing.T) {
	cards := append(testCards(), content.Card{
		ID:       "draft",
		Text:     "未",
		Reading:  content.Reading{RomajiHint: "mi"},
		Playable: true,
	})
	cards[0].Playable = false

	drill := NewDrillFromCards(cards, Config{})
	if drill.DeckSize() != 1 {
		t.Fatalf("deck size = %d, want only one playable card with kana", drill.DeckSize())
	}

	drill = drill.Start()
	enemies := drill.Enemies()
	if len(enemies) != 1 || enemies[0].CardID != "nihon" {
		t.Fatalf("spawned enemies = %#v, want only nihon", enemies)
	}
}

func testCards() []content.Card {
	return []content.Card{
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
	}
}
