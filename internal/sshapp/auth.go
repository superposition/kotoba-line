package sshapp

import "crypto/subtle"

type Authenticator struct {
	user     string
	password string
}

func NewAuthenticator(user string, password string) Authenticator {
	return Authenticator{user: user, password: password}
}

func (a Authenticator) Authenticate(user string, password string) bool {
	if subtle.ConstantTimeCompare([]byte(user), []byte(a.user)) != 1 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) == 1
}
