package station

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Catalog struct {
	Stations []Station `json:"stations"`
}

type Station struct {
	ID          string      `json:"id"`
	Number      int         `json:"number,omitempty"`
	Name        string      `json:"name"`
	Line        string      `json:"line,omitempty"`
	LevelID     string      `json:"level_id,omitempty"`
	Description string      `json:"description,omitempty"`
	VisualPulse VisualPulse `json:"visual_pulse,omitempty"`
	MIDIHooks   []MIDIHook  `json:"midi_hooks,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
}

type MIDIHook struct {
	ID          string      `json:"id"`
	Label       string      `json:"label,omitempty"`
	Path        string      `json:"path,omitempty"`
	Source      string      `json:"source,omitempty"`
	Optional    bool        `json:"optional"`
	VisualPulse VisualPulse `json:"visual_pulse,omitempty"`
	Notes       []string    `json:"notes,omitempty"`
}

type VisualPulse struct {
	Enabled     bool   `json:"enabled"`
	BPM         int    `json:"bpm,omitempty"`
	Pattern     string `json:"pattern,omitempty"`
	Color       string `json:"color,omitempty"`
	AccentEvery int    `json:"accent_every,omitempty"`
}

func (c Catalog) Find(id string) (Station, bool) {
	for _, station := range c.Stations {
		if station.ID == id {
			return station, true
		}
	}
	return Station{}, false
}

func (c *Catalog) RegisterLocalMIDI(stationID, path string) error {
	stationID = strings.TrimSpace(stationID)
	path = strings.TrimSpace(path)
	if stationID == "" {
		return fmt.Errorf("station id is required")
	}
	if path == "" {
		return fmt.Errorf("midi path is required")
	}

	for i := range c.Stations {
		if c.Stations[i].ID != stationID {
			continue
		}
		for _, hook := range c.Stations[i].MIDIHooks {
			if hook.Path == path {
				return nil
			}
		}
		c.Stations[i].MIDIHooks = append(c.Stations[i].MIDIHooks, MIDIHook{
			ID:          localHookID(path),
			Label:       "Local MIDI",
			Path:        path,
			Source:      "local",
			Optional:    true,
			VisualPulse: c.Stations[i].VisualPulse,
		})
		return nil
	}

	return fmt.Errorf("station %q not found", stationID)
}

func localHookID(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" {
		id = "midi"
	}
	return "local-" + id
}
