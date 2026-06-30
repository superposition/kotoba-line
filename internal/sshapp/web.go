package sshapp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	statestore "github.com/superposition/kotoba-line/internal/state"
	"github.com/superposition/kotoba-line/internal/tui/app"
)

const sessionCookieName = "kotoba_session"

type webApp struct {
	cfg         Config
	auth        Authenticator
	credentials credentialStore
	sessions    map[string]*webSession
	mu          sync.Mutex
}

type credentialStore interface {
	CreatePasswordUser(username string, password string) error
	AuthenticatePasswordUser(username string, password string) (bool, error)
}

type webSession struct {
	username string
	model    app.Model
	lastSeen time.Time
	mu       sync.Mutex
}

type keyRequest struct {
	Key    string `json:"key"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type screenResponse struct {
	Screen   string `json:"screen"`
	Username string `json:"username"`
}

func NewHTTPServer(cfg Config) (*http.Server, error) {
	handler, err := NewHTTPHandler(cfg)
	if err != nil {
		return nil, err
	}
	return &http.Server{
		Addr:              cfg.HTTPAddress(),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}, nil
}

func NewHTTPHandler(cfg Config) (http.Handler, error) {
	if err := cfg.ValidateHTTP(); err != nil {
		return nil, err
	}
	app := &webApp{
		cfg:         cfg,
		auth:        cfg.Authenticator(),
		credentials: newCredentialStore(cfg),
		sessions:    map[string]*webSession{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.serveIndex)
	mux.HandleFunc("/play", app.servePlay)
	mux.HandleFunc("/login", app.serveLogin)
	mux.HandleFunc("/signup", app.serveSignup)
	mux.HandleFunc("/logout", app.serveLogout)
	mux.HandleFunc("/api/key", app.serveKey)
	mux.HandleFunc("/healthz", app.serveHealth)
	return mux, nil
}

func (a *webApp) serveHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, "ok\n")
}

func (a *webApp) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if _, ok := a.currentSession(r); ok {
		http.Redirect(w, r, "/play", http.StatusSeeOther)
		return
	}
	a.writeLogin(w, "")
}

func (a *webApp) serveLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.writeLogin(w, "")
		return
	}
	if err := r.ParseForm(); err != nil {
		a.writeLogin(w, "Login failed")
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	ok := a.auth.Authenticate(username, password)
	if !ok && a.credentials != nil {
		var err error
		ok, err = a.credentials.AuthenticatePasswordUser(username, password)
		if err != nil {
			ok = false
		}
	}
	if !ok {
		a.writeLogin(w, "Login failed")
		return
	}

	a.startSession(w, r, username)
}

func (a *webApp) serveSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.writeSignup(w, "")
		return
	}
	if err := r.ParseForm(); err != nil {
		a.writeSignup(w, "Signup failed")
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if staticUser(a.cfg.AuthUsers(), username) {
		a.writeSignup(w, "That name is reserved")
		return
	}
	if err := statestore.ValidateSignup(username, password); err != nil {
		a.writeSignup(w, err.Error())
		return
	}
	if a.credentials == nil {
		a.writeSignup(w, "Signup is unavailable")
		return
	}
	if err := a.credentials.CreatePasswordUser(username, password); err != nil {
		if errors.Is(err, statestore.ErrCredentialExists) {
			a.writeSignup(w, "That name is taken")
			return
		}
		a.writeSignup(w, "Signup failed")
		return
	}

	a.startSession(w, r, username)
}

func (a *webApp) startSession(w http.ResponseWriter, r *http.Request, username string) {
	token, err := randomToken()
	if err != nil {
		http.Error(w, "session unavailable", http.StatusInternalServerError)
		return
	}
	model := NewGameplayModel(username, a.cfg, nil)
	if updated, _ := model.Update(tea.WindowSizeMsg{Width: 92, Height: 28}); updated != nil {
		if typed, ok := updated.(app.Model); ok {
			model = typed
		}
	}

	a.mu.Lock()
	a.pruneSessionsLocked(time.Now())
	a.sessions[token] = &webSession{username: username, model: model, lastSeen: time.Now()}
	a.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((24 * time.Hour).Seconds()),
	})
	http.Redirect(w, r, "/play", http.StatusSeeOther)
}

func (a *webApp) serveLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		a.mu.Lock()
		delete(a.sessions, cookie.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *webApp) servePlay(w http.ResponseWriter, r *http.Request) {
	session, ok := a.currentSession(r)
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	screen := a.renderSession(session, 92, 28)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, playHTML(session.username, screen))
}

func (a *webApp) serveKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	session, ok := a.currentSession(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req keyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, "bad key request", http.StatusBadRequest)
		return
	}

	screen := a.applyKey(session, req)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(screen)
}

func (a *webApp) currentSession(r *http.Request) (*webSession, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil, false
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	session, ok := a.sessions[cookie.Value]
	if !ok {
		return nil, false
	}
	session.lastSeen = time.Now()
	return session, true
}

func (a *webApp) pruneSessionsLocked(now time.Time) {
	for token, session := range a.sessions {
		if now.Sub(session.lastSeen) > 24*time.Hour {
			delete(a.sessions, token)
		}
	}
}

func (a *webApp) renderSession(session *webSession, width int, height int) string {
	return a.applyKey(session, keyRequest{Key: "__render__", Width: width, Height: height}).Screen
}

func (a *webApp) applyKey(session *webSession, req keyRequest) screenResponse {
	session.mu.Lock()
	defer session.mu.Unlock()

	if req.Width > 0 || req.Height > 0 {
		width := clampInt(req.Width, 40, 100)
		height := clampInt(req.Height, 18, 40)
		if updated, _ := session.model.Update(tea.WindowSizeMsg{Width: width, Height: height}); updated != nil {
			if typed, ok := updated.(app.Model); ok {
				session.model = typed
			}
		}
	}

	if req.Key != "" && req.Key != "__render__" {
		if msg, ok := webKeyMsg(req.Key); ok {
			if updated, _ := session.model.Update(msg); updated != nil {
				if typed, ok := updated.(app.Model); ok {
					session.model = typed
				}
			}
		}
	}

	return screenResponse{
		Screen:   ansiToHTML(session.model.View()),
		Username: session.username,
	}
}

func webKeyMsg(key string) (tea.KeyMsg, bool) {
	switch key {
	case "Enter":
		return tea.KeyMsg{Type: tea.KeyEnter}, true
	case "Backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}, true
	case "Escape":
		return tea.KeyMsg{Type: tea.KeyEsc}, true
	case "Tab":
		return tea.KeyMsg{Type: tea.KeyTab}, true
	case "ArrowUp":
		return tea.KeyMsg{Type: tea.KeyUp}, true
	case "ArrowDown":
		return tea.KeyMsg{Type: tea.KeyDown}, true
	case "ArrowLeft":
		return tea.KeyMsg{Type: tea.KeyLeft}, true
	case "ArrowRight":
		return tea.KeyMsg{Type: tea.KeyRight}, true
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}, true
	}

	runes := []rune(key)
	if len(runes) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: runes}, true
	}
	return tea.KeyMsg{}, false
}

func (a *webApp) writeLogin(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, loginHTML(a.cfg.AuthUsers(), message))
}

func (a *webApp) writeSignup(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, signupHTML(message))
}

func newCredentialStore(cfg Config) credentialStore {
	if strings.TrimSpace(cfg.DatabaseURL) != "" {
		return statestore.NewPostgresEventStore(cfg.DatabaseURL, "system")
	}
	if strings.TrimSpace(cfg.StateDBPath) != "" {
		return statestore.NewSQLiteEventStore(cfg.StateDBPath, "system")
	}
	return nil
}

func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func loginHTML(users []string, message string) string {
	var options strings.Builder
	for _, user := range users {
		options.WriteString(`<option value="`)
		options.WriteString(html.EscapeString(user))
		options.WriteString(`</option>`)
	}

	status := ""
	if message != "" {
		status = `<p class="status">` + html.EscapeString(message) + `</p>`
	}

	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Kotoba Beach</title>
<style>` + baseCSS() + `</style>
</head>
<body class="login">
<main class="login-shell">
  <div class="brand-mark"></div>
  <h1>Kotoba Beach</h1>
  <form method="post" action="/login" class="login-form">
    <label>user<input name="username" list="known-users" autocomplete="username" autofocus><datalist id="known-users">` + options.String() + `</datalist></label>
    <label>password<input name="password" type="password" autocomplete="current-password"></label>
    <button type="submit">Surf</button>
  </form>
  <p class="switch-link"><a href="/signup">create surfer</a></p>
  ` + status + `
</main>
</body>
</html>`
}

func signupHTML(message string) string {
	status := ""
	if message != "" {
		status = `<p class="status">` + html.EscapeString(message) + `</p>`
	}

	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Kotoba Beach Signup</title>
<style>` + baseCSS() + `</style>
</head>
<body class="login">
<main class="login-shell">
  <div class="brand-mark"></div>
  <h1>Kotoba Beach</h1>
  <form method="post" action="/signup" class="login-form">
    <label>user<input name="username" autocomplete="username" autofocus></label>
    <label>password<input name="password" type="password" autocomplete="new-password"></label>
    <button type="submit">Start</button>
  </form>
  <p class="switch-link"><a href="/">back to login</a></p>
  ` + status + `
</main>
</body>
</html>`
}

func playHTML(username string, screen string) string {
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Kotoba Beach</title>
<style>` + baseCSS() + `</style>
</head>
<body class="play">
<header class="topbar">
  <strong>Kotoba Beach</strong>
  <span>` + html.EscapeString(username) + `</span>
  <form method="post" action="/logout"><button type="submit">Exit</button></form>
</header>
<main class="game-shell">
  <pre id="screen" class="screen" tabindex="0">` + screen + `</pre>
</main>
<script>
const screen = document.getElementById('screen');
screen.focus();
function dims() {
  const rect = screen.getBoundingClientRect();
  return {
    width: Math.max(40, Math.min(100, Math.floor(rect.width / 9))),
    height: Math.max(18, Math.min(40, Math.floor(rect.height / 19)))
  };
}
async function sendKey(key) {
  const size = dims();
  const res = await fetch('/api/key', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({key, width: size.width, height: size.height})
  });
  if (res.status === 401) {
    location.href = '/';
    return;
  }
  if (!res.ok) return;
  const data = await res.json();
  screen.innerHTML = data.screen;
}
document.addEventListener('keydown', event => {
  const key = event.key;
  const playable = key.length === 1 || ['Enter','Backspace','Escape','Tab','ArrowUp','ArrowDown','ArrowLeft','ArrowRight'].includes(key);
  if (!playable || event.metaKey || event.ctrlKey || event.altKey) return;
  event.preventDefault();
  sendKey(key);
});
window.addEventListener('resize', () => sendKey('__render__'));
screen.addEventListener('click', () => screen.focus());
sendKey('__render__');
</script>
</body>
</html>`
}

func baseCSS() string {
	return `
:root {
  color-scheme: dark;
  --deep: #061826;
  --ink: #dffcff;
  --aqua: #27d9df;
  --foam: #79f5bf;
  --sun: #ffd166;
  --coral: #ff6b6b;
  --navy: #0a2f47;
  --paper: rgba(245, 255, 252, 0.92);
}
* { box-sizing: border-box; }
html, body { min-height: 100%; }
body {
  margin: 0;
  font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  color: var(--ink);
  background:
    linear-gradient(180deg, rgba(255, 209, 102, 0.24), rgba(39, 217, 223, 0.12) 28%, rgba(6, 24, 38, 0.96) 72%),
    radial-gradient(circle at 12% 14%, rgba(255, 107, 107, 0.42), transparent 24%),
    linear-gradient(135deg, #064663, #071d2f 55%, #061826);
}
button, select, input {
  min-height: 44px;
  border: 1px solid rgba(121, 245, 191, 0.5);
  border-radius: 8px;
  font: inherit;
}
button {
  cursor: pointer;
  background: var(--sun);
  color: #162031;
  font-weight: 800;
  padding: 0 18px;
}
.login {
  display: grid;
  place-items: center;
  padding: 24px;
}
.login-shell {
  width: min(420px, 100%);
  border: 1px solid rgba(121, 245, 191, 0.42);
  border-radius: 8px;
  padding: 28px;
  background: rgba(6, 24, 38, 0.82);
  box-shadow: 0 28px 90px rgba(0, 0, 0, 0.34);
}
.brand-mark {
  height: 42px;
  border-radius: 8px;
  background:
    repeating-linear-gradient(135deg, rgba(255,255,255,0.28) 0 8px, transparent 8px 16px),
    linear-gradient(90deg, var(--aqua), var(--foam), var(--sun), var(--coral));
  margin-bottom: 18px;
}
h1 {
  margin: 0 0 22px;
  font-size: 2rem;
}
.login-form {
  display: grid;
  gap: 14px;
}
label {
  display: grid;
  gap: 6px;
  color: var(--foam);
  font-weight: 700;
}
select, input {
 width: 100%;
 color: var(--ink);
 background: rgba(10, 47, 71, 0.82);
 padding: 0 12px;
}
a { color: var(--foam); font-weight: 800; }
.switch-link { margin: 16px 0 0; }
.status {
  color: var(--coral);
  font-weight: 800;
}
.topbar {
  min-height: 58px;
  display: grid;
  grid-template-columns: 1fr auto auto;
  align-items: center;
  gap: 16px;
  padding: 10px 18px;
  background: rgba(6, 24, 38, 0.72);
  border-bottom: 1px solid rgba(121, 245, 191, 0.32);
}
.topbar strong { color: var(--sun); }
.topbar span { color: var(--foam); font-weight: 700; }
.topbar button { min-height: 36px; }
.game-shell {
  width: min(1180px, calc(100vw - 24px));
  margin: 10px auto;
}
.screen {
  margin: 0;
  min-height: calc(100vh - 96px);
  overflow: auto;
  white-space: pre;
  padding: 18px;
  border: 1px solid rgba(39, 217, 223, 0.58);
  border-radius: 8px;
  outline: none;
  background: rgba(2, 10, 18, 0.88);
  color: var(--ink);
  font: 16px/1.28 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  box-shadow: inset 0 0 0 1px rgba(255,255,255,0.04), 0 22px 80px rgba(0,0,0,0.36);
}
.ansi-bold { font-weight: 800; }
@media (max-width: 680px) {
  .topbar { grid-template-columns: 1fr auto; }
  .topbar span { display: none; }
  .game-shell { width: calc(100vw - 12px); margin-top: 6px; }
  .screen { min-height: calc(100vh - 84px); padding: 12px; font-size: 14px; }
}
`
}

func staticUser(users []string, username string) bool {
	for _, user := range users {
		if strings.EqualFold(strings.TrimSpace(user), strings.TrimSpace(username)) {
			return true
		}
	}
	return false
}

type ansiStyle struct {
	fg   string
	bg   string
	bold bool
}

func ansiToHTML(input string) string {
	var out strings.Builder
	style := ansiStyle{}
	open := false

	closeSpan := func() {
		if open {
			out.WriteString(`</span>`)
			open = false
		}
	}
	openSpan := func() {
		attrs := style.css()
		if attrs == "" {
			return
		}
		out.WriteString(`<span style="`)
		out.WriteString(html.EscapeString(attrs))
		out.WriteString(`">`)
		open = true
	}
	restyle := func(next ansiStyle) {
		closeSpan()
		style = next
		openSpan()
	}

	for i := 0; i < len(input); {
		if input[i] == 0x1b && i+1 < len(input) && input[i+1] == '[' {
			end := i + 2
			for end < len(input) && input[end] != 'm' {
				end++
			}
			if end < len(input) {
				restyle(applySGR(style, input[i+2:end]))
				i = end + 1
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(input[i:])
		if r == utf8.RuneError && size == 0 {
			break
		}
		out.WriteString(html.EscapeString(string(r)))
		i += size
	}
	closeSpan()
	return out.String()
}

func applySGR(style ansiStyle, raw string) ansiStyle {
	if raw == "" {
		return ansiStyle{}
	}
	parts := strings.Split(raw, ";")
	for i := 0; i < len(parts); i++ {
		switch parts[i] {
		case "0":
			style = ansiStyle{}
		case "1":
			style.bold = true
		case "38":
			if i+2 < len(parts) && parts[i+1] == "5" {
				style.fg = ansiColor(parts[i+2])
				i += 2
			}
		case "48":
			if i+2 < len(parts) && parts[i+1] == "5" {
				style.bg = ansiColor(parts[i+2])
				i += 2
			}
		}
	}
	return style
}

func (s ansiStyle) css() string {
	var parts []string
	if s.fg != "" {
		parts = append(parts, "color: "+s.fg)
	}
	if s.bg != "" {
		parts = append(parts, "background: "+s.bg)
	}
	if s.bold {
		parts = append(parts, "font-weight: 800")
	}
	return strings.Join(parts, "; ")
}

func ansiColor(code string) string {
	switch code {
	case "15":
		return "#f8feff"
	case "17":
		return "#071a3a"
	case "33":
		return "#0087ff"
	case "51":
		return "#00ffff"
	case "121":
		return "#87ffaf"
	case "203":
		return "#ff5f5f"
	case "226":
		return "#ffff00"
	default:
		return ""
	}
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
