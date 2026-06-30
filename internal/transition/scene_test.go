package transition

import (
	"reflect"
	"strings"
	"testing"

	statestore "github.com/superposition/kotoba-line/internal/state"
)

func TestDefinitionsCoverRequiredScenesWithBriefDurations(t *testing.T) {
	want := []SceneID{
		SceneAnswerHit,
		SceneAnswerMiss,
		SceneCardMastery,
		SceneStationArrival,
		SceneBossIntro,
		SceneBossCrack,
		SceneLevelClear,
	}

	definitions := Definitions()
	if len(definitions) != len(want) {
		t.Fatalf("definition count = %d, want %d", len(definitions), len(want))
	}

	for i, sceneID := range want {
		definition := definitions[i]
		if definition.ID != sceneID {
			t.Fatalf("definition %d id = %s, want %s", i, definition.ID, sceneID)
		}
		if definition.DurationMS <= 0 || definition.DurationMS > 1000 {
			t.Fatalf("%s duration = %dms, want brief positive duration", sceneID, definition.DurationMS)
		}
		if definition.FrameMS <= 0 {
			t.Fatalf("%s frame duration = %dms, want positive frame duration", sceneID, definition.FrameMS)
		}
		if got := len(definition.Frames) * definition.FrameMS; got != definition.DurationMS {
			t.Fatalf("%s duration = %dms, want frame count total %dms", sceneID, definition.DurationMS, got)
		}
		if len(definition.Frames) < 3 {
			t.Fatalf("%s has %d frames, want at least 3", sceneID, len(definition.Frames))
		}
	}
}

func TestQueuePreservesOrderAndSceneContent(t *testing.T) {
	scenes := Queue([]Event{
		{Kind: EventStationArrival, Subject: "station-01"},
		{Kind: EventCardMastered, Subject: "card-hi"},
		{Kind: EventBossIntro, Subject: "preamble-boss"},
		{Kind: EventBossCrack, Subject: "preamble-boss"},
		{Kind: EventLevelClear, Subject: "station-01"},
	})

	wantIDs := []SceneID{
		SceneStationArrival,
		SceneCardMastery,
		SceneBossIntro,
		SceneBossCrack,
		SceneLevelClear,
	}
	if got := sceneIDs(scenes); !reflect.DeepEqual(got, wantIDs) {
		t.Fatalf("scene order = %#v, want %#v", got, wantIDs)
	}

	content := queueContent(scenes)
	for _, want := range []string{
		"WAVE START",
		"station-01",
		"NEXT WORD",
		"card-hi",
		"BOSS",
		"CRACK",
		"WAVE CLEAR",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("queued content missing %q in:\n%s", want, content)
		}
	}
	for _, unwanted := range []string{"POWER UP", "weapon charge", "combo flare"} {
		if strings.Contains(content, unwanted) {
			t.Fatalf("queued content should not include old charge copy %q in:\n%s", unwanted, content)
		}
	}
}

func TestQueueIsReplayableAndReturnsFreshFrameCopies(t *testing.T) {
	events := []Event{
		{Kind: EventCardMastered, Subject: "card-a"},
		{Kind: EventBossCrack, Subject: "boss-a"},
	}

	first := Queue(events)
	second := Queue(events)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("queue replay differed:\nfirst:  %#v\nsecond: %#v", first, second)
	}

	first[0].Definition.Frames[0].Lines[0] = "mutated"
	third := Queue(events)
	if third[0].Definition.Frames[0].Lines[0] == "mutated" {
		t.Fatalf("scene definitions should be copied before queueing")
	}
}

func TestQueueFromStateEventsAdaptsDurableProgressEvents(t *testing.T) {
	scenes := QueueFromStateEvents([]statestore.Event{
		statestore.BossIntro("boss-a"),
		statestore.EnemyHit("card-a"),
		statestore.EnemyMissed("card-a"),
		statestore.EnemyHit("card-a"),
		statestore.EnemyHit("card-a"),
		statestore.EnemyHit("card-a"),
		statestore.BossDamaged("boss-a", "card-a"),
		statestore.BossCleared("boss-a"),
		statestore.LevelUnlocked("station-02"),
	})

	want := []SceneID{SceneBossIntro, SceneCardMastery, SceneBossCrack, SceneLevelClear, SceneStationArrival}
	if got := sceneIDs(scenes); !reflect.DeepEqual(got, want) {
		t.Fatalf("state scene order = %#v, want %#v", got, want)
	}

	content := queueContent(scenes)
	for _, wantText := range []string{"boss-a", "card-a", "station-02"} {
		if !strings.Contains(content, wantText) {
			t.Fatalf("state queue missing %q in:\n%s", wantText, content)
		}
	}
}

func sceneIDs(scenes []QueuedScene) []SceneID {
	ids := make([]SceneID, 0, len(scenes))
	for _, scene := range scenes {
		ids = append(ids, scene.Definition.ID)
	}
	return ids
}

func queueContent(scenes []QueuedScene) string {
	var b strings.Builder
	for _, scene := range scenes {
		b.WriteString(scene.Definition.Title)
		b.WriteByte('\n')
		for _, frame := range scene.Frames() {
			b.WriteString(frame.ID)
			b.WriteByte('\n')
			b.WriteString(strings.Join(frame.Lines, "\n"))
			b.WriteByte('\n')
		}
	}
	return b.String()
}
