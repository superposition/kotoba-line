package boss

import (
	"testing"

	"github.com/superposition/kotoba-line/internal/content"
)

func TestNewFightKeepsBossIdentityAndStartsAtFullHP(t *testing.T) {
	fight := NewFight(testBoss())

	b := fight.Boss()
	if b.ID != "constitution-gate" || b.Title != "Tide Gate" || b.Glyph != "憲" {
		t.Fatalf("boss identity = %#v, want constitution-gate Tide Gate 憲", b)
	}
	if fight.HP() != 9 {
		t.Fatalf("HP() = %d, want 9", fight.HP())
	}
	if got := fight.Phase().ID; got != "sealed" {
		t.Fatalf("phase = %q, want sealed", got)
	}
	if fight.Cleared() {
		t.Fatalf("new fight should not start cleared")
	}
}

func TestCorrectKanaChunkDamagesBossAndAdvancesTarget(t *testing.T) {
	fight := NewFight(testBoss())

	fight, result := fight.SubmitKana("にほんこくみんは")
	if result.Status != AnswerHit {
		t.Fatalf("status = %q, want hit", result.Status)
	}
	if result.Chunk.ID != "nihon-kokumin-wa" {
		t.Fatalf("chunk = %q, want nihon-kokumin-wa", result.Chunk.ID)
	}
	if result.Damage != 3 || result.HPBefore != 9 || result.HPAfter != 6 || fight.HP() != 6 {
		t.Fatalf("damage/hp = damage %d before %d after %d fight %d, want 3/9/6/6", result.Damage, result.HPBefore, result.HPAfter, fight.HP())
	}
	if fight.Hits() != 1 || fight.Misses() != 0 {
		t.Fatalf("counts = hits %d misses %d, want 1/0", fight.Hits(), fight.Misses())
	}
	if len(result.Events) != 1 || result.Events[0].Type != EventBossDamaged {
		t.Fatalf("events = %#v, want one boss_damaged event", result.Events)
	}

	target, ok := fight.Target()
	if !ok {
		t.Fatalf("Target() returned false after non-clearing hit")
	}
	if target.ID != "shuken-ga" {
		t.Fatalf("next target = %q, want shuken-ga", target.ID)
	}
}

func TestKeyboardSyllablesDamageBossAndAdvanceTarget(t *testing.T) {
	fight := NewFight(testBoss())

	fight, result := fight.SubmitKana("nihon-kokumin-wa")
	if result.Status != AnswerHit {
		t.Fatalf("status = %q, want hit", result.Status)
	}
	if result.Chunk.ID != "nihon-kokumin-wa" {
		t.Fatalf("chunk = %q, want nihon-kokumin-wa", result.Chunk.ID)
	}
	if result.Damage != 3 || fight.HP() != 6 {
		t.Fatalf("damage/hp = %d/%d, want 3/6", result.Damage, fight.HP())
	}
}

func TestWrongKanaDoesNotDamageOrAdvanceTarget(t *testing.T) {
	fight := NewFight(testBoss())
	beforeTarget, _ := fight.Target()

	fight, result := fight.SubmitKana("wrong")
	if result.Status != AnswerMiss {
		t.Fatalf("status = %q, want miss", result.Status)
	}
	if result.Damage != 0 || result.HPBefore != 9 || result.HPAfter != 9 || fight.HP() != 9 {
		t.Fatalf("wrong answer changed hp: damage %d before %d after %d fight %d", result.Damage, result.HPBefore, result.HPAfter, fight.HP())
	}
	if fight.Hits() != 0 || fight.Misses() != 1 {
		t.Fatalf("counts = hits %d misses %d, want 0/1", fight.Hits(), fight.Misses())
	}
	if len(result.Events) != 1 || result.Events[0].Type != EventBossMissed {
		t.Fatalf("events = %#v, want one boss_missed event", result.Events)
	}

	afterTarget, ok := fight.Target()
	if !ok {
		t.Fatalf("Target() returned false after miss")
	}
	if afterTarget != beforeTarget {
		t.Fatalf("miss advanced target: before=%#v after=%#v", beforeTarget, afterTarget)
	}
}

func TestDamageCrossingThresholdChangesPhase(t *testing.T) {
	fight := NewFight(testBoss())
	fight, _ = fight.SubmitKana("にほんこくみんは")

	fight, result := fight.SubmitKana("しゅけんが")
	if result.Status != AnswerHit {
		t.Fatalf("status = %q, want hit", result.Status)
	}
	if result.PhaseBefore.ID != "sealed" || result.PhaseAfter.ID != "cracked" {
		t.Fatalf("phase before/after = %q/%q, want sealed/cracked", result.PhaseBefore.ID, result.PhaseAfter.ID)
	}
	if got := fight.Phase().ID; got != "cracked" {
		t.Fatalf("fight phase = %q, want cracked", got)
	}
	if len(result.Events) != 2 || result.Events[0].Type != EventBossDamaged || result.Events[1].Type != EventBossPhaseChanged {
		t.Fatalf("events = %#v, want damage then phase_changed", result.Events)
	}
}

func TestBossClearsAtZeroHP(t *testing.T) {
	fight := NewFight(testBoss())
	fight, _ = fight.SubmitKana("にほんこくみんは")
	fight, _ = fight.SubmitKana("しゅけんが")

	fight, result := fight.SubmitKana("けんぽう")
	if result.Status != AnswerCleared {
		t.Fatalf("status = %q, want cleared", result.Status)
	}
	if result.Damage != 3 || result.HPAfter != 0 || fight.HP() != 0 {
		t.Fatalf("clear hp = damage %d result %d fight %d, want 3/0/0", result.Damage, result.HPAfter, fight.HP())
	}
	if !fight.Cleared() {
		t.Fatalf("fight should be cleared")
	}
	if got := fight.Phase().ID; got != "cleared" {
		t.Fatalf("phase = %q, want cleared", got)
	}
	if _, ok := fight.Target(); ok {
		t.Fatalf("cleared fight should not expose a target")
	}
	if len(result.Events) != 3 {
		t.Fatalf("event count = %d, want 3: %#v", len(result.Events), result.Events)
	}
	if result.Events[0].Type != EventBossDamaged || result.Events[1].Type != EventBossPhaseChanged || result.Events[2].Type != EventBossCleared {
		t.Fatalf("events = %#v, want damage, phase_changed, cleared", result.Events)
	}
}

func TestChunksFromCardsUsesPlayablePhraseCardsOnly(t *testing.T) {
	cards := []content.Card{
		{
			ID:       "phrase",
			Text:     "日が暮れる",
			Reading:  content.Reading{Kana: "ひがくれる", RomajiHint: "hi ga kureru"},
			Meaning:  "the sun sets",
			Type:     content.CardTypePhrase,
			Playable: true,
		},
		{
			ID:       "word",
			Text:     "日本",
			Reading:  content.Reading{Kana: "にほん"},
			Type:     content.CardTypeWord,
			Playable: true,
		},
		{
			ID:       "draft",
			Text:     "未整理",
			Type:     content.CardTypePhrase,
			Playable: true,
		},
		{
			ID:       "unplayable",
			Text:     "正当に",
			Reading:  content.Reading{Kana: "せいとうに"},
			Type:     content.CardTypePhrase,
			Playable: false,
		},
	}

	chunks := ChunksFromCards(cards)
	if len(chunks) != 1 {
		t.Fatalf("chunk count = %d, want 1: %#v", len(chunks), chunks)
	}
	if chunks[0].ID != "phrase" || chunks[0].Kana != "ひがくれる" || chunks[0].Damage != 1 {
		t.Fatalf("chunk = %#v, want playable phrase with default damage", chunks[0])
	}
}

func testBoss() Boss {
	return Boss{
		ID:    "constitution-gate",
		Title: "Tide Gate",
		Glyph: "憲",
		HP:    9,
		Phases: []Phase{
			{ID: "sealed", Title: "Sealed", Glyph: "憲", StartsAtHP: 9},
			{ID: "cracked", Title: "Cracked", Glyph: "憲", StartsAtHP: 3},
			{ID: "cleared", Title: "Cleared", Glyph: "憲", StartsAtHP: 0},
		},
		Chunks: []Chunk{
			{ID: "nihon-kokumin-wa", Text: "日本国民は", Kana: "にほんこくみんは", RomajiHint: "nihon-kokumin wa", Meaning: "the Japanese people", Damage: 3},
			{ID: "shuken-ga", Text: "主権が", Kana: "しゅけんが", RomajiHint: "shuken ga", Meaning: "sovereign power", Damage: 3},
			{ID: "kenpou", Text: "憲法", Kana: "けんぽう", RomajiHint: "kenpou", Meaning: "constitution", Damage: 3},
		},
	}
}
