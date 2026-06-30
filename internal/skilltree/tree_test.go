package skilltree

import (
	"reflect"
	"testing"

	"github.com/superposition/kotoba-line/internal/content"
)

func TestHitMastersCard(t *testing.T) {
	progress := Progress{Status: StatusDiscovered}

	progress = Hit(progress)
	if progress.Status != StatusMastered || progress.Streak != MasteryStreak {
		t.Fatalf("hit = %#v, want mastered streak %d", progress, MasteryStreak)
	}
}

func TestMissResetsTrainingStreak(t *testing.T) {
	progress := Miss(Progress{Status: StatusTraining, Streak: 2})

	if progress.Status != StatusDiscovered || progress.Streak != 0 {
		t.Fatalf("miss = %#v, want discovered streak 0", progress)
	}
}

func TestDependencyUnlocksAfterPrerequisitesMastered(t *testing.T) {
	tree := mustTree(t)

	progress := MapProgress{}
	assertStatuses(t, tree, progress, map[string]Status{
		"root-a":   StatusDiscovered,
		"root-b":   StatusDiscovered,
		"branch-c": StatusLocked,
		"branch-d": StatusLocked,
	})
	assertNextUnlocks(t, tree, progress, []string{"root-a", "root-b"})

	progress["root-a"] = Progress{Status: StatusMastered, Streak: MasteryStreak}
	assertStatuses(t, tree, progress, map[string]Status{
		"root-a":   StatusMastered,
		"root-b":   StatusDiscovered,
		"branch-c": StatusLocked,
		"branch-d": StatusLocked,
	})

	progress["root-b"] = Progress{Status: StatusMastered, Streak: MasteryStreak}
	assertStatuses(t, tree, progress, map[string]Status{
		"root-a":   StatusMastered,
		"root-b":   StatusMastered,
		"branch-c": StatusDiscovered,
		"branch-d": StatusDiscovered,
	})
	assertNextUnlocks(t, tree, progress, []string{"branch-c", "branch-d"})
}

func TestStatusCarriesTrainingStreak(t *testing.T) {
	tree := mustTree(t)
	progress := MapProgress{
		"root-a":   {Status: StatusMastered, Streak: MasteryStreak},
		"root-b":   {Status: StatusMastered, Streak: MasteryStreak},
		"branch-c": {Status: StatusTraining, Streak: 2},
	}

	state := tree.Status("branch-c", progress)
	if state.Status != StatusTraining || state.Streak != 2 {
		t.Fatalf("branch-c status = %#v, want training streak 2", state)
	}
	if !reflect.DeepEqual(state.RequiredCardIDs, []string{"root-a", "root-b"}) {
		t.Fatalf("branch-c requirements = %#v, want root-a/root-b", state.RequiredCardIDs)
	}
}

func TestUnknownDependencyFailsBuild(t *testing.T) {
	_, err := New(&content.Library{
		Cards: []content.Card{card("root-a", "日", "ひ")},
		Levels: []content.Level{{
			ID:              "broken",
			Title:           "Broken",
			RequiredCardIDs: []string{"missing"},
			CardIDs:         []string{"root-a"},
		}},
	})
	if err == nil {
		t.Fatalf("New() error = nil, want unknown dependency error")
	}
}

func assertStatuses(t *testing.T, tree *Tree, progress ProgressProvider, want map[string]Status) {
	t.Helper()
	for cardID, status := range want {
		got := tree.Status(cardID, progress)
		if got.Status != status {
			t.Fatalf("%s status = %s, want %s", cardID, got.Status, status)
		}
	}
}

func assertNextUnlocks(t *testing.T, tree *Tree, progress ProgressProvider, want []string) {
	t.Helper()
	var got []string
	for _, card := range tree.NextUnlocks(progress) {
		got = append(got, card.ID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NextUnlocks() = %#v, want %#v", got, want)
	}
}

func mustTree(t *testing.T) *Tree {
	t.Helper()
	tree, err := New(&content.Library{
		Cards: []content.Card{
			card("root-a", "日", "ひ"),
			card("root-b", "本", "ほん"),
			card("branch-c", "日本", "にほん"),
			card("branch-d", "本日", "ほんじつ"),
		},
		Levels: []content.Level{
			{
				ID:      "root",
				Title:   "Root",
				CardIDs: []string{"root-a", "root-b"},
			},
			{
				ID:              "branch",
				Title:           "Branch",
				RequiredCardIDs: []string{"root-a", "root-b"},
				CardIDs:         []string{"branch-c", "branch-d"},
			},
		},
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return tree
}

func card(id, text, kana string) content.Card {
	return content.Card{
		ID:      id,
		Text:    text,
		Kanji:   text,
		Reading: content.Reading{Kana: kana},
		Meaning: "meaning " + id,
		Type:    content.CardTypeWord,
	}
}
