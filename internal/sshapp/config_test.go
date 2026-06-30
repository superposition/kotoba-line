package sshapp

import "testing"

func TestConfigFromLookupDefaults(t *testing.T) {
	cfg := ConfigFromLookup(func(string) (string, bool) { return "", false })

	if cfg.Address() != "127.0.0.1:2222" {
		t.Fatalf("Address() = %q, want %q", cfg.Address(), "127.0.0.1:2222")
	}
	if cfg.User != "player" {
		t.Fatalf("User = %q, want player", cfg.User)
	}
	if cfg.Password != "kotoba" {
		t.Fatalf("Password = %q, want kotoba", cfg.Password)
	}
	if cfg.HostKeyPath != "state/ssh_host_ed25519" {
		t.Fatalf("HostKeyPath = %q, want state/ssh_host_ed25519", cfg.HostKeyPath)
	}
	if cfg.StateDBPath != "state/kotoba.sqlite" {
		t.Fatalf("StateDBPath = %q, want state/kotoba.sqlite", cfg.StateDBPath)
	}
	if cfg.HTTPAddress() != "127.0.0.1:8080" {
		t.Fatalf("HTTPAddress() = %q, want %q", cfg.HTTPAddress(), "127.0.0.1:8080")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}
	if err := cfg.ValidateHTTP(); err != nil {
		t.Fatalf("ValidateHTTP() returned error: %v", err)
	}
}

func TestConfigFromLookupOverrides(t *testing.T) {
	values := map[string]string{
		"KOTOBA_SSH_HOST":          "0.0.0.0",
		"KOTOBA_SSH_PORT":          "2022",
		"KOTOBA_HTTP_HOST":         "0.0.0.0",
		"KOTOBA_HTTP_PORT":         "8081",
		"KOTOBA_SSH_USER":          "tester",
		"KOTOBA_SSH_PASSWORD":      "secret",
		"KOTOBA_SSH_HOST_KEY_PATH": "/tmp/kotoba_host_key",
		"KOTOBA_STATE_DB":          "/tmp/kotoba.sqlite",
	}
	cfg := ConfigFromLookup(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})

	if cfg.Address() != "0.0.0.0:2022" {
		t.Fatalf("Address() = %q, want %q", cfg.Address(), "0.0.0.0:2022")
	}
	if cfg.User != "tester" || cfg.Password != "secret" || cfg.HostKeyPath != "/tmp/kotoba_host_key" || cfg.StateDBPath != "/tmp/kotoba.sqlite" {
		t.Fatalf("ConfigFromLookup() did not apply overrides: %#v", cfg)
	}
	if cfg.HTTPAddress() != "0.0.0.0:8081" {
		t.Fatalf("HTTPAddress() = %q, want %q", cfg.HTTPAddress(), "0.0.0.0:8081")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error for explicit non-local password: %v", err)
	}
	if err := cfg.ValidateHTTP(); err != nil {
		t.Fatalf("ValidateHTTP() returned error for explicit non-local password: %v", err)
	}
}

func TestConfigFromLookupHTTPPortUsesRailwayPort(t *testing.T) {
	values := map[string]string{
		"PORT":                     "9090",
		"KOTOBA_SSH_PASSWORD":      "secret",
		"KOTOBA_STATE_DB":          "/tmp/kotoba.sqlite",
		"KOTOBA_HTTP_HOST":         "0.0.0.0",
		"KOTOBA_SSH_HOST":          "0.0.0.0",
		"KOTOBA_SSH_HOST_KEY_PATH": "/tmp/kotoba_host_key",
	}
	cfg := ConfigFromLookup(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})

	if cfg.HTTPAddress() != "0.0.0.0:9090" {
		t.Fatalf("HTTPAddress() = %q, want %q", cfg.HTTPAddress(), "0.0.0.0:9090")
	}
}

func TestConfigFromLookupDatabaseURL(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":        "postgres://railway:secret@host:5432/railway",
		"KOTOBA_DATABASE_URL": "postgres://kotoba:secret@host:5432/kotoba",
	}
	cfg := ConfigFromLookup(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})

	if cfg.DatabaseURL != "postgres://kotoba:secret@host:5432/kotoba" {
		t.Fatalf("DatabaseURL = %q, want KOTOBA_DATABASE_URL override", cfg.DatabaseURL)
	}
}

func TestConfigFromLookupUsers(t *testing.T) {
	values := map[string]string{
		"KOTOBA_SSH_USERS":          "logohere, thescoho superposition\nlogohere",
		"KOTOBA_SSH_PASSWORD":       "secret",
		"KOTOBA_SSH_USER_PASSWORDS": "superposition=kotoba",
	}
	cfg := ConfigFromLookup(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})

	users := cfg.AuthUsers()
	want := []string{"logohere", "thescoho", "superposition"}
	if len(users) != len(want) {
		t.Fatalf("AuthUsers() = %#v, want %#v", users, want)
	}
	for i := range want {
		if users[i] != want[i] {
			t.Fatalf("AuthUsers() = %#v, want %#v", users, want)
		}
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error for multi-user config: %v", err)
	}
	if got := cfg.UserPasswords["superposition"]; got != "kotoba" {
		t.Fatalf("UserPasswords[superposition] = %q, want kotoba", got)
	}
	auth := cfg.Authenticator()
	if !auth.Authenticate("superposition", "kotoba") {
		t.Fatal("expected superposition override password to authenticate")
	}
	if !auth.Authenticate("logohere", "secret") {
		t.Fatal("expected logohere shared password to authenticate")
	}
	if auth.Authenticate("logohere", "kotoba") {
		t.Fatal("logohere should not inherit superposition override")
	}
}

func TestConfigFromLookupUserPasswordsLegacyAlias(t *testing.T) {
	values := map[string]string{
		"KOTOBA_USER_PASSWORDS": "superposition:kotoba",
	}
	cfg := ConfigFromLookup(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})

	if got := cfg.UserPasswords["superposition"]; got != "kotoba" {
		t.Fatalf("UserPasswords[superposition] = %q, want kotoba", got)
	}
}

func TestConfigValidateRejectsEmptyPassword(t *testing.T) {
	cfg := ConfigFromLookup(func(string) (string, bool) { return "", false })
	cfg.Password = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() succeeded with empty password")
	}
}

func TestConfigValidateRejectsDefaultPasswordOnNonLocalHost(t *testing.T) {
	cfg := ConfigFromLookup(func(string) (string, bool) { return "", false })
	cfg.Host = "0.0.0.0"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() succeeded with default password on non-local host")
	}
}

func TestConfigValidateHTTPRejectsDefaultPasswordOnNonLocalHost(t *testing.T) {
	cfg := ConfigFromLookup(func(string) (string, bool) { return "", false })
	cfg.HTTPHost = "0.0.0.0"

	if err := cfg.ValidateHTTP(); err == nil {
		t.Fatal("ValidateHTTP() succeeded with default password on non-local host")
	}
}

func TestConfigValidateAllowsDefaultOnlyAsExplicitUserOverride(t *testing.T) {
	cfg := ConfigFromLookup(func(string) (string, bool) { return "", false })
	cfg.Host = "0.0.0.0"
	cfg.HTTPHost = "0.0.0.0"
	cfg.Users = []string{"superposition"}
	cfg.Password = "shared-secret"
	cfg.UserPasswords = map[string]string{"superposition": "kotoba"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() rejected explicit user override: %v", err)
	}
	if err := cfg.ValidateHTTP(); err != nil {
		t.Fatalf("ValidateHTTP() rejected explicit user override: %v", err)
	}
}
