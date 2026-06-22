package state

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type EventLog struct {
	path string
}

func NewEventLog(path string) EventLog {
	return EventLog{path: path}
}

func DefaultEventLog() EventLog {
	return NewEventLog(DefaultEventLogPath())
}

func DefaultEventLogPath() string {
	if stateDir := strings.TrimSpace(os.Getenv("KOTOBA_STATE_DIR")); stateDir != "" {
		return filepath.Join(stateDir, "events.jsonl")
	}
	if os.Getenv("RAILWAY_ENVIRONMENT") != "" ||
		os.Getenv("RAILWAY_PROJECT_ID") != "" ||
		os.Getenv("RAILWAY_SERVICE_ID") != "" {
		return filepath.Join(string(os.PathSeparator), "data", "events.jsonl")
	}
	return filepath.Join("state", "events.jsonl")
}

func (l EventLog) Path() string {
	return l.path
}

func (l EventLog) Append(event Event) error {
	if l.path == "" {
		return errors.New("state event log path is empty")
	}
	if err := ValidateEvent(event); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return fmt.Errorf("create event log directory: %w", err)
	}

	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open event log: %w", err)
	}
	defer file.Close()

	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	line = append(line, '\n')
	if _, err := file.Write(line); err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync event log: %w", err)
	}
	return nil
}

func (l EventLog) ReadAll() ([]Event, error) {
	file, err := os.Open(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open event log: %w", err)
	}
	defer file.Close()

	return ReadEvents(file)
}

func (l EventLog) Replay() (Progress, error) {
	events, err := l.ReadAll()
	if err != nil {
		return Progress{}, err
	}
	return ReplayEvents(events)
}

func ReadEvents(reader io.Reader) ([]Event, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var events []Event
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("decode event log line %d: %w", lineNumber, err)
		}
		if err := ValidateEvent(event); err != nil {
			return nil, fmt.Errorf("invalid event log line %d: %w", lineNumber, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read event log: %w", err)
	}
	return events, nil
}

func ValidateEvent(event Event) error {
	switch event.Type {
	case EventEnemyHit, EventEnemyMissed, EventHintRevealed, EventCardMastered:
		if event.CardID == "" {
			return fmt.Errorf("%s event requires card_id", event.Type)
		}
	case EventLevelUnlocked:
		if event.LevelID == "" {
			return fmt.Errorf("%s event requires level_id", event.Type)
		}
	case EventBossIntro, EventBossCleared:
		if event.BossID == "" {
			return fmt.Errorf("%s event requires boss_id", event.Type)
		}
	case EventBossDamaged:
		if event.BossID == "" {
			return fmt.Errorf("%s event requires boss_id", event.Type)
		}
		if event.CardID == "" {
			return fmt.Errorf("%s event requires card_id", event.Type)
		}
	default:
		return fmt.Errorf("unknown event type %q", event.Type)
	}
	return nil
}
