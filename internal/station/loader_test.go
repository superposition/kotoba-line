package station

import (
	"path/filepath"
	"testing"
)

func TestLoadStationCatalog(t *testing.T) {
	catalog, report, err := LoadFile(filepath.Join("..", "..", "content", "stations", "catalog.json"))
	if err != nil {
		t.Fatalf("load station catalog: %v", err)
	}
	if report.HasErrors() {
		t.Fatalf("station catalog has validation errors: %#v", report.Issues)
	}

	tideGate, ok := catalog.Find("tide-gate")
	if !ok {
		t.Fatalf("missing tide-gate station")
	}
	if tideGate.Number != 1 || tideGate.Name != "Tide Gate" {
		t.Fatalf("station metadata = %#v, want station 1 Tide Gate", tideGate)
	}
	if tideGate.LevelID != "journal-2026-06-22-key-readings" {
		t.Fatalf("station level id = %q", tideGate.LevelID)
	}
	if !tideGate.VisualPulse.Enabled || tideGate.VisualPulse.BPM == 0 {
		t.Fatalf("station should have a visual pulse fallback: %#v", tideGate.VisualPulse)
	}
	if len(tideGate.MIDIHooks) != 1 {
		t.Fatalf("midi hook count = %d, want 1", len(tideGate.MIDIHooks))
	}
	hook := tideGate.MIDIHooks[0]
	if !hook.Optional {
		t.Fatalf("catalog midi hook should be optional")
	}
	if hook.Path != filepath.ToSlash(filepath.Join("assets", "midi", "tide-gate.local.mid")) {
		t.Fatalf("midi hook path = %q", hook.Path)
	}

	constitutionGate, ok := catalog.Find("constitution-gate")
	if !ok {
		t.Fatalf("missing constitution-gate station")
	}
	if constitutionGate.LevelID != "constitution-preamble-1" {
		t.Fatalf("constitution gate level id = %q", constitutionGate.LevelID)
	}
	emperorSymbol, ok := catalog.Find("emperor-symbol")
	if !ok {
		t.Fatalf("missing emperor-symbol station")
	}
	if emperorSymbol.LevelID != "constitution-article-1" {
		t.Fatalf("emperor symbol level id = %q", emperorSymbol.LevelID)
	}
}

func TestRegisterLocalMIDIAddsOptionalHook(t *testing.T) {
	catalog := Catalog{Stations: []Station{{
		ID:   "tide-gate",
		Name: "Tide Gate",
		VisualPulse: VisualPulse{
			Enabled: true,
			BPM:     96,
			Pattern: "tide",
		},
	}}}

	err := catalog.RegisterLocalMIDI("tide-gate", filepath.Join("local", "Tide Gate Demo.mid"))
	if err != nil {
		t.Fatalf("register local midi: %v", err)
	}

	station, ok := catalog.Find("tide-gate")
	if !ok {
		t.Fatalf("station disappeared after registration")
	}
	if len(station.MIDIHooks) != 1 {
		t.Fatalf("midi hook count = %d, want 1", len(station.MIDIHooks))
	}
	hook := station.MIDIHooks[0]
	if hook.ID != "local-tide-gate-demo" {
		t.Fatalf("hook id = %q", hook.ID)
	}
	if hook.Source != "local" || !hook.Optional {
		t.Fatalf("local hook metadata = %#v", hook)
	}
	if !hook.VisualPulse.Enabled {
		t.Fatalf("local hook should inherit station visual pulse")
	}
}

func TestOptionalMissingMIDIPathDoesNotInvalidateCatalog(t *testing.T) {
	catalog := &Catalog{Stations: []Station{{
		ID:   "silent-yard",
		Name: "Silent Yard",
		MIDIHooks: []MIDIHook{{
			ID:       "silent-yard-local",
			Optional: true,
		}},
	}}}

	report := ValidateCatalog(catalog)
	if report.HasErrors() {
		t.Fatalf("optional missing midi path should not be an error: %#v", report.Issues)
	}
}
