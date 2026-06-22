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
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}
}

func TestConfigFromLookupOverrides(t *testing.T) {
	values := map[string]string{
		"KOTOBA_SSH_HOST":          "0.0.0.0",
		"KOTOBA_SSH_PORT":          "2022",
		"KOTOBA_SSH_USER":          "tester",
		"KOTOBA_SSH_PASSWORD":      "secret",
		"KOTOBA_SSH_HOST_KEY_PATH": "/tmp/kotoba_host_key",
	}
	cfg := ConfigFromLookup(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})

	if cfg.Address() != "0.0.0.0:2022" {
		t.Fatalf("Address() = %q, want %q", cfg.Address(), "0.0.0.0:2022")
	}
	if cfg.User != "tester" || cfg.Password != "secret" || cfg.HostKeyPath != "/tmp/kotoba_host_key" {
		t.Fatalf("ConfigFromLookup() did not apply overrides: %#v", cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error for explicit non-local password: %v", err)
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
