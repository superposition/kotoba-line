package state

import "io"

type Snapshot struct {
	Version    int      `json:"version"`
	EventCount int      `json:"event_count"`
	Progress   Progress `json:"progress"`
}

func NewSnapshot(events []Event) (Snapshot, error) {
	progress, err := ReplayEvents(events)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		Version:    1,
		EventCount: len(events),
		Progress:   progress,
	}, nil
}

func SnapshotFromReader(reader io.Reader) (Snapshot, error) {
	events, err := ReadEvents(reader)
	if err != nil {
		return Snapshot{}, err
	}
	return NewSnapshot(events)
}
