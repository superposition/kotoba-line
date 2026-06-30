package sshapp

import (
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTTPLoginRendersGameplay(t *testing.T) {
	handler := newTestHTTPHandler(t)
	cookie := loginHTTP(t, handler, "logohere", "secret")

	req := httptest.NewRequest(http.MethodGet, "/play", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /play status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{"KOTOBA BEACH", "logohere", "goal    catch the kana wave", "target", "meaning", "sound", "[ keys _"} {
		if !strings.Contains(body, want) {
			t.Fatalf("play body missing %q:\n%s", want, body)
		}
	}
	for _, bad := range []string{"SHELVES", "DOCUMENT", "TREE", "SQLite Lesson", "next key", "press   ["} {
		if strings.Contains(body, bad) {
			t.Fatalf("play body should not show noisy marker %q:\n%s", bad, body)
		}
	}
}

func TestHTTPLoginUsesPerUserPassword(t *testing.T) {
	handler := newTestHTTPHandler(t)
	cookie := loginHTTP(t, handler, "superposition", "kotoba")
	if cookie.Value == "" {
		t.Fatal("superposition override login returned empty session cookie")
	}

	form := url.Values{"username": {"logohere"}, "password": {"kotoba"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /login status = %d, want 200", rr.Code)
	}
	if cookies := rr.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("logohere accepted superposition password: %#v", cookies)
	}
}

func TestHTTPKeyEndpointUpdatesScreen(t *testing.T) {
	handler := newTestHTTPHandler(t)
	cookie := loginHTTP(t, handler, "thescoho", "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/key", strings.NewReader(`{"key":"?","width":92,"height":28}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/key status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var response screenResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode screen response: %v", err)
	}
	if response.Username != "thescoho" {
		t.Fatalf("Username = %q, want thescoho", response.Username)
	}
	if !strings.Contains(response.Screen, "hint") {
		t.Fatalf("screen response missing hint text:\n%s", response.Screen)
	}
}

func TestHTTPKeyEndpointRejectsWrongLetterAndAdvancesCorrectLetter(t *testing.T) {
	handler := newTestHTTPHandler(t)
	cookie := loginHTTP(t, handler, "thescoho", "secret")

	initial := postKeyHTTP(t, handler, cookie, "__render__")
	next := extractNextKey(t, initial.Screen)
	wrong := "x"
	if strings.EqualFold(next, wrong) {
		wrong = "z"
	}

	response := postKeyHTTP(t, handler, cookie, wrong)
	if !strings.Contains(response.Screen, "target") || !strings.Contains(response.Screen, "sound") || !strings.Contains(response.Screen, "feedback wipeout "+strings.ToUpper(wrong)) {
		t.Fatalf("wrong key response missing red target/sound feedback:\n%s", response.Screen)
	}
	if strings.Contains(response.Screen, "[ keys "+wrong+"_") {
		t.Fatalf("wrong key should not enter key lane:\n%s", response.Screen)
	}

	response = postKeyHTTP(t, handler, cookie, strings.ToLower(next))
	if !strings.Contains(response.Screen, "[ keys "+strings.ToLower(next)+"_") || !strings.Contains(response.Screen, "sound") {
		t.Fatalf("correct key should advance key lane:\n%s", response.Screen)
	}
	if strings.Contains(response.Screen, "target!") || strings.Contains(response.Screen, "wipeout") {
		t.Fatalf("correct key should clear wrong feedback:\n%s", response.Screen)
	}
}

func TestHTTPLoginRejectsWrongPassword(t *testing.T) {
	handler := newTestHTTPHandler(t)

	form := url.Values{"username": {"logohere"}, "password": {"wrong"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /login status = %d, want 200", rr.Code)
	}
	if cookies := rr.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("wrong password set cookies: %#v", cookies)
	}
	if !strings.Contains(rr.Body.String(), "Login failed") {
		t.Fatalf("wrong password body missing failure message:\n%s", rr.Body.String())
	}
}

func TestHTTPSignupCreatesPasswordUser(t *testing.T) {
	handler := newTestHTTPHandler(t)

	form := url.Values{"username": {"hikari2"}, "password": {"123"}}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST /signup status = %d, want 303: %s", rr.Code, rr.Body.String())
	}
	var cookie *http.Cookie
	for _, candidate := range rr.Result().Cookies() {
		if candidate.Name == sessionCookieName {
			cookie = candidate
		}
	}
	if cookie == nil {
		t.Fatal("signup did not set session cookie")
	}

	req = httptest.NewRequest(http.MethodGet, "/play", nil)
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /play after signup status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "hikari2") {
		t.Fatalf("play body missing signup user:\n%s", rr.Body.String())
	}

	cookie = loginHTTP(t, handler, "hikari2", "123")
	if cookie.Value == "" {
		t.Fatal("new signup user could not log in")
	}
}

func TestHTTPSignupRejectsReservedStaticUser(t *testing.T) {
	handler := newTestHTTPHandler(t)

	form := url.Values{"username": {"superposition"}, "password": {"123"}}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /signup status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "reserved") {
		t.Fatalf("reserved signup body missing message:\n%s", rr.Body.String())
	}
	if cookies := rr.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("reserved signup set cookies: %#v", cookies)
	}
}

func postKeyHTTP(t *testing.T, handler http.Handler, cookie *http.Cookie, key string) screenResponse {
	t.Helper()
	body, err := json.Marshal(keyRequest{Key: key, Width: 92, Height: 28})
	if err != nil {
		t.Fatalf("marshal key request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/key", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/key status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var response screenResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode screen response: %v", err)
	}
	return response
}

func extractNextKey(t *testing.T, screen string) string {
	t.Helper()
	plain := html.UnescapeString(screen)
	start := strings.Index(plain, "sound")
	if start < 0 {
		t.Fatalf("screen missing sound cue:\n%s", screen)
	}
	plain = plain[start:]
	start = strings.Index(plain, "[")
	if start < 0 {
		t.Fatalf("screen missing sound key box:\n%s", screen)
	}
	plain = plain[start+1:]
	end := strings.Index(plain, "]")
	if end < 0 {
		t.Fatalf("screen has malformed sound key box:\n%s", screen)
	}
	return strings.TrimSpace(plain[:end])
}

func newTestHTTPHandler(t *testing.T) http.Handler {
	t.Helper()

	cfg := Config{
		HTTPHost: "127.0.0.1",
		HTTPPort: "0",
		User:     "player",
		Users:    []string{"logohere", "thescoho", "superposition"},
		Password: "secret",
		UserPasswords: map[string]string{
			"superposition": "kotoba",
		},
		StateDBPath: filepath.Join(t.TempDir(), "kotoba.sqlite"),
	}
	handler, err := NewHTTPHandler(cfg)
	if err != nil {
		t.Fatalf("NewHTTPHandler: %v", err)
	}
	return handler
}

func loginHTTP(t *testing.T, handler http.Handler, username string, password string) *http.Cookie {
	t.Helper()

	form := url.Values{"username": {username}, "password": {password}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST /login status = %d, want 303: %s", rr.Code, rr.Body.String())
	}
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			return cookie
		}
	}
	t.Fatalf("POST /login did not set %s cookie", sessionCookieName)
	return nil
}
