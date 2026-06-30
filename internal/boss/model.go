package boss

import (
	"strings"

	"github.com/superposition/kotoba-line/internal/content"
	"github.com/superposition/kotoba-line/internal/kana"
)

type Boss struct {
	ID     string
	Title  string
	Glyph  string
	HP     int
	Phases []Phase
	Chunks []Chunk
}

type Phase struct {
	ID         string
	Title      string
	Glyph      string
	StartsAtHP int
}

type Chunk struct {
	ID         string
	Text       string
	Kana       string
	RomajiHint string
	Meaning    string
	Damage     int
}

type Fight struct {
	boss        Boss
	hp          int
	targetIndex int
	hits        int
	misses      int
	cleared     bool
}

type AnswerStatus string

const (
	AnswerEmpty   AnswerStatus = "empty"
	AnswerHit     AnswerStatus = "hit"
	AnswerMiss    AnswerStatus = "miss"
	AnswerCleared AnswerStatus = "cleared"
)

type EventType string

const (
	EventBossDamaged      EventType = "boss_damaged"
	EventBossMissed       EventType = "boss_missed"
	EventBossPhaseChanged EventType = "boss_phase_changed"
	EventBossCleared      EventType = "boss_cleared"
)

type Event struct {
	Type    EventType
	BossID  string
	ChunkID string
	PhaseID string
	Input   string
	Damage  int
	HP      int
}

type AnswerResult struct {
	Status      AnswerStatus
	Input       string
	Chunk       Chunk
	Damage      int
	HPBefore    int
	HPAfter     int
	PhaseBefore Phase
	PhaseAfter  Phase
	Events      []Event
}

func NewFight(b Boss) Fight {
	b = normalizeBoss(b)
	return Fight{
		boss:    b,
		hp:      b.HP,
		cleared: b.HP == 0,
	}
}

func ChunksFromCards(cards []content.Card) []Chunk {
	chunks := make([]Chunk, 0, len(cards))
	for _, card := range cards {
		if card.Type != content.CardTypePhrase || !card.Playable {
			continue
		}
		kana := strings.TrimSpace(card.Reading.Kana)
		if kana == "" {
			continue
		}
		chunks = append(chunks, Chunk{
			ID:         card.ID,
			Text:       card.Text,
			Kana:       kana,
			RomajiHint: card.Reading.RomajiHint,
			Meaning:    card.Meaning,
			Damage:     1,
		})
	}
	return chunks
}

func (f Fight) SubmitKana(input string) (Fight, AnswerResult) {
	answer := strings.TrimSpace(input)
	beforeHP := f.hp
	beforePhase := f.Phase()
	result := AnswerResult{
		Status:      AnswerEmpty,
		Input:       answer,
		HPBefore:    beforeHP,
		HPAfter:     beforeHP,
		PhaseBefore: beforePhase,
		PhaseAfter:  beforePhase,
	}
	if answer == "" {
		return f, result
	}

	chunk, ok := f.Target()
	if !ok || f.cleared {
		result.Status = AnswerMiss
		result.Events = []Event{{
			Type:   EventBossMissed,
			BossID: f.boss.ID,
			Input:  answer,
			HP:     f.hp,
		}}
		return f, result
	}

	result.Chunk = chunk
	if !kana.MatchesAnswer(chunk.Kana, chunk.RomajiHint, answer) {
		f.misses++
		result.Status = AnswerMiss
		result.Events = []Event{{
			Type:    EventBossMissed,
			BossID:  f.boss.ID,
			ChunkID: chunk.ID,
			PhaseID: beforePhase.ID,
			Input:   answer,
			HP:      f.hp,
		}}
		return f, result
	}

	damage := chunkDamage(chunk)
	f.hp -= damage
	if f.hp < 0 {
		f.hp = 0
	}
	f.hits++
	f.targetIndex++
	f.cleared = f.hp == 0

	afterPhase := f.Phase()
	result.Status = AnswerHit
	if f.cleared {
		result.Status = AnswerCleared
	}
	result.Damage = beforeHP - f.hp
	result.HPAfter = f.hp
	result.PhaseAfter = afterPhase
	result.Events = []Event{{
		Type:    EventBossDamaged,
		BossID:  f.boss.ID,
		ChunkID: chunk.ID,
		PhaseID: afterPhase.ID,
		Input:   answer,
		Damage:  result.Damage,
		HP:      f.hp,
	}}
	if beforePhase.ID != afterPhase.ID {
		result.Events = append(result.Events, Event{
			Type:    EventBossPhaseChanged,
			BossID:  f.boss.ID,
			PhaseID: afterPhase.ID,
			HP:      f.hp,
		})
	}
	if f.cleared {
		result.Events = append(result.Events, Event{
			Type:   EventBossCleared,
			BossID: f.boss.ID,
			HP:     f.hp,
		})
	}

	return f, result
}

func (f Fight) Target() (Chunk, bool) {
	if len(f.boss.Chunks) == 0 || f.cleared {
		return Chunk{}, false
	}
	return f.boss.Chunks[f.targetIndex%len(f.boss.Chunks)], true
}

func (f Fight) Phase() Phase {
	phases := f.boss.Phases
	if len(phases) == 0 {
		return Phase{}
	}

	current := phases[0]
	for _, phase := range phases {
		if f.hp <= phase.StartsAtHP {
			current = phase
		}
	}
	return current
}

func (f Fight) Boss() Boss {
	return copyBoss(f.boss)
}

func (f Fight) HP() int {
	return f.hp
}

func (f Fight) Cleared() bool {
	return f.cleared
}

func (f Fight) Hits() int {
	return f.hits
}

func (f Fight) Misses() int {
	return f.misses
}

func normalizeBoss(b Boss) Boss {
	if b.HP < 0 {
		b.HP = 0
	}

	b.Chunks = copyChunks(b.Chunks)
	for i := range b.Chunks {
		b.Chunks[i].Kana = strings.TrimSpace(b.Chunks[i].Kana)
		if b.Chunks[i].Damage <= 0 {
			b.Chunks[i].Damage = 1
		}
	}

	b.Phases = copyPhases(b.Phases)
	if len(b.Phases) == 0 {
		b.Phases = []Phase{{
			ID:         "main",
			Title:      b.Title,
			Glyph:      b.Glyph,
			StartsAtHP: b.HP,
		}}
	}
	for i := range b.Phases {
		if b.Phases[i].Glyph == "" {
			b.Phases[i].Glyph = b.Glyph
		}
	}

	return b
}

func copyBoss(b Boss) Boss {
	b.Phases = copyPhases(b.Phases)
	b.Chunks = copyChunks(b.Chunks)
	return b
}

func copyPhases(phases []Phase) []Phase {
	copied := make([]Phase, len(phases))
	copy(copied, phases)
	return copied
}

func copyChunks(chunks []Chunk) []Chunk {
	copied := make([]Chunk, len(chunks))
	copy(copied, chunks)
	return copied
}

func chunkDamage(chunk Chunk) int {
	if chunk.Damage <= 0 {
		return 1
	}
	return chunk.Damage
}
