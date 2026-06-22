package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/superposition/kotoba-line/internal/content"
	"github.com/superposition/kotoba-line/internal/game"
	statestore "github.com/superposition/kotoba-line/internal/state"
	"github.com/superposition/kotoba-line/internal/tui/atoms"
)

const seedContentFile = "seed-2026-06-22.json"

type Options struct {
	Username        string
	Library         *content.Library
	EventLogPath    string
	DisableEventLog bool
}

type Model struct {
	username string
	width    int
	height   int
	drill    game.Drill
	input    string
	last     string
	hint     string
	loadErr  string
	logErr   string
	eventLog statestore.EventLog
}

func New(opts Options) Model {
	username := strings.TrimSpace(opts.Username)
	if username == "" {
		username = "player"
	}

	library := opts.Library
	loadErr := ""
	if library == nil {
		loaded, report, err := loadSeedContent()
		if err != nil {
			loadErr = fmt.Sprintf("seed content unavailable: %v", err)
		} else if report.HasErrors() {
			loadErr = "seed content has validation errors"
		} else {
			library = loaded
		}
	}

	model := Model{
		username: username,
		drill:    game.NewDrill(library, game.Config{}).Start(),
		loadErr:  loadErr,
	}
	if !opts.DisableEventLog {
		if opts.EventLogPath != "" {
			model.eventLog = statestore.NewEventLog(opts.EventLogPath)
		} else {
			model.eventLog = statestore.DefaultEventLog()
		}
	}
	return model
}

func (m Model) Init() tea.Cmd {
	return drillTick()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case drillTickMsg:
		m.drill = m.drill.Tick()
		return m, drillTick()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			m = m.submitInput()
		case "backspace", "ctrl+h":
			m.input = trimLastRune(m.input)
		case "?":
			m = m.showHint()
		default:
			if msg.Type == tea.KeyRunes {
				m.input += string(msg.Runes)
				m.last = ""
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	width := m.cardWidth()
	lines := []string{
		"Kotoba Line",
		"",
		fmt.Sprintf("Player: %s", m.username),
		"",
		"Station 01: Tide Gate",
	}

	if m.loadErr != "" {
		lines = append(lines, "", m.loadErr)
	} else {
		lines = append(lines, "", m.drillCard(width))
	}

	if m.width > 0 && m.height > 0 {
		lines = append(lines, "", fmt.Sprintf("Terminal: %dx%d", m.width, m.height))
	}

	return strings.Join(lines, "\n")
}

type drillTickMsg struct{}

func drillTick() tea.Cmd {
	return tea.Tick(700*time.Millisecond, func(time.Time) tea.Msg {
		return drillTickMsg{}
	})
}

func (m Model) submitInput() Model {
	var result game.AnswerResult
	m.drill, result = m.drill.SubmitKana(m.input)
	m.input = ""
	m.hint = ""

	switch result.Status {
	case game.AnswerHit:
		m.appendEvent(statestore.EnemyHit(result.Enemy.CardID))
		m.last = fmt.Sprintf("HIT %s -> %s", result.Enemy.Text, result.Enemy.Kana)
		if len(m.drill.Enemies()) == 0 {
			m.drill, _ = m.drill.Spawn()
		}
	case game.AnswerMiss:
		if result.Enemy.CardID != "" {
			m.appendEvent(statestore.EnemyMissed(result.Enemy.CardID))
		}
		m.last = fmt.Sprintf("MISS %s", result.Input)
	default:
		m.last = "READY"
	}

	return m
}

func (m Model) showHint() Model {
	var hint game.HintResult
	m.drill, hint = m.drill.Hint()
	if !hint.Available {
		m.hint = "hint: no target"
		return m
	}
	m.appendEvent(statestore.HintRevealed(hint.Enemy.CardID, "romaji"))
	m.hint = fmt.Sprintf("hint: %s = %s", hint.Enemy.Text, hint.Romaji)
	return m
}

func (m *Model) appendEvent(event statestore.Event) {
	if m.eventLog.Path() == "" {
		return
	}
	if err := m.eventLog.Append(event); err != nil {
		m.logErr = err.Error()
	}
}

func (m Model) drillCard(width int) string {
	body := []string{
		fmt.Sprintf("HIT %02d  MISS %02d  HINT %02d", m.drill.Hits(), m.drill.Misses(), m.drill.Hints()),
	}

	enemies := m.drill.Enemies()
	if len(enemies) == 0 {
		body = append(body, "no targets")
	} else {
		for _, enemy := range enemies {
			body = append(body, fmt.Sprintf("row %02d  %s  %s", enemy.Row, enemy.Text, enemy.Meaning))
		}
	}

	body = append(body, atoms.StripANSI(atoms.InputBar("かな", m.input, width-4, true)))
	if m.hint != "" {
		body = append(body, m.hint)
	}
	if m.logErr != "" {
		body = append(body, "log: "+m.logErr)
	}
	if m.last != "" {
		body = append(body, m.last)
	}

	return atoms.Card(atoms.CardSpec{
		Title:    "DRILL",
		Subtitle: "kana wave",
		Body:     body,
		Width:    width,
	})
}

func (m Model) cardWidth() int {
	if m.width == 0 {
		return 64
	}
	if m.width < 40 {
		return 40
	}
	if m.width > 72 {
		return 72
	}
	return m.width
}

func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
}

func loadSeedContent() (*content.Library, content.ValidationReport, error) {
	candidates := seedContentCandidates()
	var lastErr error
	for _, candidate := range candidates {
		library, report, err := content.LoadFile(candidate)
		if err == nil {
			return library, report, nil
		}
		lastErr = err
	}
	return nil, content.ValidationReport{}, lastErr
}

func seedContentCandidates() []string {
	candidates := []string{filepath.Join("content", seedContentFile)}
	wd, err := os.Getwd()
	if err != nil {
		return candidates
	}

	for dir := wd; ; dir = filepath.Dir(dir) {
		candidates = append(candidates, filepath.Join(dir, "content", seedContentFile))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return candidates
}
