package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/superposition/kotoba-line/internal/boss"
	"github.com/superposition/kotoba-line/internal/content"
	"github.com/superposition/kotoba-line/internal/game"
	statestore "github.com/superposition/kotoba-line/internal/state"
	coretransition "github.com/superposition/kotoba-line/internal/transition"
	"github.com/superposition/kotoba-line/internal/tui/atoms"
	tuitransition "github.com/superposition/kotoba-line/internal/tui/transition"
)

const seedContentFile = "seed-2026-06-22.json"

type screenMode string

const (
	modeDrill screenMode = "drill"
	modeBoss  screenMode = "boss"
)

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
	mode     screenMode
	drill    game.Drill
	boss     boss.Fight
	input    string
	last     string
	hint     string
	bossHint string
	scene    string
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
		mode:     modeDrill,
		drill:    game.NewDrill(library, game.Config{}).Start(),
		boss:     boss.NewFight(newDocumentBoss(library)),
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
		case "b":
			m = m.enterBoss()
		case "esc":
			m.mode = modeDrill
			m.input = ""
			m.hint = ""
			m.bossHint = ""
		case "enter":
			if m.mode == modeBoss {
				m = m.submitBossInput()
			} else {
				m = m.submitInput()
			}
		case "backspace", "ctrl+h":
			m.input = trimLastRune(m.input)
		case "?":
			if m.mode == modeBoss {
				m = m.showBossHint()
			} else {
				m = m.showHint()
			}
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
	} else if m.mode == modeBoss {
		if m.scene != "" {
			lines = append(lines, "", m.scene)
		}
		lines = append(lines, "", m.bossCard(width))
	} else {
		if m.scene != "" {
			lines = append(lines, "", m.scene)
		}
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

func (m Model) enterBoss() Model {
	m.mode = modeBoss
	m.input = ""
	m.hint = ""
	m.bossHint = ""
	b := m.boss.Boss()
	m.appendEvent(statestore.BossIntro(b.ID))
	m.scene = renderScene(coretransition.SceneBossIntro, b.Title, m.cardWidth())
	m.last = "BOSS WAVE"
	return m
}

func (m Model) submitBossInput() Model {
	var result boss.AnswerResult
	m.boss, result = m.boss.SubmitKana(m.input)
	m.input = ""

	b := m.boss.Boss()
	switch result.Status {
	case boss.AnswerHit, boss.AnswerCleared:
		clean := m.bossHint == ""
		m.appendEvent(statestore.EnemyHitWithClean(result.Chunk.ID, clean))
		m.appendEvent(statestore.BossDamaged(b.ID, result.Chunk.ID))
		m.scene = renderScene(coretransition.SceneBossCrack, b.Title, m.cardWidth())
		m.last = fmt.Sprintf("BOSS HIT %s -%d HP", result.Chunk.Text, result.Damage)
		if result.Status == boss.AnswerCleared {
			m.appendEvent(statestore.BossCleared(b.ID))
			m.scene = renderScene(coretransition.SceneLevelClear, b.Title, m.cardWidth())
			m.last = fmt.Sprintf("LEVEL CLEAR %s", b.Title)
		}
	case boss.AnswerMiss:
		if result.Chunk.ID != "" {
			m.appendEvent(statestore.EnemyMissed(result.Chunk.ID))
		}
		m.last = fmt.Sprintf("BOSS MISS %s", result.Input)
	default:
		m.last = "BOSS READY"
	}
	m.bossHint = ""
	return m
}

func (m Model) showBossHint() Model {
	target, ok := m.boss.Target()
	if !ok {
		m.bossHint = "hint: boss cleared"
		return m
	}
	m.appendEvent(statestore.HintRevealed(target.ID, "romaji"))
	m.bossHint = fmt.Sprintf("hint: %s = %s", target.Text, target.RomajiHint)
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
		"b boss  ? hint  q quit",
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

func (m Model) bossCard(width int) string {
	b := m.boss.Boss()
	phase := m.boss.Phase()
	body := []string{
		fmt.Sprintf("%s  %s", phase.Title, phase.Glyph),
		atoms.HPBar(m.boss.HP(), b.HP, width-4),
		centerLine(phase.Glyph, width-4),
		"esc drill  ? hint  q quit",
	}

	if target, ok := m.boss.Target(); ok {
		body = append(body,
			fmt.Sprintf("target %s", target.Text),
			fmt.Sprintf("meaning %s", target.Meaning),
			atoms.StripANSI(atoms.InputBar("かな", m.input, width-4, true)),
		)
	} else {
		body = append(body, "boss cleared", atoms.StripANSI(atoms.InputBar("かな", m.input, width-4, false)))
	}

	if m.bossHint != "" {
		body = append(body, m.bossHint)
	}
	if m.logErr != "" {
		body = append(body, "log: "+m.logErr)
	}
	if m.last != "" {
		body = append(body, m.last)
	}

	return atoms.Card(atoms.CardSpec{
		Title:     "BOSS",
		Subtitle:  b.Title,
		Body:      body,
		Width:     width,
		Highlight: true,
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

func renderScene(sceneID coretransition.SceneID, subject string, width int) string {
	scene, ok := coretransition.SceneFor(sceneID, subject)
	if !ok {
		return ""
	}
	rendered := tuitransition.RenderQueue([]coretransition.QueuedScene{scene}, width)
	if len(rendered) == 0 {
		return ""
	}
	return rendered[0]
}

func newDocumentBoss(library *content.Library) boss.Boss {
	chunks := boss.ChunksFromCards(cardsFromLibrary(library))
	if len(chunks) == 0 {
		chunks = fallbackBossChunks(cardsFromLibrary(library))
	}
	hp := len(chunks) * 2
	if hp < 3 {
		hp = 3
	}
	mid := hp / 2
	if mid < 1 {
		mid = 1
	}

	return boss.Boss{
		ID:    "journal-2026-06-22-boss",
		Title: "日暮ノ門",
		Glyph: "日暮",
		HP:    hp,
		Phases: []boss.Phase{
			{ID: "sealed", Title: "SEALED SUN GATE", Glyph: "日", StartsAtHP: hp},
			{ID: "cracked", Title: "CRACKED EVENING", Glyph: "暮", StartsAtHP: mid},
			{ID: "cleared", Title: "CLEAR WATER", Glyph: "日暮", StartsAtHP: 0},
		},
		Chunks: chunks,
	}
}

func cardsFromLibrary(library *content.Library) []content.Card {
	if library == nil {
		return nil
	}
	return library.Cards
}

func fallbackBossChunks(cards []content.Card) []boss.Chunk {
	chunks := make([]boss.Chunk, 0, 3)
	for _, card := range cards {
		if !card.Playable || strings.TrimSpace(card.Reading.Kana) == "" {
			continue
		}
		chunks = append(chunks, boss.Chunk{
			ID:         card.ID,
			Text:       card.Text,
			Kana:       strings.TrimSpace(card.Reading.Kana),
			RomajiHint: card.Reading.RomajiHint,
			Meaning:    card.Meaning,
			Damage:     1,
		})
		if len(chunks) == 3 {
			break
		}
	}
	if len(chunks) == 0 {
		chunks = append(chunks, boss.Chunk{
			ID:      "empty-boss",
			Text:    "日",
			Kana:    "ひ",
			Meaning: "day",
			Damage:  1,
		})
	}
	return chunks
}

func centerLine(text string, width int) string {
	text = atoms.TruncateDisplay(text, width)
	left := (width - atoms.DisplayWidth(text)) / 2
	right := width - left - atoms.DisplayWidth(text)
	if left < 0 {
		left = 0
	}
	if right < 0 {
		right = 0
	}
	return strings.Repeat(" ", left) + text + strings.Repeat(" ", right)
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
