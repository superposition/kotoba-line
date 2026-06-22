package tree

import (
	"fmt"
	"strings"

	"github.com/superposition/kotoba-line/internal/skilltree"
	"github.com/superposition/kotoba-line/internal/tui/atoms"
)

func Render(skillTree *skilltree.Tree, progress skilltree.ProgressProvider) string {
	if skillTree == nil {
		return "SKILL TREE\n(empty)"
	}

	var lines []string
	lines = append(lines, atoms.Style{Fg: atoms.Cyan, Bold: true}.Apply("SKILL TREE"))
	for _, level := range skillTree.Levels(progress) {
		title := level.Title
		if title == "" {
			title = level.ID
		}
		lines = append(lines, "- "+title)

		if len(level.Requirements) == 0 {
			lines = append(lines, "  prereq: none")
		} else {
			parts := make([]string, 0, len(level.Requirements))
			for _, requirement := range level.Requirements {
				marker := "missing"
				if requirement.Mastered {
					marker = "mastered"
				}
				parts = append(parts, fmt.Sprintf("%s=%s", requirement.ID, marker))
			}
			lines = append(lines, "  prereq: "+strings.Join(parts, ", "))
		}

		for _, card := range level.Cards {
			lines = append(lines, "  * "+renderCard(card))
		}
	}

	unlocks := skillTree.NextUnlocks(progress)
	if len(unlocks) == 0 {
		lines = append(lines, "", "Next unlocks: none")
	} else {
		lines = append(lines, "", "Next unlocks:")
		for _, card := range unlocks {
			lines = append(lines, "  * "+renderCard(card))
		}
	}

	return strings.Join(lines, "\n")
}

func renderCard(card skilltree.CardState) string {
	label := atoms.Paint(statusColor(card.Status), string(card.Status))
	return fmt.Sprintf("[%s streak=%d] %s / %s - %s", label, card.Streak, card.Text, card.Kana, card.Meaning)
}

func statusColor(status skilltree.Status) atoms.Color {
	switch status {
	case skilltree.StatusLocked:
		return atoms.DeepNavy
	case skilltree.StatusDiscovered:
		return atoms.Seafoam
	case skilltree.StatusTraining:
		return atoms.Yellow
	case skilltree.StatusMastered:
		return atoms.Coral
	default:
		return atoms.White
	}
}
