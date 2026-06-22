package station

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type ValidationSeverity string

const (
	ValidationWarning ValidationSeverity = "warning"
	ValidationError   ValidationSeverity = "error"
)

type ValidationIssue struct {
	Severity ValidationSeverity `json:"severity"`
	Code     string             `json:"code"`
	Message  string             `json:"message"`
	Path     string             `json:"path,omitempty"`
	ID       string             `json:"id,omitempty"`
}

type ValidationReport struct {
	Issues []ValidationIssue `json:"issues"`
}

func LoadFile(path string) (*Catalog, ValidationReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ValidationReport{}, err
	}
	defer f.Close()

	return LoadJSON(f)
}

func LoadJSON(r io.Reader) (*Catalog, ValidationReport, error) {
	var catalog Catalog
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&catalog); err != nil {
		return nil, ValidationReport{}, err
	}

	report := ValidateCatalog(&catalog)
	return &catalog, report, nil
}

func ValidateCatalog(catalog *Catalog) ValidationReport {
	var report ValidationReport
	if catalog == nil {
		report.add(ValidationError, "missing_catalog", "station catalog is nil", "", "")
		return report
	}

	stationIDs := make(map[string]struct{}, len(catalog.Stations))
	for i, station := range catalog.Stations {
		path := fmt.Sprintf("stations[%d]", i)
		if blank(station.ID) {
			report.add(ValidationError, "missing_station_id", "station id is required", path+".id", "")
		} else if _, exists := stationIDs[station.ID]; exists {
			report.add(ValidationError, "duplicate_station_id", "station id must be unique", path+".id", station.ID)
		} else {
			stationIDs[station.ID] = struct{}{}
		}
		if blank(station.Name) {
			report.add(ValidationError, "missing_station_name", "station name is required", path+".name", station.ID)
		}
		validatePulse(&report, path+".visual_pulse", station.ID, station.VisualPulse)

		hookIDs := make(map[string]struct{}, len(station.MIDIHooks))
		for hookIndex, hook := range station.MIDIHooks {
			hookPath := fmt.Sprintf("%s.midi_hooks[%d]", path, hookIndex)
			if blank(hook.ID) {
				report.add(ValidationError, "missing_midi_hook_id", "midi hook id is required", hookPath+".id", station.ID)
			} else if _, exists := hookIDs[hook.ID]; exists {
				report.add(ValidationError, "duplicate_midi_hook_id", "midi hook id must be unique within a station", hookPath+".id", hook.ID)
			} else {
				hookIDs[hook.ID] = struct{}{}
			}
			if blank(hook.Path) && !hook.Optional {
				report.add(ValidationError, "missing_required_midi_path", "required midi hooks need a path", hookPath+".path", hook.ID)
			}
			validatePulse(&report, hookPath+".visual_pulse", hook.ID, hook.VisualPulse)
		}
	}

	return report
}

func (r ValidationReport) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == ValidationError {
			return true
		}
	}
	return false
}

func (r ValidationReport) HasIssue(code string) bool {
	for _, issue := range r.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func validatePulse(report *ValidationReport, path, id string, pulse VisualPulse) {
	if !pulse.Enabled {
		return
	}
	if pulse.BPM < 0 {
		report.add(ValidationError, "invalid_visual_pulse_bpm", "visual pulse bpm cannot be negative", path+".bpm", id)
	}
	if pulse.AccentEvery < 0 {
		report.add(ValidationError, "invalid_visual_pulse_accent", "visual pulse accent cannot be negative", path+".accent_every", id)
	}
}

func (r *ValidationReport) add(severity ValidationSeverity, code, message, path, id string) {
	r.Issues = append(r.Issues, ValidationIssue{
		Severity: severity,
		Code:     code,
		Message:  message,
		Path:     path,
		ID:       id,
	})
}

func blank(value string) bool {
	return strings.TrimSpace(value) == ""
}
