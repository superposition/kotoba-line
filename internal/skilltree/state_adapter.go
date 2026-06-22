package skilltree

import statestore "github.com/superposition/kotoba-line/internal/state"

type StateProgress struct {
	Progress statestore.Progress
}

func (s StateProgress) CardProgress(cardID string) Progress {
	card := s.Progress.Cards[cardID]
	switch {
	case card.Mastered:
		return Progress{Status: StatusMastered, Streak: card.Streak}
	case card.Streak > 0:
		return Progress{Status: StatusTraining, Streak: card.Streak}
	default:
		return Progress{Status: StatusDiscovered}
	}
}
