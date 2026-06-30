package skilltree

import (
	"testing"

	statestore "github.com/superposition/kotoba-line/internal/state"
)

func TestStateProgressAdapter(t *testing.T) {
	progress, err := statestore.ReplayEvents([]statestore.Event{
		statestore.EnemyHit("mastered-card"),
		statestore.EnemyMissed("missed-card"),
	})
	if err != nil {
		t.Fatalf("replay state events: %v", err)
	}

	adapter := StateProgress{Progress: progress}

	if got := adapter.CardProgress("mastered-card"); got.Status != StatusMastered {
		t.Fatalf("mastered card progress = %#v, want mastered", got)
	}
	if got := adapter.CardProgress("missed-card"); got.Status != StatusDiscovered {
		t.Fatalf("missed card progress = %#v, want discovered", got)
	}
	if got := adapter.CardProgress("new-card"); got.Status != StatusDiscovered {
		t.Fatalf("new card progress = %#v, want discovered", got)
	}
}
