package state

type EventType string

const (
	EventEnemyHit      EventType = "enemy_hit"
	EventEnemyMissed   EventType = "enemy_missed"
	EventHintRevealed  EventType = "hint_revealed"
	EventCardMastered  EventType = "card_mastered"
	EventLevelUnlocked EventType = "level_unlocked"
	EventBossIntro     EventType = "boss_intro"
	EventBossDamaged   EventType = "boss_damaged"
	EventBossCleared   EventType = "boss_cleared"

	MasteryCleanHitStreak = 3
)

type Event struct {
	Type    EventType `json:"type"`
	CardID  string    `json:"card_id,omitempty"`
	LevelID string    `json:"level_id,omitempty"`
	BossID  string    `json:"boss_id,omitempty"`
	HintID  string    `json:"hint_id,omitempty"`
	Clean   *bool     `json:"clean,omitempty"`
}

func EnemyHit(cardID string) Event {
	return EnemyHitWithClean(cardID, true)
}

func EnemyHitWithClean(cardID string, clean bool) Event {
	return Event{
		Type:   EventEnemyHit,
		CardID: cardID,
		Clean:  &clean,
	}
}

func EnemyMissed(cardID string) Event {
	return Event{
		Type:   EventEnemyMissed,
		CardID: cardID,
	}
}

func HintRevealed(cardID, hintID string) Event {
	return Event{
		Type:   EventHintRevealed,
		CardID: cardID,
		HintID: hintID,
	}
}

func CardMastered(cardID string) Event {
	return Event{
		Type:   EventCardMastered,
		CardID: cardID,
	}
}

func LevelUnlocked(levelID string) Event {
	return Event{
		Type:    EventLevelUnlocked,
		LevelID: levelID,
	}
}

func BossIntro(bossID string) Event {
	return Event{
		Type:   EventBossIntro,
		BossID: bossID,
	}
}

func BossDamaged(bossID, cardID string) Event {
	return Event{
		Type:   EventBossDamaged,
		BossID: bossID,
		CardID: cardID,
	}
}

func BossCleared(bossID string) Event {
	return Event{
		Type:   EventBossCleared,
		BossID: bossID,
	}
}

func (e Event) cleanHit() bool {
	if e.Clean == nil {
		return true
	}
	return *e.Clean
}
