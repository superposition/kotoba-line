package audio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/superposition/kotoba-line/internal/content"
	"github.com/superposition/kotoba-line/internal/game"
	"github.com/superposition/kotoba-line/internal/station"
)

func TestPlanUsesRegisteredLocalMIDIWhenAvailable(t *testing.T) {
	midiPath := filepath.Join(t.TempDir(), "tide-gate.mid")
	if err := os.WriteFile(midiPath, []byte("MThd"), 0o600); err != nil {
		t.Fatalf("write temp midi placeholder: %v", err)
	}
	catalog := station.Catalog{Stations: []station.Station{testStation()}}
	if err := catalog.RegisterLocalMIDI("tide-gate", midiPath); err != nil {
		t.Fatalf("register local midi: %v", err)
	}
	st, ok := catalog.Find("tide-gate")
	if !ok {
		t.Fatalf("registered station not found")
	}

	plan := PlanStation(st, Options{MIDIEnabled: true})
	if plan.Mode != PlaybackMIDIFile {
		t.Fatalf("playback mode = %q, want midi_file: %#v", plan.Mode, plan)
	}
	if plan.MIDIPath != midiPath {
		t.Fatalf("midi path = %q, want %q", plan.MIDIPath, midiPath)
	}
	if plan.Silent {
		t.Fatalf("available local midi should not be silent")
	}
}

func TestMissingMIDIFileFallsBackSilentlyToVisualPulse(t *testing.T) {
	st := testStation()
	st.MIDIHooks = []station.MIDIHook{{
		ID:       "missing-local",
		Path:     filepath.Join(t.TempDir(), "missing.mid"),
		Optional: true,
	}}

	plan := PlanStation(st, Options{MIDIEnabled: true})
	if plan.Mode != PlaybackVisualPulse {
		t.Fatalf("playback mode = %q, want visual_pulse: %#v", plan.Mode, plan)
	}
	if !plan.Silent {
		t.Fatalf("missing midi should produce a silent audio plan")
	}
	if plan.Reason != ReasonMIDIFileMissing {
		t.Fatalf("fallback reason = %q", plan.Reason)
	}
}

func TestRailwayUsesVisualPulseOnly(t *testing.T) {
	st := testStation()
	st.MIDIHooks = []station.MIDIHook{{
		ID:       "available-local",
		Path:     filepath.Join("assets", "midi", "tide-gate.local.mid"),
		Optional: true,
	}}

	plan := PlanStation(st, Options{
		Runtime:     RuntimeRailway,
		MIDIEnabled: true,
		FileExists:  func(string) bool { return true },
	})
	if plan.Mode != PlaybackVisualPulse {
		t.Fatalf("railway playback mode = %q, want visual_pulse", plan.Mode)
	}
	if plan.MIDIPath != "" {
		t.Fatalf("railway plan should not expose a midi path: %#v", plan)
	}
	if !plan.Silent || plan.Reason != ReasonRailwayVisualOnly {
		t.Fatalf("railway plan should be silent visual fallback: %#v", plan)
	}
}

func TestSilentFallbackDoesNotBlockDrill(t *testing.T) {
	st := testStation()
	st.MIDIHooks = []station.MIDIHook{{
		ID:       "local-disabled",
		Path:     filepath.Join("assets", "midi", "tide-gate.local.mid"),
		Optional: true,
	}}
	plan := PlanStation(st, Options{MIDIEnabled: false})
	if plan.Mode != PlaybackVisualPulse || !plan.Silent {
		t.Fatalf("disabled midi should produce silent visual fallback: %#v", plan)
	}

	drill := game.NewDrillFromCards([]content.Card{{
		ID:       "hi",
		Text:     "日",
		Reading:  content.Reading{Kana: "ひ", RomajiHint: "hi"},
		Meaning:  "sun; day",
		Type:     content.CardTypeKanjiReading,
		Playable: true,
	}}, game.Config{}).Start()
	if len(drill.Enemies()) != 1 {
		t.Fatalf("silent audio fallback should not block gameplay spawn: %#v", drill.Enemies())
	}
}

func testStation() station.Station {
	return station.Station{
		ID:   "tide-gate",
		Name: "Tide Gate",
		VisualPulse: station.VisualPulse{
			Enabled:     true,
			BPM:         96,
			Pattern:     "tide",
			Color:       "cyan",
			AccentEvery: 4,
		},
	}
}
