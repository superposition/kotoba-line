package tree

import (
	"strings"
	"testing"

	"github.com/superposition/kotoba-line/internal/content"
	"github.com/superposition/kotoba-line/internal/skilltree"
	"github.com/superposition/kotoba-line/internal/tui/atoms"
)

func TestRenderShowsStatusStreakPrerequisitesAndNextUnlocks(t *testing.T) {
	tree := mustSkillTree(t)
	progress := skilltree.MapProgress{
		"root-a": {Status: skilltree.StatusMastered, Streak: skilltree.MasteryStreak},
		"root-b": {Status: skilltree.StatusMastered, Streak: skilltree.MasteryStreak},
	}

	out := atoms.StripANSI(Render(tree, progress))
	for _, want := range []string{
		"SKILL TREE",
		"- Root",
		"prereq: none",
		"[mastered streak=3] 日 / ひ",
		"- Branch",
		"prereq: root-a=mastered, root-b=mastered",
		"[discovered streak=0] 日本 / にほん",
		"Next unlocks:",
		"branch-c",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Render() missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderShowsLockedWhenPrerequisitesAreMissing(t *testing.T) {
	tree := mustSkillTree(t)
	progress := skilltree.MapProgress{
		"root-a": {Status: skilltree.StatusMastered, Streak: skilltree.MasteryStreak},
		"root-b": {Status: skilltree.StatusTraining, Streak: 2},
	}

	out := atoms.StripANSI(Render(tree, progress))
	for _, want := range []string{
		"prereq: root-a=mastered, root-b=missing",
		"[training streak=2] 本 / ほん",
		"[locked streak=0] 日本 / にほん",
		"Next unlocks: none",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Render() missing %q in:\n%s", want, out)
		}
	}
}

func mustSkillTree(t *testing.T) *skilltree.Tree {
	t.Helper()
	tree, err := skilltree.New(&content.Library{
		Cards: []content.Card{
			card("root-a", "日", "ひ"),
			card("root-b", "本", "ほん"),
			card("branch-c", "日本", "にほん"),
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
				CardIDs:         []string{"branch-c"},
			},
		},
	})
	if err != nil {
		t.Fatalf("skilltree.New(): %v", err)
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
