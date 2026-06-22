package sshapp

import (
	"errors"
	"net"
	"os"
	"strings"
)

const (
	defaultHost        = "127.0.0.1"
	defaultPort        = "2222"
	defaultUser        = "player"
	defaultPassword    = "kotoba"
	defaultHostKeyPath = "state/ssh_host_ed25519"
)

type Config struct {
	Host        string
	Port        string
	User        string
	Password    string
	HostKeyPath string
}

func LoadConfig() Config {
	return ConfigFromLookup(os.LookupEnv)
}

func ConfigFromLookup(lookup func(string) (string, bool)) Config {
	return Config{
		Host:        lookupOrDefault(lookup, "KOTOBA_SSH_HOST", defaultHost),
		Port:        lookupOrDefault(lookup, "KOTOBA_SSH_PORT", defaultPort),
		User:        lookupOrDefault(lookup, "KOTOBA_SSH_USER", defaultUser),
		Password:    lookupOrDefault(lookup, "KOTOBA_SSH_PASSWORD", defaultPassword),
		HostKeyPath: lookupOrDefault(lookup, "KOTOBA_SSH_HOST_KEY_PATH", defaultHostKeyPath),
	}
}

func (c Config) Address() string {
	return net.JoinHostPort(c.Host, c.Port)
}

func (c Config) Validate() error {
	switch {
	case strings.TrimSpace(c.Host) == "":
		return errors.New("KOTOBA_SSH_HOST cannot be empty")
	case strings.TrimSpace(c.Port) == "":
		return errors.New("KOTOBA_SSH_PORT cannot be empty")
	case strings.TrimSpace(c.User) == "":
		return errors.New("KOTOBA_SSH_USER cannot be empty")
	case c.Password == "":
		return errors.New("KOTOBA_SSH_PASSWORD cannot be empty")
	case strings.TrimSpace(c.HostKeyPath) == "":
		return errors.New("KOTOBA_SSH_HOST_KEY_PATH cannot be empty")
	case c.Password == defaultPassword && !isLoopbackHost(c.Host):
		return errors.New("KOTOBA_SSH_PASSWORD must be set for non-local SSH hosts")
	default:
		return nil
	}
}

func lookupOrDefault(lookup func(string) (string, bool), key string, fallback string) string {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func isLoopbackHost(host string) bool {
	switch strings.TrimSpace(strings.ToLower(host)) {
	case "127.0.0.1", "::1", "localhost":
		return true
	default:
		return false
	}
}
