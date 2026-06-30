package state

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReplayDeterminism(t *testing.T) {
	events := []Event{
		LevelUnlocked("station-01"),
		BossIntro("boss-a"),
		HintRevealed("card-a", "kana"),
		EnemyHit("card-a"),
		BossDamaged("boss-a", "card-a"),
		EnemyMissed("card-a"),
		EnemyHit("card-a"),
		EnemyHit("card-a"),
		EnemyHit("card-a"),
		BossCleared("boss-a"),
		CardMastered("card-b"),
	}

	first, err := ReplayEvents(events)
	if err != nil {
		t.Fatalf("first replay: %v", err)
	}
	second, err := ReplayEvents(events)
	if err != nil {
		t.Fatalf("second replay: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("replay state differed:\nfirst:  %#v\nsecond: %#v", first, second)
	}
}

func TestMissDoesNotEraseLearnedCard(t *testing.T) {
	progress, err := ReplayEvents([]Event{
		EnemyHit("card-a"),
		EnemyMissed("card-a"),
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	card := progress.Cards["card-a"]
	if !card.Mastered {
		t.Fatalf("card should stay mastered after later miss")
	}
}

func TestOneHitMastersCard(t *testing.T) {
	progress, err := ReplayEvents([]Event{
		EnemyHit("card-a"),
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	card := progress.Cards["card-a"]
	if card.Streak != MasteryCleanHitStreak {
		t.Fatalf("streak = %d, want %d", card.Streak, MasteryCleanHitStreak)
	}
	if !card.Mastered {
		t.Fatalf("card should be mastered after one hit")
	}
}

func TestHintedHitCountsTowardMastery(t *testing.T) {
	progress, err := ReplayEvents([]Event{
		EnemyHitWithClean("card-a", false),
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	card := progress.Cards["card-a"]
	if card.Streak != MasteryCleanHitStreak {
		t.Fatalf("streak = %d, want %d", card.Streak, MasteryCleanHitStreak)
	}
	if !card.Mastered {
		t.Fatalf("hinted hit should still master card")
	}
}

func TestHintTracking(t *testing.T) {
	progress, err := ReplayEvents([]Event{
		HintRevealed("card-a", "kana"),
		HintRevealed("card-a", "meaning"),
		HintRevealed("card-a", "kana"),
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	card := progress.Cards["card-a"]
	if card.HintsUsed != 3 {
		t.Fatalf("hints used = %d, want 3", card.HintsUsed)
	}
	for _, hintID := range []string{"kana", "meaning"} {
		if !card.RevealedHints[hintID] {
			t.Fatalf("hint %q was not tracked: %#v", hintID, card.RevealedHints)
		}
	}
}

func TestLevelUnlocksAreTracked(t *testing.T) {
	progress, err := ReplayEvents([]Event{
		LevelUnlocked("station-01"),
		LevelUnlocked("station-02"),
		LevelUnlocked("station-01"),
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	for _, levelID := range []string{"station-01", "station-02"} {
		if !progress.UnlockedLevels[levelID] {
			t.Fatalf("level %q was not unlocked: %#v", levelID, progress.UnlockedLevels)
		}
	}
}

func TestPointsReplayClampsAtZero(t *testing.T) {
	progress, err := ReplayEvents([]Event{
		Points(120, "clean hit"),
		Points(-25, "wipeout"),
		Points(-500, "wipeout"),
		Points(40, "hinted hit"),
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if progress.Points != 40 {
		t.Fatalf("points = %d, want 40", progress.Points)
	}
}

func TestPointsRequireReason(t *testing.T) {
	if err := ValidateEvent(Points(10, "")); err == nil {
		t.Fatal("points event without reason validated")
	}
	if err := ValidateEvent(Points(0, "noop")); err == nil {
		t.Fatal("zero points event validated")
	}
}

func TestSnapshotEquivalentToReplay(t *testing.T) {
	events := []Event{
		LevelUnlocked("station-01"),
		HintRevealed("card-a", "kana"),
		EnemyHit("card-a"),
		EnemyHit("card-a"),
		EnemyHit("card-a"),
		EnemyMissed("card-b"),
		CardMastered("card-c"),
	}

	replayed, err := ReplayEvents(events)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	snapshot, err := NewSnapshot(events)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if snapshot.EventCount != len(events) {
		t.Fatalf("event count = %d, want %d", snapshot.EventCount, len(events))
	}
	if !reflect.DeepEqual(snapshot.Progress, replayed) {
		t.Fatalf("snapshot differs from replay:\nsnapshot: %#v\nreplay:   %#v", snapshot.Progress, replayed)
	}
}

func TestEventLogAppendReadAndReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "events.jsonl")
	log := NewEventLog(path)
	events := []Event{
		LevelUnlocked("station-01"),
		BossIntro("boss-a"),
		EnemyHit("card-a"),
		BossDamaged("boss-a", "card-a"),
		EnemyHit("card-a"),
		EnemyHit("card-a"),
		BossCleared("boss-a"),
	}

	for _, event := range events {
		if err := log.Append(event); err != nil {
			t.Fatalf("append %s: %v", event.Type, err)
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw log: %v", err)
	}
	if got := strings.Count(string(raw), "\n"); got != len(events) {
		t.Fatalf("jsonl line count = %d, want %d\n%s", got, len(events), raw)
	}

	readEvents, err := log.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if !reflect.DeepEqual(readEvents, events) {
		t.Fatalf("read events differed:\nread: %#v\nwant: %#v", readEvents, events)
	}

	progress, err := log.Replay()
	if err != nil {
		t.Fatalf("replay log: %v", err)
	}
	if !progress.Cards["card-a"].Mastered {
		t.Fatalf("card-a should be mastered after replaying log")
	}
}

func TestDefaultEventLogPath(t *testing.T) {
	t.Setenv("KOTOBA_STATE_DIR", "")
	t.Setenv("RAILWAY_ENVIRONMENT", "")
	t.Setenv("RAILWAY_PROJECT_ID", "")
	t.Setenv("RAILWAY_SERVICE_ID", "")
	if got := DefaultEventLogPath(); got != filepath.Join("state", "events.jsonl") {
		t.Fatalf("local default path = %q", got)
	}
	if got := DefaultEventLog().Path(); got != filepath.Join("state", "events.jsonl") {
		t.Fatalf("local default log path = %q", got)
	}

	t.Setenv("RAILWAY_ENVIRONMENT", "production")
	if got := DefaultEventLogPath(); got != filepath.Join(string(os.PathSeparator), "data", "events.jsonl") {
		t.Fatalf("railway default path = %q", got)
	}

	t.Setenv("KOTOBA_STATE_DIR", "/tmp/kotoba-state")
	if got := DefaultEventLogPath(); got != filepath.Join("/tmp/kotoba-state", "events.jsonl") {
		t.Fatalf("explicit state dir path = %q", got)
	}
}
