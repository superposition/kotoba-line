package state

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type UserRecord struct {
	Username   string
	CreatedAt  string
	LastSeenAt string
}

type SQLiteEventStore struct {
	path     string
	username string
}

func NewSQLiteEventStore(path string, username string) SQLiteEventStore {
	username = strings.TrimSpace(username)
	if username == "" {
		username = "player"
	}
	return SQLiteEventStore{path: path, username: username}
}

func DefaultSQLiteEventStore(username string) SQLiteEventStore {
	return NewSQLiteEventStore(DefaultSQLitePath(), username)
}

func DefaultSQLitePath() string {
	return DefaultSQLitePathFromLookup(os.LookupEnv)
}

func DefaultSQLitePathFromLookup(lookup func(string) (string, bool)) string {
	if dbPath := lookupTrimmed(lookup, "KOTOBA_STATE_DB"); dbPath != "" {
		return dbPath
	}
	if stateDir := lookupTrimmed(lookup, "KOTOBA_STATE_DIR"); stateDir != "" {
		return filepath.Join(stateDir, "kotoba.sqlite")
	}
	if lookupPresent(lookup, "RAILWAY_ENVIRONMENT") ||
		lookupPresent(lookup, "RAILWAY_PROJECT_ID") ||
		lookupPresent(lookup, "RAILWAY_SERVICE_ID") {
		return filepath.Join(string(os.PathSeparator), "data", "kotoba.sqlite")
	}
	return filepath.Join("state", "kotoba.sqlite")
}

func (s SQLiteEventStore) Path() string {
	return s.path
}

func (s SQLiteEventStore) Username() string {
	return s.username
}

func (s SQLiteEventStore) TouchUser() error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return touchUser(db, s.username)
}

func (s SQLiteEventStore) UserRecord() (UserRecord, error) {
	db, err := s.open()
	if err != nil {
		return UserRecord{}, err
	}
	defer db.Close()
	if err := touchUser(db, s.username); err != nil {
		return UserRecord{}, err
	}

	var record UserRecord
	err = db.QueryRow(`
		SELECT username, created_at, last_seen_at
		FROM users
		WHERE username = ?
	`, s.username).Scan(&record.Username, &record.CreatedAt, &record.LastSeenAt)
	if err != nil {
		return UserRecord{}, fmt.Errorf("read sqlite user: %w", err)
	}
	return record, nil
}

func (s SQLiteEventStore) Append(event Event) error {
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
		return fmt.Errorf("begin sqlite event append: %w", err)
	}
	defer tx.Rollback()

	if err := touchUserTx(tx, s.username); err != nil {
		return err
	}
	clean, cleanValid := cleanValue(event)
	if _, err := tx.Exec(`
		INSERT INTO events (username, type, card_id, level_id, boss_id, hint_id, clean, points_delta, reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.username, event.Type, event.CardID, event.LevelID, event.BossID, event.HintID, nullableBool(clean, cleanValid), event.Points, event.Reason); err != nil {
		return fmt.Errorf("insert sqlite event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite event append: %w", err)
	}
	return nil
}

func (s SQLiteEventStore) ReadAll() ([]Event, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT type, card_id, level_id, boss_id, hint_id, clean, points_delta, reason
		FROM events
		WHERE username = ?
		ORDER BY id ASC
	`, s.username)
	if err != nil {
		return nil, fmt.Errorf("read sqlite events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		var eventType string
		var clean sql.NullBool
		if err := rows.Scan(&eventType, &event.CardID, &event.LevelID, &event.BossID, &event.HintID, &clean, &event.Points, &event.Reason); err != nil {
			return nil, fmt.Errorf("scan sqlite event: %w", err)
		}
		event.Type = EventType(eventType)
		if clean.Valid {
			value := clean.Bool
			event.Clean = &value
		}
		if err := ValidateEvent(event); err != nil {
			return nil, fmt.Errorf("invalid sqlite event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sqlite events: %w", err)
	}
	return events, nil
}

func (s SQLiteEventStore) Replay() (Progress, error) {
	events, err := s.ReadAll()
	if err != nil {
		return Progress{}, err
	}
	return ReplayEvents(events)
}

func (s SQLiteEventStore) EventCount() (int, error) {
	db, err := s.open()
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE username = ?`, s.username).Scan(&count); err != nil {
		return 0, fmt.Errorf("count sqlite events: %w", err)
	}
	return count, nil
}

func SeedSQLiteFromEventLogIfEmpty(store SQLiteEventStore, log EventLog) (int, error) {
	return SeedEventStoreFromEventLogIfEmpty(store, log)
}

func (s SQLiteEventStore) open() (*sql.DB, error) {
	if strings.TrimSpace(s.path) == "" {
		return nil, errors.New("sqlite state path is empty")
	}
	if strings.TrimSpace(s.username) == "" {
		return nil, errors.New("sqlite username is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite state directory: %w", err)
	}

	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite state: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := configureSQLite(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateSQLite(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func configureSQLite(db *sql.DB) error {
	for _, statement := range []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
	} {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("configure sqlite %q: %w", statement, err)
		}
	}
	return nil
}

func migrateSQLite(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			username TEXT PRIMARY KEY,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
			last_seen_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		)`,
		`CREATE TABLE IF NOT EXISTS auth_users (
			username TEXT PRIMARY KEY REFERENCES users(username) ON DELETE CASCADE,
			password_hash TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		)`,
		`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL REFERENCES users(username) ON DELETE CASCADE,
			type TEXT NOT NULL,
			card_id TEXT NOT NULL DEFAULT '',
			level_id TEXT NOT NULL DEFAULT '',
			boss_id TEXT NOT NULL DEFAULT '',
			hint_id TEXT NOT NULL DEFAULT '',
			clean INTEGER CHECK (clean IN (0, 1) OR clean IS NULL),
			points_delta INTEGER NOT NULL DEFAULT 0,
			reason TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		)`,
		`CREATE INDEX IF NOT EXISTS events_username_id_idx ON events(username, id)`,
		`CREATE TABLE IF NOT EXISTS lessons (
			id TEXT PRIMARY KEY,
			position INTEGER NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			document_title TEXT NOT NULL DEFAULT '',
			required_lesson_id TEXT NOT NULL DEFAULT '',
			required_points INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS lesson_cards (
			id TEXT PRIMARY KEY,
			lesson_id TEXT NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
			position INTEGER NOT NULL,
			text TEXT NOT NULL,
			kanji TEXT NOT NULL DEFAULT '',
			kana TEXT NOT NULL,
			romaji_hint TEXT NOT NULL DEFAULT '',
			meaning TEXT NOT NULL,
			type TEXT NOT NULL,
			notes TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '',
			playable INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE INDEX IF NOT EXISTS lesson_cards_lesson_position_idx ON lesson_cards(lesson_id, position)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("migrate sqlite state: %w", err)
		}
	}
	for _, column := range []sqliteColumn{
		{Table: "events", Name: "points_delta", Definition: "INTEGER NOT NULL DEFAULT 0"},
		{Table: "events", Name: "reason", Definition: "TEXT NOT NULL DEFAULT ''"},
		{Table: "lessons", Name: "required_points", Definition: "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := addSQLiteColumnIfMissing(db, column); err != nil {
			return err
		}
	}
	return nil
}

type sqliteColumn struct {
	Table      string
	Name       string
	Definition string
}

func addSQLiteColumnIfMissing(db *sql.DB, column sqliteColumn) error {
	rows, err := db.Query(`PRAGMA table_info(` + column.Table + `)`)
	if err != nil {
		return fmt.Errorf("inspect sqlite table %s: %w", column.Table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan sqlite table info %s: %w", column.Table, err)
		}
		if name == column.Name {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate sqlite table info %s: %w", column.Table, err)
	}
	if _, err := db.Exec(`ALTER TABLE ` + column.Table + ` ADD COLUMN ` + column.Name + ` ` + column.Definition); err != nil {
		return fmt.Errorf("add sqlite column %s.%s: %w", column.Table, column.Name, err)
	}
	return nil
}

func touchUser(db *sql.DB, username string) error {
	_, err := db.Exec(`
		INSERT INTO users (username)
		VALUES (?)
		ON CONFLICT(username) DO UPDATE SET last_seen_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
	`, username)
	if err != nil {
		return fmt.Errorf("touch sqlite user: %w", err)
	}
	return nil
}

func touchUserTx(tx *sql.Tx, username string) error {
	_, err := tx.Exec(`
		INSERT INTO users (username)
		VALUES (?)
		ON CONFLICT(username) DO UPDATE SET last_seen_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
	`, username)
	if err != nil {
		return fmt.Errorf("touch sqlite user: %w", err)
	}
	return nil
}

func cleanValue(event Event) (bool, bool) {
	if event.Clean == nil {
		return false, false
	}
	return *event.Clean, true
}

func nullableBool(value bool, valid bool) any {
	if !valid {
		return nil
	}
	if value {
		return 1
	}
	return 0
}

func lookupTrimmed(lookup func(string) (string, bool), key string) string {
	value, ok := lookup(key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func lookupPresent(lookup func(string) (string, bool), key string) bool {
	value, ok := lookup(key)
	return ok && strings.TrimSpace(value) != ""
}
