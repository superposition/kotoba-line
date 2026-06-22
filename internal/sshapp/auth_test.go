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
