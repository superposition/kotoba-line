package sshapp

import "crypto/subtle"

type Authenticator struct {
	passwords map[string]string
}

func NewAuthenticator(user string, password string) Authenticator {
	return NewAuthenticatorForUsers([]string{user}, password)
}

func NewAuthenticatorForUsers(users []string, password string) Authenticator {
	return NewAuthenticatorForUsersWithPasswords(users, password, nil)
}

func NewAuthenticatorForUsersWithPasswords(users []string, password string, userPasswords map[string]string) Authenticator {
	passwords := make(map[string]string, len(users))
	for _, user := range users {
		if user == "" {
			continue
		}
		passwords[user] = password
	}
	for user, userPassword := range userPasswords {
		if user == "" || userPassword == "" {
			continue
		}
		if _, ok := passwords[user]; ok {
			passwords[user] = userPassword
		}
	}
	return Authenticator{passwords: passwords}
}

func (a Authenticator) Authenticate(user string, password string) bool {
	expected, ok := a.passwords[user]
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(password), []byte(expected)) == 1
}
