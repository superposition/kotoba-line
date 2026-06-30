package state

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrCredentialExists = errors.New("user already exists")
	ErrInvalidUsername  = errors.New("username must be 3-32 characters: letters, numbers, underscore, or dash")
	ErrInvalidPassword  = errors.New("password must be at least 3 characters")
)

var usernameRE = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]{2,31}$`)

func ValidateSignup(username string, password string) error {
	if !usernameRE.MatchString(strings.TrimSpace(username)) {
		return ErrInvalidUsername
	}
	if len(password) < 3 {
		return ErrInvalidPassword
	}
	return nil
}

func (s SQLiteEventStore) CreatePasswordUser(username string, password string) error {
	username = strings.TrimSpace(username)
	if err := ValidateSignup(username, password); err != nil {
		return err
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return createPasswordUser(db, placeholderSQLite, username, password)
}

func (s SQLiteEventStore) AuthenticatePasswordUser(username string, password string) (bool, error) {
	db, err := s.open()
	if err != nil {
		return false, err
	}
	defer db.Close()
	return authenticatePasswordUser(db, placeholderSQLite, username, password)
}

func (s PostgresEventStore) CreatePasswordUser(username string, password string) error {
	username = strings.TrimSpace(username)
	if err := ValidateSignup(username, password); err != nil {
		return err
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return createPasswordUser(db, placeholderPostgres, username, password)
}

func (s PostgresEventStore) AuthenticatePasswordUser(username string, password string) (bool, error) {
	db, err := s.open()
	if err != nil {
		return false, err
	}
	defer db.Close()
	return authenticatePasswordUser(db, placeholderPostgres, username, password)
}

type placeholderStyle int

const (
	placeholderSQLite placeholderStyle = iota
	placeholderPostgres
)

func createPasswordUser(db *sql.DB, style placeholderStyle, username string, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	touchQuery := `INSERT INTO users (username) VALUES (?) ON CONFLICT(username) DO UPDATE SET last_seen_at = last_seen_at`
	insertQuery := `INSERT INTO auth_users (username, password_hash) VALUES (?, ?)`
	args := []any{username, string(hash)}
	if style == placeholderPostgres {
		touchQuery = `INSERT INTO users (username) VALUES ($1) ON CONFLICT(username) DO UPDATE SET last_seen_at = users.last_seen_at`
		insertQuery = `INSERT INTO auth_users (username, password_hash) VALUES ($1, $2)`
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin credential create: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(touchQuery, username); err != nil {
		return fmt.Errorf("touch credential user: %w", err)
	}
	if _, err := tx.Exec(insertQuery, args...); err != nil {
		if isUniqueConstraint(err) {
			return ErrCredentialExists
		}
		return fmt.Errorf("insert credential user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit credential create: %w", err)
	}
	return nil
}

func authenticatePasswordUser(db *sql.DB, style placeholderStyle, username string, password string) (bool, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return false, nil
	}
	query := `SELECT password_hash FROM auth_users WHERE username = ?`
	if style == placeholderPostgres {
		query = `SELECT password_hash FROM auth_users WHERE username = $1`
	}
	var hash string
	if err := db.QueryRow(query, username).Scan(&hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("read credential user: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return false, nil
	}
	update := `UPDATE users SET last_seen_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE username = ?`
	if style == placeholderPostgres {
		update = `UPDATE users SET last_seen_at = now() WHERE username = $1`
	}
	if _, err := db.Exec(update, username); err != nil {
		return false, fmt.Errorf("touch credential login: %w", err)
	}
	return true, nil
}

func isUniqueConstraint(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate key")
}
