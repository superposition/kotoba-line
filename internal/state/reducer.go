package state

import "fmt"

type Progress struct {
	Cards          map[string]CardProgress `json:"cards"`
	UnlockedLevels map[string]bool         `json:"unlocked_levels"`
	Points         int                     `json:"points"`
}

type CardProgress struct {
	Streak        int             `json:"streak"`
	Mastered      bool            `json:"mastered"`
	HintsUsed     int             `json:"hints_used"`
	RevealedHints map[string]bool `json:"revealed_hints,omitempty"`
}

func NewProgress() Progress {
	return Progress{
		Cards:          map[string]CardProgress{},
		UnlockedLevels: map[string]bool{},
	}
}

func ReplayEvents(events []Event) (Progress, error) {
	progress := NewProgress()
	for index, event := range events {
		if err := progress.Apply(event); err != nil {
			return Progress{}, fmt.Errorf("apply event %d: %w", index, err)
		}
	}
	return progress, nil
}

func (p *Progress) Apply(event Event) error {
	if err := ValidateEvent(event); err != nil {
		return err
	}
	p.ensureMaps()

	switch event.Type {
	case EventEnemyHit:
		card := p.Cards[event.CardID]
		card.Streak++
		if card.Streak >= MasteryCleanHitStreak {
			card.Mastered = true
		}
		p.Cards[event.CardID] = card
	case EventEnemyMissed:
		card := p.Cards[event.CardID]
		card.Streak = 0
		p.Cards[event.CardID] = card
	case EventHintRevealed:
		card := p.Cards[event.CardID]
		card.HintsUsed++
		if event.HintID != "" {
			if card.RevealedHints == nil {
				card.RevealedHints = map[string]bool{}
			}
			card.RevealedHints[event.HintID] = true
		}
		p.Cards[event.CardID] = card
	case EventCardMastered:
		card := p.Cards[event.CardID]
		card.Mastered = true
		p.Cards[event.CardID] = card
	case EventLevelUnlocked:
		p.UnlockedLevels[event.LevelID] = true
	case EventPoints:
		p.Points = clampPoints(p.Points + event.Points)
	case EventBossIntro, EventBossDamaged, EventBossCleared:
		// Boss events are replayed by transition views; progression is tracked by
		// the underlying card hit and level unlock events.
	}

	return nil
}

func (p *Progress) ensureMaps() {
	if p.Cards == nil {
		p.Cards = map[string]CardProgress{}
	}
	if p.UnlockedLevels == nil {
		p.UnlockedLevels = map[string]bool{}
	}
}

func clampPoints(points int) int {
	if points < 0 {
		return 0
	}
	return points
}
