package sshapp

import (
	"errors"
	"net"
	"os"
	"strings"

	statestore "github.com/superposition/kotoba-line/internal/state"
)

const (
	defaultHost        = "127.0.0.1"
	defaultPort        = "2222"
	defaultHTTPHost    = "127.0.0.1"
	defaultHTTPPort    = "8080"
	defaultUser        = "player"
	defaultPassword    = "kotoba"
	defaultHostKeyPath = "state/ssh_host_ed25519"
)

type Config struct {
	Host          string
	Port          string
	User          string
	Users         []string
	Password      string
	UserPasswords map[string]string
	HostKeyPath   string
	StateDBPath   string
	DatabaseURL   string
	HTTPHost      string
	HTTPPort      string
}

func LoadConfig() Config {
	return ConfigFromLookup(os.LookupEnv)
}

func ConfigFromLookup(lookup func(string) (string, bool)) Config {
	return Config{
		Host:     lookupOrDefault(lookup, "KOTOBA_SSH_HOST", defaultHost),
		Port:     lookupOrDefault(lookup, "KOTOBA_SSH_PORT", defaultPort),
		User:     lookupOrDefault(lookup, "KOTOBA_SSH_USER", defaultUser),
		Users:    splitUsers(lookupValue(lookup, "KOTOBA_SSH_USERS")),
		Password: lookupOrDefault(lookup, "KOTOBA_SSH_PASSWORD", defaultPassword),
		UserPasswords: parseUserPasswords(
			firstNonBlank(lookupValue(lookup, "KOTOBA_SSH_USER_PASSWORDS"), lookupValue(lookup, "KOTOBA_USER_PASSWORDS")),
		),
		HostKeyPath: lookupOrDefault(lookup, "KOTOBA_SSH_HOST_KEY_PATH", defaultHostKeyPath),
		StateDBPath: statestore.DefaultSQLitePathFromLookup(lookup),
		DatabaseURL: firstNonBlank(lookupValue(lookup, "KOTOBA_DATABASE_URL"), lookupValue(lookup, "DATABASE_URL")),
		HTTPHost:    lookupOrDefault(lookup, "KOTOBA_HTTP_HOST", defaultHTTPHost),
		HTTPPort:    firstNonBlank(lookupValue(lookup, "KOTOBA_HTTP_PORT"), lookupValue(lookup, "PORT"), defaultHTTPPort),
	}
}

func (c Config) Address() string {
	return net.JoinHostPort(c.Host, c.Port)
}

func (c Config) HTTPAddress() string {
	return net.JoinHostPort(c.HTTPHost, c.HTTPPort)
}

func (c Config) AuthUsers() []string {
	users := c.Users
	if len(c.Users) > 0 {
		users = c.Users
	} else {
		users = []string{c.User}
	}
	out := make([]string, 0, len(users))
	seen := map[string]bool{}
	for _, user := range users {
		user = strings.TrimSpace(user)
		if user == "" || seen[user] {
			continue
		}
		seen[user] = true
		out = append(out, user)
	}
	return out
}

func (c Config) Authenticator() Authenticator {
	return NewAuthenticatorForUsersWithPasswords(c.AuthUsers(), c.Password, c.UserPasswords)
}

func (c Config) Validate() error {
	switch {
	case strings.TrimSpace(c.Host) == "":
		return errors.New("KOTOBA_SSH_HOST cannot be empty")
	case strings.TrimSpace(c.Port) == "":
		return errors.New("KOTOBA_SSH_PORT cannot be empty")
	case len(c.AuthUsers()) == 0:
		return errors.New("KOTOBA_SSH_USER or KOTOBA_SSH_USERS cannot be empty")
	case c.Password == "":
		return errors.New("KOTOBA_SSH_PASSWORD cannot be empty")
	case strings.TrimSpace(c.HostKeyPath) == "":
		return errors.New("KOTOBA_SSH_HOST_KEY_PATH cannot be empty")
	case strings.TrimSpace(c.StateDBPath) == "" && strings.TrimSpace(c.DatabaseURL) == "":
		return errors.New("KOTOBA_STATE_DB cannot be empty")
	case c.sharedDefaultPasswordExposesUser() && !isLoopbackHost(c.Host):
		return errors.New("KOTOBA_SSH_PASSWORD must be set for non-local SSH hosts")
	default:
		return nil
	}
}

func (c Config) ValidateHTTP() error {
	switch {
	case strings.TrimSpace(c.HTTPHost) == "":
		return errors.New("KOTOBA_HTTP_HOST cannot be empty")
	case strings.TrimSpace(c.HTTPPort) == "":
		return errors.New("KOTOBA_HTTP_PORT or PORT cannot be empty")
	case len(c.AuthUsers()) == 0:
		return errors.New("KOTOBA_SSH_USER or KOTOBA_SSH_USERS cannot be empty")
	case c.Password == "":
		return errors.New("KOTOBA_SSH_PASSWORD cannot be empty")
	case strings.TrimSpace(c.StateDBPath) == "" && strings.TrimSpace(c.DatabaseURL) == "":
		return errors.New("KOTOBA_STATE_DB cannot be empty")
	case c.sharedDefaultPasswordExposesUser() && !isLoopbackHost(c.HTTPHost):
		return errors.New("KOTOBA_SSH_PASSWORD must be set for non-local HTTP hosts")
	default:
		return nil
	}
}

func (c Config) sharedDefaultPasswordExposesUser() bool {
	if c.Password != defaultPassword {
		return false
	}
	for _, user := range c.AuthUsers() {
		if c.UserPasswords[user] == "" {
			return true
		}
	}
	return false
}

func lookupValue(lookup func(string) (string, bool), key string) string {
	value, ok := lookup(key)
	if !ok {
		return ""
	}
	return value
}

func lookupOrDefault(lookup func(string) (string, bool), key string, fallback string) string {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func splitUsers(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\t' || r == ' '
	})
	users := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		user := strings.TrimSpace(part)
		if user == "" || seen[user] {
			continue
		}
		seen[user] = true
		users = append(users, user)
	}
	return users
}

func parseUserPasswords(value string) map[string]string {
	passwords := map[string]string{}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\t' || r == ' '
	})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, password, ok := strings.Cut(part, "=")
		if !ok {
			key, password, ok = strings.Cut(part, ":")
		}
		key = strings.TrimSpace(key)
		password = strings.TrimSpace(password)
		if !ok || key == "" || password == "" {
			continue
		}
		passwords[key] = password
	}
	return passwords
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isLoopbackHost(host string) bool {
	switch strings.TrimSpace(strings.ToLower(host)) {
	case "127.0.0.1", "::1", "localhost":
		return true
	default:
		return false
	}
}
