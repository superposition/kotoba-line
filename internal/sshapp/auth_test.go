package sshapp

import "testing"

func TestAuthenticator(t *testing.T) {
	auth := NewAuthenticator("player", "kotoba")

	tests := []struct {
		name     string
		user     string
		password string
		want     bool
	}{
		{name: "correct credentials", user: "player", password: "kotoba", want: true},
		{name: "wrong password", user: "player", password: "wrong", want: false},
		{name: "wrong user", user: "admin", password: "kotoba", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := auth.Authenticate(tt.user, tt.password); got != tt.want {
				t.Fatalf("Authenticate(%q, %q) = %v, want %v", tt.user, tt.password, got, tt.want)
			}
		})
	}
}

func TestAuthenticatorForUsers(t *testing.T) {
	auth := NewAuthenticatorForUsers([]string{"logohere", "thescoho", "superposition"}, "shared-secret")

	for _, user := range []string{"logohere", "thescoho", "superposition"} {
		if !auth.Authenticate(user, "shared-secret") {
			t.Fatalf("Authenticate(%q) = false, want true", user)
		}
	}
	if auth.Authenticate("player", "shared-secret") {
		t.Fatal("unexpected player auth with multi-user list")
	}
	if auth.Authenticate("logohere", "wrong") {
		t.Fatal("unexpected auth with wrong password")
	}
}

func TestAuthenticatorForUsersWithPasswords(t *testing.T) {
	auth := NewAuthenticatorForUsersWithPasswords(
		[]string{"logohere", "thescoho", "superposition"},
		"shared-secret",
		map[string]string{"superposition": "kotoba", "unknown": "ignored"},
	)

	if !auth.Authenticate("superposition", "kotoba") {
		t.Fatal("expected superposition override password to authenticate")
	}
	if auth.Authenticate("superposition", "shared-secret") {
		t.Fatal("unexpected superposition auth with shared password")
	}
	for _, user := range []string{"logohere", "thescoho"} {
		if !auth.Authenticate(user, "shared-secret") {
			t.Fatalf("Authenticate(%q) = false, want shared password auth", user)
		}
		if auth.Authenticate(user, "kotoba") {
			t.Fatalf("Authenticate(%q) accepted superposition override", user)
		}
	}
	if auth.Authenticate("unknown", "ignored") {
		t.Fatal("unexpected auth for unconfigured override user")
	}
}
