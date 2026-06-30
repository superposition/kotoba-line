package state

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresEventStore struct {
	dsn      string
	username string
}

func NewPostgresEventStore(dsn string, username string) PostgresEventStore {
	username = strings.TrimSpace(username)
	if username == "" {
		username = "player"
	}
	return PostgresEventStore{dsn: strings.TrimSpace(dsn), username: username}
}

func (s PostgresEventStore) Path() string {
	if strings.TrimSpace(s.dsn) == "" {
		return ""
	}
	return "postgres"
}

func (s PostgresEventStore) Username() string {
	return s.username
}

func (s PostgresEventStore) TouchUser() error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return touchPostgresUser(db, s.username)
}

func (s PostgresEventStore) UserRecord() (UserRecord, error) {
	db, err := s.open()
	if err != nil {
		return UserRecord{}, err
	}
	defer db.Close()
	if err := touchPostgresUser(db, s.username); err != nil {
		return UserRecord{}, err
	}

	var record UserRecord
	err = db.QueryRow(`
		SELECT username, created_at::text, last_seen_at::text
		FROM users
		WHERE username = $1
	`, s.username).Scan(&record.Username, &record.CreatedAt, &record.LastSeenAt)
	if err != nil {
		return UserRecord{}, fmt.Errorf("read postgres user: %w", err)
	}
	return record, nil
}

func (s PostgresEventStore) Append(event Event) error {
	if err := ValidateEvent(event); err != nil {
		return err
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin postgres event append: %w", err)
	}
	defer tx.Rollback()

	if err := touchPostgresUserTx(tx, s.username); err != nil {
		return err
	}
	clean, cleanValid := cleanValue(event)
	if _, err := tx.Exec(`
		INSERT INTO events (username, type, card_id, level_id, boss_id, hint_id, clean, points_delta, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, s.username, event.Type, event.CardID, event.LevelID, event.BossID, event.HintID, nullablePostgresBool(clean, cleanValid), event.Points, event.Reason); err != nil {
		return fmt.Errorf("insert postgres event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit postgres event append: %w", err)
	}
	return nil
}

func (s PostgresEventStore) ReadAll() ([]Event, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT type, card_id, level_id, boss_id, hint_id, clean, points_delta, reason
		FROM events
		WHERE username = $1
		ORDER BY id ASC
	`, s.username)
	if err != nil {
		return nil, fmt.Errorf("read postgres events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		var eventType string
		var clean sql.NullBool
		if err := rows.Scan(&eventType, &event.CardID, &event.LevelID, &event.BossID, &event.HintID, &clean, &event.Points, &event.Reason); err != nil {
			return nil, fmt.Errorf("scan postgres event: %w", err)
		}
		event.Type = EventType(eventType)
		if clean.Valid {
			value := clean.Bool
			event.Clean = &value
		}
		if err := ValidateEvent(event); err != nil {
			return nil, fmt.Errorf("invalid postgres event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres events: %w", err)
	}
	return events, nil
}

func (s PostgresEventStore) Replay() (Progress, error) {
	events, err := s.ReadAll()
	if err != nil {
		return Progress{}, err
	}
	return ReplayEvents(events)
}

func (s PostgresEventStore) EventCount() (int, error) {
	db, err := s.open()
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE username = $1`, s.username).Scan(&count); err != nil {
		return 0, fmt.Errorf("count postgres events: %w", err)
	}
	return count, nil
}

func (s PostgresEventStore) open() (*sql.DB, error) {
	if strings.TrimSpace(s.dsn) == "" {
		return nil, errors.New("postgres database url is empty")
	}
	if strings.TrimSpace(s.username) == "" {
		return nil, errors.New("postgres username is empty")
	}

	db, err := sql.Open("pgx", s.dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres state: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	if err := migratePostgres(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migratePostgres(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			username TEXT PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS auth_users (
			username TEXT PRIMARY KEY REFERENCES users(username) ON DELETE CASCADE,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS events (
			id BIGSERIAL PRIMARY KEY,
			username TEXT NOT NULL REFERENCES users(username) ON DELETE CASCADE,
			type TEXT NOT NULL,
			card_id TEXT NOT NULL DEFAULT '',
			level_id TEXT NOT NULL DEFAULT '',
			boss_id TEXT NOT NULL DEFAULT '',
			hint_id TEXT NOT NULL DEFAULT '',
			clean BOOLEAN,
			points_delta INTEGER NOT NULL DEFAULT 0,
			reason TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`ALTER TABLE events ADD COLUMN IF NOT EXISTS points_delta INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE events ADD COLUMN IF NOT EXISTS reason TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS events_username_id_idx ON events(username, id)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("migrate postgres state: %w", err)
		}
	}
	return nil
}

func touchPostgresUser(db *sql.DB, username string) error {
	_, err := db.Exec(`
		INSERT INTO users (username)
		VALUES ($1)
		ON CONFLICT(username) DO UPDATE SET last_seen_at = now()
	`, username)
	if err != nil {
		return fmt.Errorf("touch postgres user: %w", err)
	}
	return nil
}

func touchPostgresUserTx(tx *sql.Tx, username string) error {
	_, err := tx.Exec(`
		INSERT INTO users (username)
		VALUES ($1)
		ON CONFLICT(username) DO UPDATE SET last_seen_at = now()
	`, username)
	if err != nil {
		return fmt.Errorf("touch postgres user: %w", err)
	}
	return nil
}

func nullablePostgresBool(value bool, valid bool) any {
	if !valid {
		return nil
	}
	return value
}
