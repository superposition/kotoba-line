package audio

import (
	"os"

	"github.com/superposition/kotoba-line/internal/station"
)

type RuntimeMode string

const (
	RuntimeLocal   RuntimeMode = "local"
	RuntimeRailway RuntimeMode = "railway"
)

type PlaybackMode string

const (
	PlaybackMIDIFile    PlaybackMode = "midi_file"
	PlaybackVisualPulse PlaybackMode = "visual_pulse"
	PlaybackSilent      PlaybackMode = "silent"
)

const (
	ReasonNoMIDIHook        = "no_midi_hook"
	ReasonMIDIDisabled      = "midi_tool_unavailable"
	ReasonMIDIFileMissing   = "midi_file_unavailable"
	ReasonRailwayVisualOnly = "railway_visual_only"
)

type Options struct {
	Runtime     RuntimeMode
	MIDIEnabled bool
	FileExists  func(path string) bool
}

type PlaybackPlan struct {
	StationID   string
	Mode        PlaybackMode
	MIDIPath    string
	VisualPulse station.VisualPulse
	Silent      bool
	Reason      string
}

func PlanStation(st station.Station, opts Options) PlaybackPlan {
	hook, hasHook := firstMIDIHook(st)
	pulse := pulseFor(st, hook)

	if opts.Runtime == RuntimeRailway {
		return fallbackPlan(st.ID, pulse, ReasonRailwayVisualOnly)
	}
	if !hasHook {
		return fallbackPlan(st.ID, pulse, ReasonNoMIDIHook)
	}
	if !opts.MIDIEnabled {
		return fallbackPlan(st.ID, pulse, ReasonMIDIDisabled)
	}
	if hook.Path == "" || !fileExists(opts, hook.Path) {
		return fallbackPlan(st.ID, pulse, ReasonMIDIFileMissing)
	}

	return PlaybackPlan{
		StationID:   st.ID,
		Mode:        PlaybackMIDIFile,
		MIDIPath:    hook.Path,
		VisualPulse: pulse,
	}
}

func firstMIDIHook(st station.Station) (station.MIDIHook, bool) {
	for _, hook := range st.MIDIHooks {
		if hook.Path != "" {
			return hook, true
		}
	}
	if len(st.MIDIHooks) == 0 {
		return station.MIDIHook{}, false
	}
	return st.MIDIHooks[0], true
}

func pulseFor(st station.Station, hook station.MIDIHook) station.VisualPulse {
	if hook.VisualPulse.Enabled {
		return hook.VisualPulse
	}
	return st.VisualPulse
}

func fallbackPlan(stationID string, pulse station.VisualPulse, reason string) PlaybackPlan {
	mode := PlaybackSilent
	if pulse.Enabled {
		mode = PlaybackVisualPulse
	}
	return PlaybackPlan{
		StationID:   stationID,
		Mode:        mode,
		VisualPulse: pulse,
		Silent:      true,
		Reason:      reason,
	}
}

func fileExists(opts Options, path string) bool {
	if opts.FileExists != nil {
		return opts.FileExists(path)
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
