package game

import (
	"strings"

	"github.com/superposition/kotoba-line/internal/content"
)

const (
	defaultSpawnEvery = 3
	defaultMaxEnemies = 4
)

type Config struct {
	SpawnEvery int
	MaxEnemies int
}

type Drill struct {
	deck       []content.Card
	deckIndex  int
	enemies    []Enemy
	nextID     int
	tick       int
	spawnEvery int
	maxEnemies int
	hits       int
	misses     int
	hints      int
}

type Enemy struct {
	ID         int
	CardID     string
	Text       string
	Kana       string
	RomajiHint string
	Meaning    string
	Type       content.CardType
	Row        int
	SpawnedAt  int
}

type AnswerStatus string

const (
	AnswerEmpty AnswerStatus = "empty"
	AnswerHit   AnswerStatus = "hit"
	AnswerMiss  AnswerStatus = "miss"
)

type AnswerResult struct {
	Status AnswerStatus
	Input  string
	Enemy  Enemy
}

type HintResult struct {
	Available bool
	Enemy     Enemy
	Romaji    string
}

func NewDrill(library *content.Library, cfg Config) Drill {
	if library == nil {
		return NewDrillFromCards(nil, cfg)
	}
	return NewDrillFromCards(library.Cards, cfg)
}

func NewDrillFromCards(cards []content.Card, cfg Config) Drill {
	spawnEvery := cfg.SpawnEvery
	if spawnEvery <= 0 {
		spawnEvery = defaultSpawnEvery
	}
	maxEnemies := cfg.MaxEnemies
	if maxEnemies <= 0 {
		maxEnemies = defaultMaxEnemies
	}

	deck := make([]content.Card, 0, len(cards))
	for _, card := range cards {
		if card.Playable && strings.TrimSpace(card.Reading.Kana) != "" {
			deck = append(deck, card)
		}
	}

	return Drill{
		deck:       deck,
		nextID:     1,
		spawnEvery: spawnEvery,
		maxEnemies: maxEnemies,
	}
}

func (d Drill) Start() Drill {
	d, _ = d.Spawn()
	return d
}

func (d Drill) Tick() Drill {
	d.tick++
	for i := range d.enemies {
		d.enemies[i].Row++
	}
	if d.tick%d.spawnEvery == 0 {
		d, _ = d.Spawn()
	}
	return d
}

func (d Drill) Spawn() (Drill, bool) {
	if len(d.deck) == 0 || len(d.enemies) >= d.maxEnemies {
		return d, false
	}

	card := d.deck[d.deckIndex%len(d.deck)]
	enemy := Enemy{
		ID:         d.nextID,
		CardID:     card.ID,
		Text:       card.Text,
		Kana:       card.Reading.Kana,
		RomajiHint: card.Reading.RomajiHint,
		Meaning:    card.Meaning,
		Type:       card.Type,
		SpawnedAt:  d.tick,
	}

	d.nextID++
	d.deckIndex++
	d.enemies = append(d.enemies, enemy)
	return d, true
}

func (d Drill) SubmitKana(input string) (Drill, AnswerResult) {
	answer := strings.TrimSpace(input)
	result := AnswerResult{Status: AnswerEmpty, Input: answer}
	if answer == "" {
		return d, result
	}

	for i, enemy := range d.enemies {
		if enemy.Kana != answer {
			continue
		}

		d.enemies = append(d.enemies[:i:i], d.enemies[i+1:]...)
		d.hits++
		result.Status = AnswerHit
		result.Enemy = enemy
		return d, result
	}

	d.misses++
	if target, ok := d.Target(); ok {
		result.Enemy = target
	}
	result.Status = AnswerMiss
	return d, result
}

func (d Drill) Hint() (Drill, HintResult) {
	enemy, ok := d.Target()
	if !ok {
		return d, HintResult{}
	}
	d.hints++
	return d, HintResult{
		Available: true,
		Enemy:     enemy,
		Romaji:    enemy.RomajiHint,
	}
}

func (d Drill) Target() (Enemy, bool) {
	if len(d.enemies) == 0 {
		return Enemy{}, false
	}
	return d.enemies[0], true
}

func (d Drill) Enemies() []Enemy {
	enemies := make([]Enemy, len(d.enemies))
	copy(enemies, d.enemies)
	return enemies
}

func (d Drill) DeckSize() int {
	return len(d.deck)
}

func (d Drill) TickCount() int {
	return d.tick
}

func (d Drill) Hits() int {
	return d.hits
}

func (d Drill) Misses() int {
	return d.misses
}

func (d Drill) Hints() int {
	return d.hints
}
