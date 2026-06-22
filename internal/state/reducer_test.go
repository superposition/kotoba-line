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
		HintRevealed("card-a", "kana"),
		EnemyHit("card-a"),
		EnemyMissed("card-a"),
		EnemyHit("card-a"),
		EnemyHit("card-a"),
		EnemyHit("card-a"),
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

func TestMissResetsStreak(t *testing.T) {
	progress, err := ReplayEvents([]Event{
		EnemyHit("card-a"),
		EnemyHit("card-a"),
		EnemyMissed("card-a"),
		EnemyHit("card-a"),
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	card := progress.Cards["card-a"]
	if card.Streak != 1 {
		t.Fatalf("streak = %d, want 1", card.Streak)
	}
	if card.Mastered {
		t.Fatalf("card should not be mastered after miss reset")
	}
}

func TestThreeCleanHitsMasterCard(t *testing.T) {
	progress, err := ReplayEvents([]Event{
		EnemyHit("card-a"),
		EnemyHit("card-a"),
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
		t.Fatalf("card should be mastered after 3 clean hits")
	}
}

func TestUncleanHitBreaksMasteryStreak(t *testing.T) {
	progress, err := ReplayEvents([]Event{
		EnemyHit("card-a"),
		EnemyHitWithClean("card-a", false),
		EnemyHit("card-a"),
		EnemyHit("card-a"),
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	card := progress.Cards["card-a"]
	if card.Streak != 2 {
		t.Fatalf("streak = %d, want 2", card.Streak)
	}
	if card.Mastered {
		t.Fatalf("card should not be mastered after an unclean hit breaks the streak")
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
		EnemyHit("card-a"),
		EnemyHit("card-a"),
		EnemyHit("card-a"),
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
