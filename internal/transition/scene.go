package transition

import (
	"strings"

	statestore "github.com/superposition/kotoba-line/internal/state"
)

// SceneID identifies one replayable transition scene.
type SceneID string

const (
	SceneAnswerHit      SceneID = "answer_hit"
	SceneAnswerMiss     SceneID = "answer_miss"
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
		ID:         SceneAnswerHit,
		Title:      "BLAST",
		DurationMS: 240,
		FrameMS:    80,
		Frames: []Frame{
			{ID: "shot", Lines: []string{"    |", "{subject}", "reading fired"}},
			{ID: "burst", Lines: []string{"  * * *", "{subject}", "glyph burst"}},
			{ID: "wake", Lines: []string{" ~~~~~", "{subject}", "wake fades"}},
		},
	},
	{
		ID:         SceneAnswerMiss,
		Title:      "MISS",
		DurationMS: 240,
		FrameMS:    80,
		Frames: []Frame{
			{ID: "wide", Lines: []string{"  .", "{subject}", "shot wide"}},
			{ID: "impact", Lines: []string{"!!!", "{subject}", "hull shakes"}},
			{ID: "steady", Lines: []string{"...", "{subject}", "line holds"}},
		},
	},
	{
		ID:         SceneCardMastery,
		Title:      "NEXT WORD",
		DurationMS: 240,
		FrameMS:    80,
		Frames: []Frame{
			{ID: "saved", Lines: []string{"OK", "{subject}", "saved"}},
			{ID: "shift", Lines: []string{"-->", "{subject}", "next word"}},
			{ID: "ready", Lines: []string{"READY", "{subject}", "type"}},
		},
	},
	{
		ID:         SceneStationArrival,
		Title:      "WAVE START",
		DurationMS: 700,
		FrameMS:    140,
		Frames: []Frame{
			{ID: "horizon", Lines: []string{"~~~~~ ~~~~~", "{subject}", "ocean lane opens"}},
			{ID: "signal", Lines: []string{"< < < > > >", "{subject}", "station signal"}},
			{ID: "launch", Lines: []string{"|== KOTOBA BEACH ==|", "{subject}", "launch"}},
			{ID: "map", Lines: []string{"o--o--O--o--o", "{subject}", "route lit"}},
			{ID: "ready", Lines: []string{"READY", "{subject}", "fire readings"}},
		},
	},
	{
		ID:         SceneBossIntro,
		Title:      "BOSS",
		DurationMS: 800,
		FrameMS:    160,
		Frames: []Frame{
			{ID: "warning", Lines: []string{"!!! !!! !!!", "{subject}", "boss alarm"}},
			{ID: "shadow", Lines: []string{"######", "{subject}", "glyph rises"}},
			{ID: "emblem", Lines: []string{"<<        >>", "{subject}", "shield online"}},
			{ID: "tide", Lines: []string{"~~~~~", "{subject}", "floor shakes"}},
			{ID: "fight", Lines: []string{">>>", "{subject}", "break weak points"}},
		},
	},
	{
		ID:         SceneBossCrack,
		Title:      "CRACK",
		DurationMS: 440,
		FrameMS:    110,
		Frames: []Frame{
			{ID: "hit", Lines: []string{">>><<<", "{subject}", "coral snap"}},
			{ID: "fracture", Lines: []string{"//// ////", "{subject}", "shell split"}},
			{ID: "spray", Lines: []string{"* * * * *", "{subject}", "seafoam"}},
			{ID: "settle", Lines: []string{"...", "{subject}", "still floating"}},
		},
	},
	{
		ID:         SceneLevelClear,
		Title:      "WAVE CLEAR",
		DurationMS: 750,
		FrameMS:    150,
		Frames: []Frame{
			{ID: "break", Lines: []string{"////", "{subject}", "shield gone"}},
			{ID: "route", Lines: []string{"O==O==O==O", "{subject}", "route lit"}},
			{ID: "banner", Lines: []string{"^^^^", "{subject}", "signal high"}},
			{ID: "unlock", Lines: []string{"o--o--O", "{subject}", "next wave"}},
			{ID: "fade", Lines: []string{"... ... ...", "{subject}", "calm"}},
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
