package transition

import (
	"strings"

	statestore "github.com/superposition/kotoba-line/internal/state"
)

// SceneID identifies one replayable transition scene.
type SceneID string

const (
	SceneCardMastery    SceneID = "card_mastery"
	SceneStationArrival SceneID = "station_arrival"
	SceneBossIntro      SceneID = "boss_intro"
	SceneBossCrack      SceneID = "boss_crack"
	SceneLevelClear     SceneID = "level_clear"
)

// EventKind is the small replay surface that drives the transition queue.
type EventKind string

const (
	EventCardMastered   EventKind = "card_mastered"
	EventStationArrival EventKind = "station_arrival"
	EventBossIntro      EventKind = "boss_intro"
	EventBossCrack      EventKind = "boss_crack"
	EventLevelClear     EventKind = "level_clear"
)

// Event is an append-log-friendly transition trigger.
type Event struct {
	Kind    EventKind
	Subject string
}

// Frame is one deterministic frame in a transition scene.
type Frame struct {
	ID    string
	Lines []string
}

// Definition describes a replayable transition scene and its timing metadata.
type Definition struct {
	ID         SceneID
	Title      string
	DurationMS int
	FrameMS    int
	Frames     []Frame
}

// QueuedScene is a concrete scene instance with the subject from the event log.
type QueuedScene struct {
	Definition Definition
	Subject    string
}

var sceneDefinitions = []Definition{
	{
		ID:         SceneCardMastery,
		Title:      "CARD MASTERY",
		DurationMS: 480,
		FrameMS:    120,
		Frames: []Frame{
			{ID: "wake", Lines: []string{"~~~ combo tide ~~~", "CARD MASTERED", "{subject}"}},
			{ID: "glow", Lines: []string{"*** coral flash ***", "glyph locks bright", "{subject}"}},
			{ID: "stamp", Lines: []string{"[MASTER] [MASTER]", "seafoam seal set", "{subject}"}},
			{ID: "fade", Lines: []string{"...wave fade...", "next card rises", "{subject}"}},
		},
	},
	{
		ID:         SceneStationArrival,
		Title:      "STATION ARRIVAL",
		DurationMS: 700,
		FrameMS:    140,
		Frames: []Frame{
			{ID: "horizon", Lines: []string{"~~~~~ horizon ~~~~~", "station lights blink", "{subject}"}},
			{ID: "signal", Lines: []string{"< < < SIGNAL > > >", "tide gate opens", "{subject}"}},
			{ID: "dock", Lines: []string{"|== KOTOBA LINE ==|", "dock bell rings", "{subject}"}},
			{ID: "map", Lines: []string{"o--o--O--o--o", "route dots pulse", "{subject}"}},
			{ID: "ready", Lines: []string{"READY", "next drill board", "{subject}"}},
		},
	},
	{
		ID:         SceneBossIntro,
		Title:      "BOSS INTRO",
		DurationMS: 800,
		FrameMS:    160,
		Frames: []Frame{
			{ID: "warning", Lines: []string{"!!! BOSS WAVE !!!", "deep navy alarm", "{subject}"}},
			{ID: "shadow", Lines: []string{"######", "kanji shadow rises", "{subject}"}},
			{ID: "emblem", Lines: []string{"<< BIG EMBLEM >>", "phrase shield online", "{subject}"}},
			{ID: "tide", Lines: []string{"~~~~~", "ocean floor shakes", "{subject}"}},
			{ID: "fight", Lines: []string{"FIGHT", "reading salvo armed", "{subject}"}},
		},
	},
	{
		ID:         SceneBossCrack,
		Title:      "BOSS CRACK",
		DurationMS: 440,
		FrameMS:    110,
		Frames: []Frame{
			{ID: "hit", Lines: []string{"HIT!", "coral damage flash", "{subject}"}},
			{ID: "fracture", Lines: []string{"//// CRACK ////", "boss shell splits", "{subject}"}},
			{ID: "spray", Lines: []string{"* * *", "seafoam burst", "{subject}"}},
			{ID: "settle", Lines: []string{"HP drops", "emblem still floats", "{subject}"}},
		},
	},
	{
		ID:         SceneLevelClear,
		Title:      "LEVEL CLEAR",
		DurationMS: 750,
		FrameMS:    150,
		Frames: []Frame{
			{ID: "break", Lines: []string{"BOSS DOWN", "gate chain breaks", "{subject}"}},
			{ID: "route", Lines: []string{"O==O==O==O", "ocean route complete", "{subject}"}},
			{ID: "banner", Lines: []string{"LEVEL CLEAR", "yellow signal high", "{subject}"}},
			{ID: "unlock", Lines: []string{"NEW STATION", "next tide unlocked", "{subject}"}},
			{ID: "fade", Lines: []string{"...calm water...", "route memory glows", "{subject}"}},
		},
	},
}

// Definitions returns fresh copies of every built-in scene definition.
func Definitions() []Definition {
	out := make([]Definition, 0, len(sceneDefinitions))
	for _, definition := range sceneDefinitions {
		out = append(out, cloneDefinition(definition))
	}
	return out
}

// DefinitionFor returns a fresh copy of the scene definition for id.
func DefinitionFor(id SceneID) (Definition, bool) {
	for _, definition := range sceneDefinitions {
		if definition.ID == id {
			return cloneDefinition(definition), true
		}
	}
	return Definition{}, false
}

// SceneFor creates a replayable scene instance for id.
func SceneFor(id SceneID, subject string) (QueuedScene, bool) {
	definition, ok := DefinitionFor(id)
	if !ok {
		return QueuedScene{}, false
	}
	return QueuedScene{
		Definition: definition,
		Subject:    strings.TrimSpace(subject),
	}, true
}

// Frames returns the scene frames with the subject token resolved.
func (s QueuedScene) Frames() []Frame {
	subject := s.Subject
	if subject == "" {
		subject = "next tide"
	}

	out := make([]Frame, 0, len(s.Definition.Frames))
	for _, frame := range s.Definition.Frames {
		lines := make([]string, 0, len(frame.Lines))
		for _, line := range frame.Lines {
			lines = append(lines, strings.ReplaceAll(line, "{subject}", subject))
		}
		out = append(out, Frame{
			ID:    frame.ID,
			Lines: lines,
		})
	}
	return out
}

// Queue turns transition events into queued scenes while preserving event order.
func Queue(events []Event) []QueuedScene {
	scenes := make([]QueuedScene, 0, len(events))
	for _, event := range events {
		if scene, ok := SceneFor(event.sceneID(), event.Subject); ok {
			scenes = append(scenes, scene)
		}
	}
	return scenes
}

// QueueFromStateEvents replays the existing durable state events into scenes.
func QueueFromStateEvents(events []statestore.Event) []QueuedScene {
	return Queue(FromStateEvents(events))
}

// FromStateEvents extracts transition triggers from the current state event log.
func FromStateEvents(events []statestore.Event) []Event {
	out := make([]Event, 0, len(events))
	streaks := map[string]int{}
	mastered := map[string]bool{}
	for _, event := range events {
		switch event.Type {
		case statestore.EventEnemyHit:
			if event.CardID == "" {
				continue
			}
			if cleanHit(event) {
				streaks[event.CardID]++
				if streaks[event.CardID] >= statestore.MasteryCleanHitStreak && !mastered[event.CardID] {
					mastered[event.CardID] = true
					out = append(out, Event{Kind: EventCardMastered, Subject: event.CardID})
				}
			} else {
				streaks[event.CardID] = 0
			}
		case statestore.EventEnemyMissed:
			streaks[event.CardID] = 0
		case statestore.EventCardMastered:
			if !mastered[event.CardID] {
				mastered[event.CardID] = true
				out = append(out, Event{Kind: EventCardMastered, Subject: event.CardID})
			}
		case statestore.EventLevelUnlocked:
			out = append(out, Event{Kind: EventStationArrival, Subject: event.LevelID})
		case statestore.EventBossIntro:
			out = append(out, Event{Kind: EventBossIntro, Subject: event.BossID})
		case statestore.EventBossDamaged:
			out = append(out, Event{Kind: EventBossCrack, Subject: event.BossID})
		case statestore.EventBossCleared:
			out = append(out, Event{Kind: EventLevelClear, Subject: event.BossID})
		}
	}
	return out
}

func (e Event) sceneID() SceneID {
	switch e.Kind {
	case EventCardMastered:
		return SceneCardMastery
	case EventStationArrival:
		return SceneStationArrival
	case EventBossIntro:
		return SceneBossIntro
	case EventBossCrack:
		return SceneBossCrack
	case EventLevelClear:
		return SceneLevelClear
	default:
		return ""
	}
}

func cleanHit(event statestore.Event) bool {
	return event.Clean == nil || *event.Clean
}

func cloneDefinition(definition Definition) Definition {
	clone := definition
	clone.Frames = make([]Frame, 0, len(definition.Frames))
	for _, frame := range definition.Frames {
		lines := append([]string(nil), frame.Lines...)
		clone.Frames = append(clone.Frames, Frame{
			ID:    frame.ID,
			Lines: lines,
		})
	}
	return clone
}
