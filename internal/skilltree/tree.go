package skilltree

import (
	"fmt"
	"sort"

	"github.com/superposition/kotoba-line/internal/content"
)

type Tree struct {
	levels    []Level
	cards     map[string]Card
	cardOrder []string
}

type Level struct {
	ID              string
	Title           string
	Description     string
	RequiredCardIDs []string
	CardIDs         []string
}

type Card struct {
	Content         content.Card
	RequiredCardIDs []string
	LevelIDs        []string
}

type LevelState struct {
	ID           string
	Title        string
	Description  string
	Requirements []RequirementState
	Cards        []CardState
}

type RequirementState struct {
	ID       string
	Status   Status
	Mastered bool
}

type CardState struct {
	ID              string
	Text            string
	Kana            string
	Meaning         string
	Type            content.CardType
	RequiredCardIDs []string
	Status          Status
	Streak          int
	NextUnlock      bool
}

func New(library *content.Library) (*Tree, error) {
	if library == nil {
		return nil, fmt.Errorf("skilltree: nil content library")
	}

	cards := make(map[string]Card, len(library.Cards))
	cardOrder := make([]string, 0, len(library.Cards))
	for _, card := range library.Cards {
		if _, exists := cards[card.ID]; exists {
			return nil, fmt.Errorf("skilltree: duplicate card %q", card.ID)
		}
		cards[card.ID] = Card{Content: card}
		cardOrder = append(cardOrder, card.ID)
	}

	levels := make([]Level, 0, len(library.Levels))
	seenInLevel := make(map[string]bool, len(library.Cards))
	for _, level := range library.Levels {
		if level.ID == "" {
			return nil, fmt.Errorf("skilltree: level id is required")
		}
		for _, cardID := range level.RequiredCardIDs {
			if _, exists := cards[cardID]; !exists {
				return nil, fmt.Errorf("skilltree: level %q requires unknown card %q", level.ID, cardID)
			}
		}
		for _, cardID := range level.CardIDs {
			card, exists := cards[cardID]
			if !exists {
				return nil, fmt.Errorf("skilltree: level %q contains unknown card %q", level.ID, cardID)
			}

			card.RequiredCardIDs = mergeIDs(card.RequiredCardIDs, level.RequiredCardIDs)
			card.LevelIDs = append(card.LevelIDs, level.ID)
			cards[cardID] = card
			seenInLevel[cardID] = true
		}

		levels = append(levels, Level{
			ID:              level.ID,
			Title:           level.Title,
			Description:     level.Description,
			RequiredCardIDs: append([]string(nil), level.RequiredCardIDs...),
			CardIDs:         append([]string(nil), level.CardIDs...),
		})
	}

	var ungrouped []string
	for _, cardID := range cardOrder {
		if !seenInLevel[cardID] {
			ungrouped = append(ungrouped, cardID)
		}
	}
	if len(ungrouped) > 0 {
		levels = append(levels, Level{
			ID:      "ungrouped",
			Title:   "Ungrouped Cards",
			CardIDs: ungrouped,
		})
	}

	return &Tree{levels: levels, cards: cards, cardOrder: cardOrder}, nil
}

func (t *Tree) Levels(progress ProgressProvider) []LevelState {
	if t == nil {
		return nil
	}

	progress = ensureProgress(progress)
	next := indexNextUnlocks(t.NextUnlocks(progress))
	levels := make([]LevelState, 0, len(t.levels))
	for _, level := range t.levels {
		state := LevelState{
			ID:          level.ID,
			Title:       level.Title,
			Description: level.Description,
		}
		for _, cardID := range level.RequiredCardIDs {
			status := t.Status(cardID, progress)
			state.Requirements = append(state.Requirements, RequirementState{
				ID:       cardID,
				Status:   status.Status,
				Mastered: status.Status == StatusMastered,
			})
		}
		for _, cardID := range level.CardIDs {
			cardState := t.Status(cardID, progress)
			cardState.NextUnlock = next[cardID]
			state.Cards = append(state.Cards, cardState)
		}
		levels = append(levels, state)
	}
	return levels
}

func (t *Tree) Status(cardID string, progress ProgressProvider) CardState {
	if t == nil {
		return CardState{ID: cardID, Status: StatusLocked}
	}

	card, exists := t.cards[cardID]
	if !exists {
		return CardState{ID: cardID, Status: StatusLocked}
	}

	progress = ensureProgress(progress)
	stored := normalize(progress.CardProgress(cardID))
	if stored.Status != StatusMastered && !t.prerequisitesMastered(card.RequiredCardIDs, progress) {
		stored.Status = StatusLocked
		stored.Streak = 0
	}

	return CardState{
		ID:              card.Content.ID,
		Text:            card.Content.Text,
		Kana:            card.Content.Reading.Kana,
		Meaning:         card.Content.Meaning,
		Type:            card.Content.Type,
		RequiredCardIDs: append([]string(nil), card.RequiredCardIDs...),
		Status:          stored.Status,
		Streak:          stored.Streak,
	}
}

func (t *Tree) NextUnlocks(progress ProgressProvider) []CardState {
	if t == nil {
		return nil
	}

	progress = ensureProgress(progress)
	unlocks := make([]CardState, 0)
	for _, cardID := range t.cardOrder {
		state := t.Status(cardID, progress)
		if state.Status == StatusDiscovered {
			state.NextUnlock = true
			unlocks = append(unlocks, state)
		}
	}
	return unlocks
}

func (t *Tree) prerequisitesMastered(required []string, progress ProgressProvider) bool {
	for _, cardID := range required {
		if normalize(progress.CardProgress(cardID)).Status != StatusMastered {
			return false
		}
	}
	return true
}

func normalize(progress Progress) Progress {
	if progress.Streak < 0 {
		progress.Streak = 0
	}
	switch progress.Status {
	case StatusLocked, StatusDiscovered, StatusTraining, StatusMastered:
	case "":
		if progress.Streak > 0 {
			progress.Status = StatusTraining
		} else {
			progress.Status = StatusDiscovered
		}
	default:
		progress.Status = StatusDiscovered
		progress.Streak = 0
	}
	if progress.Status == StatusTraining && progress.Streak == 0 {
		progress.Status = StatusDiscovered
	}
	return progress
}

func mergeIDs(existing, incoming []string) []string {
	if len(incoming) == 0 {
		return existing
	}

	seen := make(map[string]bool, len(existing)+len(incoming))
	for _, id := range existing {
		seen[id] = true
	}
	for _, id := range incoming {
		if !seen[id] {
			existing = append(existing, id)
			seen[id] = true
		}
	}
	sort.Strings(existing)
	return existing
}

func ensureProgress(progress ProgressProvider) ProgressProvider {
	if progress == nil {
		return MapProgress{}
	}
	return progress
}

func indexNextUnlocks(unlocks []CardState) map[string]bool {
	index := make(map[string]bool, len(unlocks))
	for _, unlock := range unlocks {
		index[unlock.ID] = true
	}
	return index
}
