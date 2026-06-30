package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/superposition/kotoba-line/internal/boss"
	"github.com/superposition/kotoba-line/internal/content"
	"github.com/superposition/kotoba-line/internal/game"
	"github.com/superposition/kotoba-line/internal/kana"
	"github.com/superposition/kotoba-line/internal/skilltree"
	statestore "github.com/superposition/kotoba-line/internal/state"
	"github.com/superposition/kotoba-line/internal/station"
	coretransition "github.com/superposition/kotoba-line/internal/transition"
	"github.com/superposition/kotoba-line/internal/tui/atoms"
)

const (
	starterLevelID      = "journal-2026-06-22-key-readings"
	constitutionLevelID = "constitution-preamble-1"
)

type screenMode string

const (
	modeDrill      screenMode = "drill"
	modeBoss       screenMode = "boss"
	modeStations   screenMode = "stations"
	modeTransition screenMode = "transition"
)

const (
	pointsWrongKey   = -5
	pointsHint       = -10
	pointsCleanHit   = 100
	pointsHintedHit  = 40
	pointsPerRune    = 5
	pointsComboBonus = 25
	pointsLevelClear = 250
	pointsUnlock     = 100
	pointsBossHit    = 250
)

type Options struct {
	Username        string
	Library         *content.Library
	StationCatalog  *station.Catalog
	EventStore      statestore.EventStore
	EventLogPath    string
	DisableEventLog bool
}

type levelOption struct {
	LevelID        string
	LevelTitle     string
	StationName    string
	Description    string
	CardCount      int
	RequiredPoints int
	MissingPoints  int
	Required       []content.Card
	Missing        []content.Card
	Locked         bool
	Complete       bool
}

type levelUnlock struct {
	ID    string
	Title string
}

type transitionSummary struct {
	Scene       coretransition.QueuedScene
	Frame       int
	Subject     string
	PointsDelta int
	TotalPoints int
	FromLevel   string
	ToLevel     string
	SetBefore   string
	SetAfter    string
	Unlocked    []levelUnlock
	Lines       []string
	ReturnMode  screenMode
}

type Model struct {
	username   string
	width      int
	height     int
	library    *content.Library
	stations   *station.Catalog
	skillTree  *skilltree.Tree
	mode       screenMode
	drill      game.Drill
	boss       boss.Fight
	levelID    string
	level      string
	hull       int
	combo      int
	score      int
	cursor     int
	input      string
	last       string
	hint       string
	bossHint   string
	hinted     map[string]bool
	transition transitionSummary
	loadErr    string
	logErr     string
	eventLog   statestore.EventStore
}

func New(opts Options) Model {
	username := strings.TrimSpace(opts.Username)
	if username == "" {
		username = "player"
	}

	library := opts.Library
	stations := opts.StationCatalog
	loadErr := ""
	if library == nil {
		loaded, report, err := content.LoadDefaultPlayableLibrary()
		if err != nil {
			loadErr = fmt.Sprintf("seed content unavailable: %v", err)
		} else if report.HasErrors() {
			loadErr = "seed content has validation errors"
		} else {
			library = loaded
		}
	}
	if stations == nil {
		loaded, report, err := loadStationCatalog()
		if err == nil && !report.HasErrors() {
			stations = loaded
		}
	}
	tree, err := skilltree.New(library)
	if err != nil {
		tree = nil
		if loadErr == "" {
			loadErr = err.Error()
		}
	}

	var eventLog statestore.EventStore
	if !opts.DisableEventLog {
		if opts.EventStore != nil {
			eventLog = opts.EventStore
		} else if opts.EventLogPath != "" {
			eventLog = statestore.NewEventLog(opts.EventLogPath)
		} else {
			eventLog = statestore.DefaultSQLiteEventStore(username)
		}
	}
	progress := statestore.NewProgress()
	logErr := ""
	if eventLog != nil && eventLog.Path() != "" {
		replayed, err := eventLog.Replay()
		if err != nil {
			logErr = err.Error()
		} else {
			progress = replayed
		}
	}

	initialLevelID := resumeLevelID(library, starterLevelID, progress)
	initialTitle := levelTitle(library, initialLevelID, "Tide Gate")
	model := Model{
		username:  username,
		library:   library,
		stations:  stations,
		skillTree: tree,
		mode:      modeDrill,
		drill:     newLevelDrill(library, initialLevelID, progress),
		boss:      boss.NewFight(newDocumentBoss(library)),
		levelID:   initialLevelID,
		level:     initialTitle,
		hull:      5,
		score:     progress.Points,
		loadErr:   loadErr,
		logErr:    logErr,
		eventLog:  eventLog,
	}
	if model.drill.DeckSize() == 0 && loadErr == "" {
		model = model.openStations()
	}
	return model
}

func (m Model) Init() tea.Cmd {
	return drillTick()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case drillTickMsg:
		if m.mode == modeDrill {
			m.drill = m.drill.Tick()
		} else if m.mode == modeTransition && m.transitionFrameCount() > 0 {
			m.transition.Frame = (m.transition.Frame + 1) % m.transitionFrameCount()
		}
		return m, drillTick()
	case tea.KeyMsg:
		if m.mode == modeTransition {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "enter", " ", "space", "esc":
				m = m.dismissTransition()
				return m, nil
			default:
				returnMode := m.transitionReturnMode()
				m = m.dismissTransition()
				if returnMode == modeDrill && msg.Type == tea.KeyRunes {
					return m.Update(msg)
				}
				return m, nil
			}
		}
		if m.mode == modeStations && msg.Type == tea.KeyRunes && msg.String() != "q" {
			m = m.handleStationRunes(msg.Runes)
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.last = "catch the wave"
		case "j":
			if m.mode == modeStations {
				m = m.moveStationCursor(1)
			} else {
				m = m.appendInputText(string(msg.Runes))
			}
		case "k":
			if m.mode == modeStations {
				m = m.moveStationCursor(-1)
			} else if msg.Type == tea.KeyRunes {
				m = m.appendInputText(string(msg.Runes))
			}
		case "up":
			if m.mode == modeStations {
				m = m.moveStationCursor(-1)
			}
		case "down":
			if m.mode == modeStations {
				m = m.moveStationCursor(1)
			}
		case "esc":
			if m.mode == modeStations || m.mode == modeBoss {
				m = m.returnToStream()
			} else if m.input == "" {
				m = m.openStations()
			} else {
				m.input = ""
				m.last = ""
			}
		case "enter":
			if m.mode == modeStations {
				m = m.selectStation()
			} else if m.mode == modeBoss {
				m = m.submitBossInput()
			} else {
				m = m.submitInput()
			}
		case "backspace", "ctrl+h":
			m.input = trimLastRune(m.input)
			if m.mode == modeDrill {
				m = m.updateDrillTypingFeedback()
			}
		case " ", "space":
			m = m.appendInputText(" ")
		case "?", "？":
			if m.mode == modeStations {
				m.last = "select with enter"
			} else if m.mode == modeBoss {
				m = m.showBossHint()
			} else {
				m = m.showHint()
			}
		default:
			if msg.Type == tea.KeyRunes {
				m = m.appendInputText(string(msg.Runes))
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	m = m.normalizeEmptyDrill()
	return m, nil
}

func (m Model) appendInputText(text string) Model {
	if text == "" {
		return m
	}
	if m.mode == modeDrill {
		return m.consumeDrillKeys(text)
	}
	m.input += text
	m.last = ""
	return m
}

func (m Model) consumeDrillKeys(text string) Model {
	for _, r := range text {
		beforeTarget, beforeOK := m.drill.Target()
		m = m.consumeDrillKey(r)
		afterTarget, afterOK := m.drill.Target()
		if strings.HasPrefix(m.last, "wipeout") {
			break
		}
		if beforeOK && (!afterOK || beforeTarget.ID != afterTarget.ID) {
			break
		}
	}
	return m
}

func (m Model) consumeDrillKey(r rune) Model {
	target, ok := m.drill.Target()
	if !ok {
		m.last = "waiting for wave"
		return m
	}
	sequence := kana.TypingSequence(target.Kana, target.RomajiHint)
	if sequence == "" {
		m.last = "no wave path"
		return m
	}
	if !strings.HasPrefix(sequence, m.input) {
		m.input = ""
	}

	key, ok := normalizedTypingKey(r)
	if !ok {
		m.addPoints(pointsWrongKey, "bad key")
		m.last = fmt.Sprintf("wipeout %s %s  [%s]", displayTypedKey(r), signedPoints(pointsWrongKey), nextTypingKey(sequence, m.input))
		return m
	}

	remaining := []rune(sequence[len(m.input):])
	if len(remaining) == 0 {
		return m.submitInput()
	}
	expected := remaining[0]
	if expected == ' ' && key != " " {
		m.input += " "
		remaining = []rune(sequence[len(m.input):])
		if len(remaining) == 0 {
			return m.submitInput()
		}
		expected = remaining[0]
	}

	if string(expected) != key {
		m.combo = 0
		m.addPoints(pointsWrongKey, "bad key")
		m.last = fmt.Sprintf("wipeout %s %s  [%s]", strings.ToUpper(keyLabel(key)), signedPoints(pointsWrongKey), nextTypingKey(sequence, m.input))
		return m
	}

	m.input += string(expected)
	m.combo++
	if m.input == sequence {
		return m.submitInput()
	}
	m.last = fmt.Sprintf("rad glide %s  [%s]", strings.ToUpper(keyLabel(key)), nextTypingKey(sequence, m.input))
	return m
}

func (m Model) updateDrillTypingFeedback() Model {
	target, ok := m.drill.Target()
	if !ok {
		m.last = ""
		return m
	}
	sequence := kana.TypingSequence(target.Kana, target.RomajiHint)
	if m.input == "" {
		m.last = ""
		return m
	}
	m.last = fmt.Sprintf("surf line  [%s]", nextTypingKey(sequence, m.input))
	return m
}

func normalizedTypingKey(r rune) (string, bool) {
	if r == ' ' {
		return " ", true
	}
	lowered := strings.ToLower(string(r))
	if lowered == "" {
		return "", false
	}
	key := []rune(lowered)[0]
	if (key >= 'a' && key <= 'z') || (key >= '0' && key <= '9') {
		return string(key), true
	}
	return "", false
}

func displayTypedKey(r rune) string {
	if r == ' ' {
		return "SPACE"
	}
	return strings.ToUpper(string(r))
}

func keyLabel(key string) string {
	if key == " " {
		return "space"
	}
	return key
}

func nextTypingKey(sequence, input string) string {
	if strings.HasPrefix(sequence, input) {
		remaining := []rune(sequence[len(input):])
		for _, r := range remaining {
			if r == ' ' {
				return "SPACE"
			}
			return strings.ToUpper(string(r))
		}
	}
	return "DONE"
}

func answerPreviewForTarget(targetKana, input string) string {
	answer := strings.TrimSpace(input)
	if preview := kana.PreviewForTarget(targetKana, input); preview != "" {
		return fmt.Sprintf("%s -> %s", answer, preview)
	}
	return answer
}

func (m Model) handleStationRunes(runes []rune) Model {
	for _, r := range runes {
		switch r {
		case 'j':
			m = m.moveStationCursor(1)
		case 'k':
			m = m.moveStationCursor(-1)
		case 's':
			m = m.openStations()
		case '?', '？':
			m.last = "select with enter"
		}
	}
	return m
}

func (m Model) View() string {
	m = m.normalizeEmptyDrill()
	width := m.cardWidth()
	if m.loadErr == "" {
		switch m.mode {
		case modeStations:
			return m.stationsCard(width)
		case modeBoss:
			return m.bossCard(width)
		case modeTransition:
			return m.transitionCard(width)
		default:
			return m.drillCard(width)
		}
	}

	document := m.documentTitleForLevel(m.levelID)
	lines := []string{
		"Kotoba Beach",
		"",
		fmt.Sprintf("Player: %s", m.username),
		"",
		fmt.Sprintf("Document: %s", document),
		fmt.Sprintf("Wave: %s", m.level),
	}

	lines = append(lines, "", m.loadErr)

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
	before := m.progress()
	beforePoints := m.score
	beforeLevelID := m.levelID
	beforeLevelTitle := m.level
	beforeSet := m.levelSetLine(before, m.levelID)
	if target, ok := m.drill.Target(); ok {
		sequence := kana.TypingSequence(target.Kana, target.RomajiHint)
		if sequence != "" && m.input != "" && m.input != sequence && !kana.MatchesAnswer(target.Kana, target.RomajiHint, m.input) {
			m.last = fmt.Sprintf("keep riding  [%s]", nextTypingKey(sequence, m.input))
			return m
		}
	}
	m.drill, result = m.drill.SubmitTargetKana(m.input)

	switch result.Status {
	case game.AnswerHit:
		m.input = ""
		m.hint = ""
		clean := !m.hinted[result.Enemy.CardID]
		beforeAvailable := m.availableLevelIDs(before)
		m.appendEvent(statestore.EnemyHitWithClean(result.Enemy.CardID, clean))
		delete(m.hinted, result.Enemy.CardID)
		m.combo++
		hitPoints := drillHitPoints(result.Enemy, clean, m.combo)
		m.addPoints(hitPoints, drillHitReason(clean))
		after := m.progress()
		unlocked := m.appendUnlocks(beforeAvailable, after)
		if len(unlocked) > 0 {
			m.addPoints(pointsUnlock*len(unlocked), "tree unlock")
			after = m.progress()
		}
		m.last = fmt.Sprintf("wave saved %s -> %s", result.Enemy.Text, result.Enemy.Kana)
		levelCleared := !levelComplete(m.library, beforeLevelID, before) && levelComplete(m.library, beforeLevelID, after)
		returnMode := modeDrill
		if levelCleared {
			m.addPoints(pointsLevelClear, "level clear")
			if next := m.nextOpenLevelID(); next != "" {
				title := levelTitle(m.library, next, next)
				m = m.switchLevel(next)
				m.last = fmt.Sprintf("lesson saved -> %s", title)
			} else {
				returnMode = modeStations
				m.last = "lesson saved -> route map"
			}
		} else if !before.Cards[result.Enemy.CardID].Mastered && after.Cards[result.Enemy.CardID].Mastered {
			m.last = fmt.Sprintf("saved %s", result.Enemy.Text)
			m.drill = newLevelDrill(m.library, m.levelID, after)
		} else if len(unlocked) > 0 {
			m.last = fmt.Sprintf("new break -> %s", unlocked[len(unlocked)-1].Title)
		}
		if !levelCleared && len(m.drill.Enemies()) == 0 {
			m.drill, _ = m.drill.Spawn()
		}
		m = m.beginClearTransition(clearTransition{
			CardText:      result.Enemy.Text,
			CardKana:      result.Enemy.Kana,
			PointsDelta:   m.score - beforePoints,
			FromLevelID:   beforeLevelID,
			FromLevel:     beforeLevelTitle,
			ToLevel:       m.level,
			BeforeSetLine: beforeSet,
			AfterSetLine:  m.levelSetLine(m.progress(), m.levelID),
			Unlocked:      unlocked,
			LevelCleared:  levelCleared,
			ReturnMode:    returnMode,
		})
	case game.AnswerMiss:
		if result.Enemy.CardID != "" {
			m.appendEvent(statestore.EnemyMissed(result.Enemy.CardID))
			m.combo = 0
			m.hull = clampInt(m.hull-1, 0, 5)
			m.last = fmt.Sprintf("wipeout %s  %s wants %s", answerPreviewForTarget(result.Enemy.Kana, result.Input), result.Enemy.Text, result.Enemy.Kana)
		} else {
			m.combo = 0
			m.hull = clampInt(m.hull-1, 0, 5)
			m.last = fmt.Sprintf("wipeout %s", result.Input)
		}
	default:
		m.last = "ready to surf"
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
	if m.hinted == nil {
		m.hinted = map[string]bool{}
	}
	m.hinted[hint.Enemy.CardID] = true
	m.addPoints(pointsHint, "hint")
	m.hint = fmt.Sprintf("hint: %s = %s (%s)", hint.Enemy.Text, hint.Enemy.Kana, hint.Romaji)
	m.last = fmt.Sprintf("hint tax %s", signedPoints(pointsHint))
	return m
}

func (m Model) openStations() Model {
	m.mode = modeStations
	m.input = ""
	m.hint = ""
	m.bossHint = ""
	m.hinted = nil
	options := m.levelOptions()
	m.cursor = m.stationFocusIndex(options)
	m.last = "ROUTE MAP"
	return m
}

func (m Model) stationFocusIndex(options []levelOption) int {
	if len(options) == 0 {
		return 0
	}
	current := indexLevelOption(options, m.levelID)
	if current < len(options) && !isKanaFoundationLevelID(options[current].LevelID) {
		for index, option := range options {
			if isKanaFoundationLevelID(option.LevelID) && !option.Locked && !option.Complete {
				return index
			}
		}
	}
	if current < len(options) && !options[current].Locked && !options[current].Complete {
		return current
	}
	for offset := 1; offset <= len(options); offset++ {
		index := (current + offset) % len(options)
		option := options[index]
		if !option.Locked && !option.Complete {
			return index
		}
	}
	return current
}

func (m Model) returnToStream() Model {
	m.input = ""
	m.hint = ""
	m.bossHint = ""
	m.hinted = nil
	if m.drill.DeckSize() > 0 {
		m.mode = modeDrill
		return m
	}
	if next := m.nextOpenLevelID(); next != "" {
		return m.switchLevel(next)
	}
	m = m.openStations()
	m.last = "all visible waves done"
	return m
}

func (m Model) normalizeEmptyDrill() Model {
	if m.mode != modeDrill || m.loadErr != "" || m.drill.DeckSize() > 0 {
		return m
	}
	if next := m.nextOpenLevelID(); next != "" {
		return m.switchLevel(next)
	}
	m = m.openStations()
	m.last = "all visible waves done"
	return m
}

func (m Model) moveStationCursor(delta int) Model {
	options := m.levelOptions()
	if len(options) == 0 {
		m.cursor = 0
		return m
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = len(options) - 1
	}
	if m.cursor >= len(options) {
		m.cursor = 0
	}
	return m
}

func (m Model) selectStation() Model {
	options := m.levelOptions()
	if len(options) == 0 {
		m.last = "no stations"
		return m
	}
	option := options[boundedIndex(m.cursor, len(options))]
	if option.Locked {
		m.last = fmt.Sprintf("LOCKED %s", option.LevelTitle)
		return m
	}
	if option.Complete {
		if next := m.nextOpenLevelID(); next != "" {
			return m.switchLevel(next)
		}
		m.last = fmt.Sprintf("DONE %s", option.LevelTitle)
		return m
	}
	return m.switchLevel(option.LevelID)
}

func (m Model) switchLevel(levelID string) Model {
	if m.library == nil {
		m.last = "level unavailable"
		return m
	}
	if option, ok := m.levelOption(levelID); ok && option.Locked {
		m.mode = modeStations
		m.cursor = indexLevelOption(m.levelOptions(), levelID)
		m.last = fmt.Sprintf("LOCKED %s", option.LevelTitle)
		return m
	}
	drill := newLevelDrill(m.library, levelID, m.progress())
	if drill.DeckSize() == 0 {
		m.last = "level unavailable"
		return m
	}
	m.mode = modeDrill
	m.drill = drill
	m.levelID = levelID
	m.level = levelTitle(m.library, levelID, levelID)
	m.cursor = indexLevelOption(m.levelOptions(), levelID)
	m.input = ""
	m.hint = ""
	m.bossHint = ""
	m.hinted = nil
	m.combo = 0
	m.last = fmt.Sprintf("wave %s", m.level)
	return m
}

func (m Model) enterBoss() Model {
	m.mode = modeBoss
	m.boss = boss.NewFight(newLevelBoss(m.library, m.levelID, m.level))
	m.input = ""
	m.hint = ""
	m.bossHint = ""
	m.hinted = nil
	m.combo = 0
	b := m.boss.Boss()
	m.appendEvent(statestore.BossIntro(b.ID))
	m.last = fmt.Sprintf("boss %s", b.Glyph)
	return m
}

func (m Model) submitBossInput() Model {
	var result boss.AnswerResult
	m.boss, result = m.boss.SubmitKana(m.input)
	preview := ""
	if result.Chunk.Kana != "" {
		preview = kana.PreviewForTarget(result.Chunk.Kana, m.input)
	}
	m.input = ""

	b := m.boss.Boss()
	switch result.Status {
	case boss.AnswerHit, boss.AnswerCleared:
		clean := m.bossHint == ""
		m.appendEvent(statestore.EnemyHitWithClean(result.Chunk.ID, clean))
		m.appendEvent(statestore.BossDamaged(b.ID, result.Chunk.ID))
		m.combo++
		m.addPoints(pointsBossHit+m.combo*pointsComboBonus, "boss crack")
		m.last = fmt.Sprintf("burst %s -%d", result.Chunk.Text, result.Damage)
		if result.Status == boss.AnswerCleared {
			m.appendEvent(statestore.BossCleared(b.ID))
			m.last = fmt.Sprintf("clear %s", b.Title)
			if next := m.nextOpenLevelID(); next != "" {
				m = m.switchLevel(next)
			}
		}
	case boss.AnswerMiss:
		if result.Chunk.ID != "" {
			m.appendEvent(statestore.EnemyMissed(result.Chunk.ID))
		}
		input := result.Input
		if preview != "" {
			input = fmt.Sprintf("%s -> %s", result.Input, preview)
		}
		m.combo = 0
		m.hull = clampInt(m.hull-1, 0, 5)
		m.last = fmt.Sprintf("miss %s", input)
	default:
		m.last = "ready"
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
	m.addPoints(pointsHint, "boss hint")
	m.bossHint = fmt.Sprintf("hint: %s = %s", target.Text, target.RomajiHint)
	m.last = fmt.Sprintf("hint tax %s", signedPoints(pointsHint))
	return m
}

func (m *Model) appendEvent(event statestore.Event) {
	if m.eventLog == nil || m.eventLog.Path() == "" {
		return
	}
	if err := m.eventLog.Append(event); err != nil {
		m.logErr = err.Error()
	}
}

func (m *Model) addPoints(delta int, reason string) {
	if delta == 0 {
		return
	}
	m.score = clampInt(m.score+delta, 0, 99999)
	m.appendEvent(statestore.Points(delta, reason))
}

func (m *Model) appendUnlocks(beforeAvailable map[string]bool, after statestore.Progress) []levelUnlock {
	if m.library == nil {
		return nil
	}
	unlocked := make([]levelUnlock, 0)
	for _, level := range m.library.Levels {
		if len(level.RequiredCardIDs) == 0 {
			continue
		}
		if beforeAvailable[level.ID] || after.UnlockedLevels[level.ID] || !levelAvailable(level, after) {
			continue
		}
		m.appendEvent(statestore.LevelUnlocked(level.ID))
		unlocked = append(unlocked, levelUnlock{
			ID:    level.ID,
			Title: firstNonBlank(level.Title, level.ID),
		})
	}
	return unlocked
}

type clearTransition struct {
	CardText      string
	CardKana      string
	PointsDelta   int
	FromLevelID   string
	FromLevel     string
	ToLevel       string
	BeforeSetLine string
	AfterSetLine  string
	Unlocked      []levelUnlock
	LevelCleared  bool
	ReturnMode    screenMode
}

func (m Model) beginClearTransition(clear clearTransition) Model {
	returnMode := clear.ReturnMode
	if returnMode == "" {
		returnMode = modeDrill
	}

	feedback := fmt.Sprintf("saved %s %s", clear.CardText, signedPoints(clear.PointsDelta))
	if clear.LevelCleared && clear.ToLevel != "" && clear.FromLevel != clear.ToLevel {
		feedback = "lesson clear -> " + clear.ToLevel
	} else if clear.LevelCleared && returnMode == modeStations {
		feedback = "lesson clear -> route map"
	}
	for _, unlock := range clear.Unlocked {
		feedback += "  unlock " + unlock.Title
	}
	if returnMode == modeStations {
		m = m.openStations()
		m.last = feedback
		return m
	} else {
		m.mode = returnMode
	}
	m.transition = transitionSummary{}
	m.last = feedback
	return m
}

func (m Model) transitionReturnMode() screenMode {
	if m.transition.ReturnMode == "" {
		return modeDrill
	}
	return m.transition.ReturnMode
}

func (m Model) dismissTransition() Model {
	returnMode := m.transitionReturnMode()
	m.transition = transitionSummary{}
	if returnMode == modeStations {
		return m.openStations()
	}
	m.mode = returnMode
	return m
}

func (m Model) drillCard(width int) string {
	progress := m.progress()
	return strings.Join(m.branchRunLines(progress, width, false), "\n")
}

func (m Model) transitionCard(width int) string {
	if width < 44 {
		width = 44
	}
	bodyWidth := width - 4
	frame := coretransition.Frame{ID: "summary", Lines: []string{"~~~~~", "tree signal", "next wave"}}
	title := "TREE SURF"
	subtitle := "clear"
	if len(m.transition.Scene.Frames()) > 0 {
		frames := m.transition.Scene.Frames()
		frame = frames[m.transition.Frame%len(frames)]
		title = m.transition.Scene.Definition.Title
		subtitle = frame.ID
	}

	body := []string{
		atoms.StripANSI(atoms.DitherLine(bodyWidth, m.transition.Frame*2)),
	}
	body = append(body, frame.Lines...)
	body = append(body, "")
	body = append(body, m.transition.Lines...)

	return atoms.Card(atoms.CardSpec{
		Title:     title,
		Subtitle:  subtitle,
		Body:      body,
		Width:     width,
		Highlight: true,
	})
}

func (m Model) transitionFrameCount() int {
	if len(m.transition.Scene.Frames()) == 0 {
		return 0
	}
	return len(m.transition.Scene.Frames())
}

func (m Model) exerciseLines(progress statestore.Progress, width int, compact bool) []string {
	body := []string{
		fmt.Sprintf("POINTS %05d  TIDE %s  STREAK x%02d", m.score, hullMeter(m.hull, 5), m.combo),
		fmt.Sprintf("HIT %02d  WIPE %02d  HINT %02d", m.drill.Hits(), m.drill.Misses(), m.drill.Hints()),
		"ride kana  ?/？ hint  q quit",
		m.waveProgressLine(progress),
	}

	enemies := m.drill.Enemies()
	previewTarget := ""
	if len(enemies) == 0 {
		body = append(body, "NODE --")
	} else {
		target := enemies[0]
		previewTarget = target.Kana
		body = append(body,
			fmt.Sprintf("NODE %s  %s", target.Text, target.Meaning),
			m.masteryLine(target.CardID, progress, exerciseWidth(width)),
		)
		if len(enemies) > 1 {
			if compact {
				body = append(body, fmt.Sprintf("QUEUE +%d", len(enemies)-1))
			} else {
				body = append(body, "QUEUE")
				for _, enemy := range enemies[1:] {
					body = append(body, fmt.Sprintf("  %s  %s", enemy.Text, enemy.Meaning))
				}
			}
		}
	}

	nextLines := m.nextRouteLines(progress)
	if compact && len(nextLines) > 2 {
		nextLines = nextLines[:2]
	}
	body = append(body, nextLines...)
	body = append(body, atoms.StripANSI(atoms.InputBar("type", m.input, exerciseWidth(width), true)))
	body = append(body, fmt.Sprintf("sound   %s", kana.PreviewForTarget(previewTarget, m.input)))
	body = append(body, fmt.Sprintf("hint    %s", strings.TrimPrefix(m.hint, "hint: ")))
	if m.logErr != "" {
		body = append(body, "log: "+m.logErr)
	} else {
		body = append(body, "log     ")
	}
	body = append(body, fmt.Sprintf("status  %s", m.last))
	return body
}

func (m Model) branchRunLines(progress statestore.Progress, width int, bossMode bool) []string {
	if width < 40 {
		width = 40
	}
	if !bossMode {
		return m.promptSquareLines(width)
	}
	lines := []string{
		styledBranchLine(width, m.hudStatusLine(width, bossMode), atoms.Style{Fg: atoms.Cyan, Bold: true}),
		branchLine(width, ""),
	}
	lines = append(lines, m.activeExerciseHUDLines(progress, width, bossMode)...)
	if m.logErr != "" {
		lines = append(lines, branchLine(width, ""), styledBranchLine(width, "log "+m.logErr, atoms.Style{Fg: atoms.Coral}))
	}
	return lines
}

func (m Model) hudStatusLine(width int, bossMode bool) string {
	mode := "SURF"
	if bossMode {
		mode = "BOSS BREAK"
	}
	return fmt.Sprintf("KOTOBA BEACH  %s  |  %s  |  points %05d  tide %s  streak x%02d", m.username, mode, m.score, hullMeter(m.hull, 5), m.combo)
}

func (m Model) activeExerciseHUDLines(progress statestore.Progress, width int, bossMode bool) []string {
	if bossMode {
		return m.bossHUDLines(width)
	}
	target, ok := m.drill.Target()
	if !ok {
		return hudPanel("SURF RUN", []string{
			"goal    waiting for the next wave",
			atoms.StripANSI(atoms.InputBar("keys", m.input, width-4, false)),
		}, width)
	}
	targetPrefix := "target  "
	soundPrefix := "sound   "
	if strings.HasPrefix(m.last, "wipeout") {
		targetPrefix = "target! "
		soundPrefix = "sound!  "
	}
	sequence := kana.TypingSequence(target.Kana, target.RomajiHint)
	body := []string{
		"goal    catch the kana wave",
		targetPrefix + targetProgressText(target.Text, target.Kana, m.input),
		soundPrefix + soundCue(target.Kana, sequence, m.input),
		"meaning " + target.Meaning,
	}
	body = append(body,
		typingProgressLine(sequence, m.input, width-4),
		atoms.StripANSI(atoms.InputBar("keys", m.input, width-4, true)),
	)
	if m.hint != "" {
		body = append(body, "hint    "+strings.TrimPrefix(m.hint, "hint: "))
	} else {
		body = append(body, "hint    ? for kana/romaji")
	}
	body = append(body,
		"feedback "+firstNonBlank(m.last, "ready to surf"),
		m.waveProgressLine(progress),
		m.masteryLine(target.CardID, progress, width-4),
	)
	if card, ok := m.cardByID(target.CardID); ok {
		body = append(body, sentenceTreeLines(card)...)
	}
	if !m.compactViewport() {
		body = append(body, fmt.Sprintf("stats   hit %02d  wipe %02d  hint %02d", m.drill.Hits(), m.drill.Misses(), m.drill.Hints()))
	}
	return hudPanel("SURF RUN", body, width)
}

func (m Model) promptSquareLines(width int) []string {
	target, ok := m.drill.Target()
	boxWidth := clampInt(width-8, 36, 56)
	if !ok {
		body := promptSquareBody([]string{
			"goal",
			"waiting for the next wave",
			"",
			atoms.StripANSI(atoms.InputBar("keys", m.input, boxWidth-4, false)),
		}, boxWidth-4)
		return centerBlock(hudPanel("KOTOBA BEACH", body, boxWidth), width)
	}

	sequence := kana.TypingSequence(target.Kana, target.RomajiHint)
	targetText := targetProgressText(target.Text, target.Kana, m.input)
	feedback := firstNonBlank(m.last, "ready to surf")
	if strings.HasPrefix(m.last, "wipeout") {
		feedback = m.last
	}

	body := []string{
		"goal    catch the kana wave",
		"",
		"target  " + targetText,
		"",
	}
	body = append(body, prefixedCenterWrappedText("meaning ", target.Meaning, boxWidth-4)...)
	body = append(body,
		"",
		"sound   "+soundCue(target.Kana, sequence, m.input),
		atoms.StripANSI(atoms.InputBar("keys", m.input, boxWidth-4, true)),
	)
	if m.hint != "" {
		body = append(body, prefixedCenterWrappedText("hint    ", strings.TrimPrefix(m.hint, "hint: "), boxWidth-4)...)
	} else {
		body = append(body, "hint    ? for kana/romaji")
	}
	body = append(body, "feedback "+feedback)
	if m.logErr != "" {
		body = append(body, "log "+m.logErr)
	}
	body = promptSquareBody(body, boxWidth-4)
	lines := centerBlock(hudPanel("KOTOBA BEACH", body, boxWidth), width)
	if m.height > 0 {
		topPadding := (m.height - len(lines)) / 2
		if topPadding > 0 {
			padding := make([]string, topPadding)
			lines = append(padding, lines...)
		}
	}
	return lines
}

func promptSquareBody(lines []string, inner int) []string {
	body := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			body = append(body, "")
			continue
		}
		body = append(body, centerLine(line, inner))
	}
	for len(body) < 13 {
		body = append(body, "")
	}
	return body
}

func centerWrappedText(text string, width int) []string {
	chunks := wrapDisplayText(text, width, "")
	if len(chunks) == 0 {
		return []string{""}
	}
	for i, chunk := range chunks {
		chunks[i] = centerLine(chunk, width)
	}
	return chunks
}

func prefixedCenterWrappedText(prefix, text string, width int) []string {
	available := width - atoms.DisplayWidth(prefix)
	if available < 12 {
		return centerWrappedText(prefix+text, width)
	}
	chunks := wrapDisplayText(text, available, "")
	if len(chunks) == 0 {
		return []string{centerLine(prefix, width)}
	}
	lines := make([]string, 0, len(chunks))
	for index, chunk := range chunks {
		linePrefix := prefix
		if index > 0 {
			linePrefix = strings.Repeat(" ", atoms.DisplayWidth(prefix))
		}
		lines = append(lines, centerLine(linePrefix+chunk, width))
	}
	return lines
}

func centerBlock(lines []string, width int) []string {
	centered := make([]string, 0, len(lines))
	for _, line := range lines {
		visible := atoms.DisplayWidth(line)
		if visible >= width {
			centered = append(centered, line)
			continue
		}
		centered = append(centered, strings.Repeat(" ", (width-visible)/2)+line)
	}
	return centered
}

func targetSoundProgress(targetKana, input string) string {
	typed := input
	for typed != "" {
		if preview := kana.PreviewForTarget(targetKana, typed); preview != "" {
			return prefixRunes(targetKana, len([]rune(preview)))
		}
		typed = trimLastRune(typed)
	}
	return ""
}

func soundCue(targetKana, sequence, input string) string {
	progress := targetSoundProgress(targetKana, input)
	key := "[" + nextTypingKey(sequence, input) + "]"
	if progress == "" {
		return key
	}
	return progress + " " + key
}

func targetProgressText(targetText, targetKana, input string) string {
	progress := targetSoundProgress(targetKana, input)
	if progress == "" {
		return targetText
	}
	done := len([]rune(progress))
	runes := []rune(targetKana)
	if len(runes) == 0 {
		return targetText
	}
	done = clampInt(done, 0, len(runes))
	return "[" + string(runes[:done]) + "]" + string(runes[done:])
}

func prefixRunes(value string, count int) string {
	if count <= 0 {
		return ""
	}
	runes := []rune(value)
	if count > len(runes) {
		count = len(runes)
	}
	return string(runes[:count])
}

func typingProgressLine(sequence, input string, width int) string {
	total := len([]rune(sequence))
	done := len([]rune(input))
	if total <= 0 {
		return "wave    --"
	}
	done = clampInt(done, 0, total)
	label := "wave    "
	trail := fmt.Sprintf(" %02d/%02d", done, total)
	barWidth := width - atoms.DisplayWidth(label) - atoms.DisplayWidth(trail) - 2
	if barWidth < 6 {
		barWidth = 6
	}
	if barWidth > 42 {
		barWidth = 42
	}
	filled := done * barWidth / total
	return label + "[" + strings.Repeat("#", filled) + strings.Repeat(".", barWidth-filled) + "]" + trail
}

func (m Model) hudShelfLines(progress statestore.Progress, width int, compact bool) []string {
	document := m.documentShelfLines(compact)
	tree := m.treeShelfLines(progress, compact)
	if !compact && width >= 78 {
		lines := []string{branchLine(width, "SHELVES")}
		lines = append(lines, twoColumnLines(document, tree, width)...)
		if m.logErr != "" {
			lines = append(lines, branchLine(width, "log "+m.logErr))
		}
		return lines
	}

	lines := []string{branchLine(width, "SHELVES")}
	for _, line := range document {
		lines = append(lines, branchLine(width, line))
	}
	for _, line := range tree {
		lines = append(lines, branchLine(width, line))
	}
	if m.logErr != "" {
		lines = append(lines, branchLine(width, "log "+m.logErr))
	}
	return lines
}

func (m Model) documentShelfLines(compact bool) []string {
	lines := []string{
		"DOCUMENT",
		"doc    " + m.documentTitleForLevel(m.levelID),
		"level  " + m.level,
	}
	if campaign := m.campaignTitleForLevel(m.levelID); campaign != "" {
		lines = append(lines, "path   "+campaign)
	}
	if compact {
		return lines
	}
	if description := m.levelDescription(m.levelID); description != "" {
		lines = append(lines, "focus  "+description)
	}
	return lines
}

func (m Model) treeShelfLines(progress statestore.Progress, compact bool) []string {
	lines := []string{"TREE"}
	levels := m.branchLevels(progress, compact)
	if len(levels) == 0 {
		lines = append(lines, "no route loaded")
		return lines
	}
	currentFound := false
	for _, level := range levels {
		if level.ID == m.levelID {
			currentFound = true
			break
		}
	}
	for index, level := range levels {
		state := branchLevelStatus(level)
		mastered, total, training := branchLevelCounts(level)
		prefix := "  "
		if level.ID == m.levelID || (!currentFound && index == 0) {
			prefix = "=>"
		}
		title := firstNonBlank(m.stationNameForLevel(level.ID), level.Title, level.ID)
		lines = append(lines, fmt.Sprintf("%s %s  %s  %d/%d", prefix, title, state, mastered, total))
		if !compact && training > 0 {
			lines = append(lines, fmt.Sprintf("   charging %d", training))
		}
		if compact && len(lines) >= 4 {
			break
		}
	}

	nextLines := m.nextRouteLines(progress)
	if len(nextLines) > 0 {
		lines = append(lines, strings.ToLower(nextLines[0]))
	}
	if len(nextLines) > 1 {
		lines = append(lines, strings.TrimSpace(nextLines[1]))
	}
	return lines
}

func (m Model) renderedGraphLines(levels []skilltree.LevelState, progress statestore.Progress, width int, bossMode bool, currentIndex int, currentFound bool) []string {
	lines := []string{}
	for index, level := range levels {
		state := branchLevelStatus(level)
		title := firstNonBlank(m.stationNameForLevel(level.ID), level.Title, level.ID)
		mastered, total, training := branchLevelCounts(level)
		active := level.ID == m.levelID || (!currentFound && index == currentIndex)
		label := fmt.Sprintf("%s | %s | %d/%d", title, state, mastered, total)
		if active {
			label = "CURRENT | " + label
		}
		lines = append(lines, renderBox(label, width, "  ")...)
		if state == "locked" {
			lines = append(lines, m.renderedRequirementLines(level.ID, progress, width)...)
		} else if training > 0 {
			lines = append(lines, branchLine(width, fmt.Sprintf("      charging %d", training)))
		}
		if index < len(levels)-1 {
			lines = append(lines, "        |", "        v")
		}
	}
	return lines
}

func (m Model) compactRenderedGraphLines(levels []skilltree.LevelState, progress statestore.Progress, width int, bossMode bool, currentIndex int, currentFound bool) []string {
	lines := []string{}
	for index, level := range levels {
		state := branchLevelStatus(level)
		title := firstNonBlank(m.stationNameForLevel(level.ID), level.Title, level.ID)
		mastered, total, _ := branchLevelCounts(level)
		active := level.ID == m.levelID || (!currentFound && index == currentIndex)
		prefix := "  "
		if active {
			prefix = "=>"
		}
		lines = append(lines, branchLine(width, fmt.Sprintf("%s [%s | %s | %d/%d]", prefix, title, state, mastered, total)))
		if !active && state == "locked" {
			lines = append(lines, m.compactNeedLine(level.ID, progress, width))
		}
	}
	return lines
}

func (m Model) bossHUDLines(width int) []string {
	target, ok := m.boss.Target()
	b := m.boss.Boss()
	phase := m.boss.Phase()
	body := []string{
		"goal    crack the boss wave with kana",
		"boss    " + b.Title + " / " + phase.Title,
		atoms.StripANSI(atoms.HPBar(m.boss.HP(), b.HP, width-4)),
	}
	if ok {
		body = append(body,
			"target  "+target.Text,
			"meaning "+target.Meaning,
			atoms.StripANSI(atoms.InputBar("answer", m.input, width-4, true)),
		)
	} else {
		body = append(body,
			"target  gate cleared",
			atoms.StripANSI(atoms.InputBar("answer", m.input, width-4, false)),
		)
	}
	previewTarget := ""
	if ok {
		previewTarget = target.Kana
	}
	if preview := kana.PreviewForTarget(previewTarget, m.input); preview != "" {
		body = append(body, "sound   "+preview)
	}
	if m.bossHint != "" {
		body = append(body, "hint    "+strings.TrimPrefix(m.bossHint, "hint: "))
	} else if ok {
		body = append(body, "hint    ? for kana/romaji")
	}
	body = append(body, "feedback "+firstNonBlank(m.last, "ready to surf"))
	return hudPanel("BOSS BREAK", body, width)
}

func (m Model) renderedQuestionLines(width int, bossMode bool) []string {
	question, answer, ok := m.questionAndAnswer(bossMode)
	if !ok {
		return renderBox("waiting for study item", width, "       ")
	}
	lines := []string{
		branchLine(width, "        v"),
	}
	lines = append(lines, renderBox("Q: type kana reading for "+question, width, "       ")...)
	if answer != "" {
		lines = append(lines, branchLine(width, "       |"))
		lines = append(lines, branchLine(width, "       v"))
		lines = append(lines, renderBox(answer, width, "       ")...)
	}
	return lines
}

func (m Model) compactQuestionLines(width int, bossMode bool) []string {
	question, answer, ok := m.questionAndAnswer(bossMode)
	if !ok {
		return []string{branchLine(width, "      -> waiting for study item")}
	}
	lines := []string{branchLine(width, "      -> Q: type kana reading for "+question)}
	if answer != "" {
		lines = append(lines, branchLine(width, "         "+answer))
	}
	return lines
}

func (m Model) questionAndAnswer(bossMode bool) (string, string, bool) {
	if bossMode {
		target, ok := m.boss.Target()
		if !ok {
			return "boss cleared", "", true
		}
		answer := "? hint reveals answer"
		if m.bossHint != "" {
			answer = strings.TrimPrefix(m.bossHint, "hint: ")
		}
		return target.Text, answer, true
	}
	target, ok := m.drill.Target()
	if !ok {
		return "", "", false
	}
	answer := "? hint reveals kana/romaji"
	if m.hint != "" {
		answer = strings.TrimPrefix(m.hint, "hint: ")
	}
	enemies := m.drill.Enemies()
	if len(enemies) > 1 {
		queue := make([]string, 0, len(enemies)-1)
		for _, enemy := range enemies[1:] {
			queue = append(queue, enemy.Text)
		}
		answer += "  next: " + strings.Join(queue, ", ")
	}
	return target.Text, answer, true
}

func (m Model) renderedRequirementLines(levelID string, progress statestore.Progress, width int) []string {
	option, ok := m.levelOption(levelID)
	if !ok || len(option.Missing) == 0 {
		return nil
	}
	lines := []string{}
	for index, requirement := range option.Missing {
		if index == 2 {
			lines = append(lines, branchLine(width, fmt.Sprintf("      needs +%d more", len(option.Missing)-index)))
			break
		}
		card := progress.Cards[requirement.ID]
		lines = append(lines, branchLine(width, fmt.Sprintf("      needs %s/%s %d/%d", requirement.Text, requirement.Reading.Kana, cleanStreak(card), statestore.MasteryCleanHitStreak)))
	}
	return lines
}

func (m Model) compactNeedLine(levelID string, progress statestore.Progress, width int) string {
	option, ok := m.levelOption(levelID)
	if !ok || len(option.Missing) == 0 {
		return ""
	}
	requirement := option.Missing[0]
	card := progress.Cards[requirement.ID]
	suffix := ""
	if len(option.Missing) > 1 {
		suffix = fmt.Sprintf(" (+%d)", len(option.Missing)-1)
	}
	return branchLine(width, fmt.Sprintf("      needs %s/%s %d/%d%s", requirement.Text, requirement.Reading.Kana, cleanStreak(card), statestore.MasteryCleanHitStreak, suffix))
}

func (m Model) activeBranchNodeLines(progress statestore.Progress, width int, bossMode bool) []string {
	if bossMode {
		return m.bossBranchNodeLines(width)
	}
	compact := m.compactViewport()
	lines := []string{}
	if !compact {
		lines = append(lines,
			branchLine(width, fmt.Sprintf(" |   hits %02d   miss %02d   hint %02d", m.drill.Hits(), m.drill.Misses(), m.drill.Hints())),
			branchLine(width, fmt.Sprintf(" |   %s", strings.ToLower(m.waveProgressLine(progress)))),
		)
	}
	enemies := m.drill.Enemies()
	previewTarget := ""
	if len(enemies) == 0 {
		lines = append(lines, branchLine(width, " |   exercise --"))
	} else {
		target := enemies[0]
		previewTarget = target.Kana
		lines = append(lines,
			branchLine(width, fmt.Sprintf(" |   exercise: %s  %s", target.Text, target.Meaning)),
			branchLine(width, " |   task: type the kana reading"),
			branchLine(width, " |   "+m.masteryLine(target.CardID, progress, width-6)),
		)
		if len(enemies) > 1 {
			queued := make([]string, 0, len(enemies)-1)
			for _, enemy := range enemies[1:] {
				queued = append(queued, enemy.Text)
			}
			lines = append(lines, branchLine(width, " |   queued:   "+strings.Join(queued, " -> ")))
		}
	}
	lines = append(lines, branchLine(width, " |   "+atoms.StripANSI(atoms.InputBar("type", m.input, width-10, true))))
	if preview := kana.PreviewForTarget(previewTarget, m.input); preview != "" || !compact {
		lines = append(lines, branchLine(width, fmt.Sprintf(" |   sound %s", preview)))
	}
	hint := strings.TrimPrefix(m.hint, "hint: ")
	if hint != "" || !compact {
		lines = append(lines, branchLine(width, fmt.Sprintf(" |   hint  %s", hint)))
	}
	if m.logErr != "" {
		lines = append(lines, branchLine(width, " |   log   "+m.logErr))
	}
	lines = append(lines, branchLine(width, fmt.Sprintf(" |   status %s", m.last)))
	return lines
}

func (m Model) branchRequirementLines(levelID string, progress statestore.Progress, width int) []string {
	option, ok := m.levelOption(levelID)
	if !ok || len(option.Missing) == 0 {
		return nil
	}
	lines := []string{}
	for index, requirement := range option.Missing {
		if index == 2 {
			lines = append(lines, branchLine(width, fmt.Sprintf(" |   needs +%d more", len(option.Missing)-index)))
			break
		}
		card := progress.Cards[requirement.ID]
		lines = append(lines, branchLine(width, fmt.Sprintf(" |   needs %s/%s %d/%d", requirement.Text, requirement.Reading.Kana, cleanStreak(card), statestore.MasteryCleanHitStreak)))
	}
	return lines
}

func (m Model) branchNextLines(progress statestore.Progress, width int, compact bool) []string {
	for _, option := range m.levelOptions() {
		if !option.Locked || len(option.Missing) == 0 {
			continue
		}
		name := firstNonBlank(option.StationName, option.LevelTitle)
		lines := []string{branchLine(width, "blocked branch: "+name)}
		for index, requirement := range option.Missing {
			if compact && index == 1 {
				lines = append(lines, branchLine(width, fmt.Sprintf("  +%d more locks", len(option.Missing)-index)))
				break
			}
			if index == 3 {
				lines = append(lines, branchLine(width, fmt.Sprintf("  +%d more locks", len(option.Missing)-index)))
				break
			}
			card := progress.Cards[requirement.ID]
			lines = append(lines, branchLine(width, fmt.Sprintf("  %s/%s %d/%d", requirement.Text, requirement.Reading.Kana, cleanStreak(card), statestore.MasteryCleanHitStreak)))
		}
		return lines
	}
	return []string{branchLine(width, "no blocked branches")}
}

func (m Model) bossBranchNodeLines(width int) []string {
	b := m.boss.Boss()
	phase := m.boss.Phase()
	lines := []string{
		branchLine(width, fmt.Sprintf(" |   boss %s", b.Title)),
		branchLine(width, fmt.Sprintf(" |   phase %s  %s", phase.Title, phase.Glyph)),
		branchLine(width, " |   "+atoms.HPBar(m.boss.HP(), b.HP, width-10)),
		branchLine(width, " |   esc wave  ? hint  q quit"),
	}
	previewTarget := ""
	if target, ok := m.boss.Target(); ok {
		previewTarget = target.Kana
		lines = append(lines,
			branchLine(width, fmt.Sprintf(" |   weak point %s", target.Text)),
			branchLine(width, fmt.Sprintf(" |   meaning    %s", target.Meaning)),
			branchLine(width, " |   "+atoms.StripANSI(atoms.InputBar("type", m.input, width-10, true))),
		)
	} else {
		lines = append(lines,
			branchLine(width, " |   boss cleared"),
			branchLine(width, " |   "+atoms.StripANSI(atoms.InputBar("type", m.input, width-10, false))),
		)
	}
	if preview := kana.PreviewForTarget(previewTarget, m.input); preview != "" {
		lines = append(lines, branchLine(width, fmt.Sprintf(" |   sound %s", preview)))
	}
	if m.bossHint != "" {
		lines = append(lines, branchLine(width, " |   "+m.bossHint))
	}
	if m.logErr != "" {
		lines = append(lines, branchLine(width, " |   log "+m.logErr))
	}
	if m.last != "" {
		lines = append(lines, branchLine(width, " |   "+m.last))
	}
	return lines
}

func (m Model) branchLevels(progress statestore.Progress, compact bool) []skilltree.LevelState {
	if m.skillTree == nil {
		return nil
	}
	levels := m.skillTree.Levels(skilltree.StateProgress{Progress: progress})
	if !compact || len(levels) <= 4 {
		return levels
	}
	current := 0
	for index, level := range levels {
		if level.ID == m.levelID {
			current = index
			break
		}
	}
	start := current - 1
	if start < 0 {
		start = 0
	}
	end := start + 3
	if end > len(levels) {
		end = len(levels)
		start = end - 3
		if start < 0 {
			start = 0
		}
	}
	return levels[start:end]
}

func branchLevelStatus(level skilltree.LevelState) string {
	if len(level.Requirements) == 0 {
		return "open"
	}
	for _, requirement := range level.Requirements {
		if !requirement.Mastered {
			return "locked"
		}
	}
	return "open"
}

func branchLevelCounts(level skilltree.LevelState) (mastered int, total int, training int) {
	total = len(level.Cards)
	for _, card := range level.Cards {
		switch card.Status {
		case skilltree.StatusMastered:
			mastered++
		case skilltree.StatusTraining:
			training++
		}
	}
	return mastered, total, training
}

func branchLine(width int, text string) string {
	return atoms.TruncateDisplay(text, width)
}

func styledBranchLine(width int, text string, style atoms.Style) string {
	return style.Apply(branchLine(width, text))
}

func hudPanel(title string, body []string, width int) []string {
	if width < 40 {
		width = 40
	}
	inner := width - 4
	label := " " + strings.ToUpper(strings.TrimSpace(title)) + " "
	top := "+" + atoms.FitDisplay("--"+label+strings.Repeat("-", width), width-2) + "+"
	bottom := "+" + strings.Repeat("-", width-2) + "+"

	frame := atoms.Cyan
	lines := []string{atoms.Paint(frame, top)}
	for _, line := range body {
		for _, wrapped := range wrapHUDPanelLine(line, inner) {
			lines = append(lines, hudPanelLine(wrapped, inner, frame))
		}
	}
	if len(body) == 0 {
		lines = append(lines, hudPanelLine("", inner, frame))
	}
	lines = append(lines, atoms.Paint(frame, bottom))
	return lines
}

func wrapHUDPanelLine(line string, inner int) []string {
	line = atoms.StripANSI(line)
	if inner <= 0 || atoms.DisplayWidth(line) <= inner {
		return []string{line}
	}

	prefix := hudWrapPrefix(line)
	if prefix == "" {
		return wrapDisplayText(line, inner, "")
	}
	available := inner - atoms.DisplayWidth(prefix)
	if available < 12 {
		return wrapDisplayText(line, inner, "")
	}
	text := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if text == "" {
		return []string{line}
	}

	chunks := wrapDisplayChunks(text, available)
	wrapped := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		wrapped = append(wrapped, prefix+chunk)
	}
	return wrapped
}

func hudWrapPrefix(line string) string {
	for _, prefix := range []string{
		"feedback ",
		"wave    ",
		"tree particle ",
		"tree sentence ",
		"tree subject  ",
		"tree adverb   ",
		"tree thanks   ",
		"tree polite   ",
		"tree place    ",
		"tree time     ",
		"tree topic    ",
		"tree verb     ",
		"tree note     ",
		"meaning ",
		"target  ",
		"target! ",
		"hint    ",
		"goal    ",
		"sound   ",
		"sound!  ",
		"set     ",
		"stats   ",
		"boss    ",
	} {
		if strings.HasPrefix(line, prefix) {
			return prefix
		}
	}
	return ""
}

func wrapDisplayText(text string, width int, prefix string) []string {
	chunks := wrapDisplayChunks(text, width-atoms.DisplayWidth(prefix))
	wrapped := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		wrapped = append(wrapped, prefix+chunk)
	}
	return wrapped
}

func wrapDisplayChunks(text string, width int) []string {
	text = strings.TrimSpace(text)
	if width <= 0 || text == "" {
		return []string{text}
	}

	var chunks []string
	for atoms.DisplayWidth(text) > width {
		head, tail := splitDisplayChunk(text, width)
		if strings.TrimSpace(head) == "" {
			break
		}
		chunks = append(chunks, strings.TrimSpace(head))
		text = strings.TrimSpace(tail)
	}
	if text != "" {
		chunks = append(chunks, text)
	}
	return chunks
}

func splitDisplayChunk(text string, width int) (string, string) {
	used := 0
	cut := 0
	lastSpace := -1
	lastSpaceWidth := 0
	for index, r := range text {
		rw := atoms.DisplayWidth(string(r))
		if used+rw > width {
			break
		}
		used += rw
		cut = index + len(string(r))
		if r == ' ' {
			lastSpace = cut
			lastSpaceWidth = used
		}
	}
	if cut <= 0 {
		_, size := utf8.DecodeRuneInString(text)
		if size <= 0 {
			return text, ""
		}
		return text[:size], text[size:]
	}
	if lastSpace > 0 && lastSpace < len(text) && lastSpaceWidth >= width/2 {
		return text[:lastSpace], text[lastSpace:]
	}
	return text[:cut], text[cut:]
}

func hudPanelLine(line string, inner int, frame atoms.Color) string {
	return atoms.Paint(frame, "| ") + hudLineStyle(line).Apply(atoms.FitDisplay(line, inner)) + atoms.Paint(frame, " |")
}

func hudLineStyle(line string) atoms.Style {
	line = strings.TrimSpace(atoms.StripANSI(line))
	switch {
	case strings.HasPrefix(line, "target!"), strings.HasPrefix(line, "sound!"):
		return atoms.Style{Fg: atoms.Coral, Bold: true}
	case strings.HasPrefix(line, "target"):
		return atoms.Style{Fg: atoms.Yellow, Bold: true}
	case strings.HasPrefix(line, "sound"):
		return atoms.Style{Fg: atoms.Yellow, Bold: true}
	case strings.HasPrefix(line, "[ answer"), strings.HasPrefix(line, "[ typed"), strings.HasPrefix(line, "[ keys"):
		return atoms.Style{Fg: atoms.White, Bg: atoms.BGDeepNavy, Bold: true}
	case strings.HasPrefix(line, "feedback"):
		return atoms.Style{Fg: atoms.Coral, Bold: true}
	case strings.HasPrefix(line, "tree topic"), strings.HasPrefix(line, "tree particle"):
		return atoms.Style{Fg: atoms.Yellow}
	case strings.HasPrefix(line, "tree verb"), strings.HasPrefix(line, "tree thanks"):
		return atoms.Style{Fg: atoms.Coral}
	case strings.HasPrefix(line, "tree time"), strings.HasPrefix(line, "tree subject"), strings.HasPrefix(line, "tree place"):
		return atoms.Style{Fg: atoms.Seafoam}
	case strings.HasPrefix(line, "tree adverb"), strings.HasPrefix(line, "tree polite"), strings.HasPrefix(line, "tree sentence"), strings.HasPrefix(line, "tree note"):
		return atoms.Style{Fg: atoms.Cyan}
	case strings.HasPrefix(line, "set"), strings.HasPrefix(line, "wave"):
		return atoms.Style{Fg: atoms.Cyan}
	case strings.HasPrefix(line, "STREAK"):
		return atoms.Style{Fg: atoms.Seafoam, Bold: true}
	case strings.HasPrefix(line, "hint"), strings.HasPrefix(line, "goal"):
		return atoms.Style{Fg: atoms.Seafoam}
	case strings.HasPrefix(line, "meaning"), strings.HasPrefix(line, "stats"), strings.HasPrefix(line, "boss"):
		return atoms.Style{Fg: atoms.White}
	default:
		return atoms.Style{Fg: atoms.White}
	}
}

func renderBox(label string, width int, prefix string) []string {
	available := width - atoms.DisplayWidth(prefix) - 4
	if available < 12 {
		available = 12
	}
	if available > 58 {
		available = 58
	}
	label = atoms.TruncateDisplay(label, available)
	inner := atoms.DisplayWidth(label)
	if inner < 12 {
		inner = 12
	}
	top := prefix + "+" + strings.Repeat("-", inner+2) + "+"
	mid := prefix + "| " + atoms.FitDisplay(label, inner) + " |"
	bottom := prefix + "+" + strings.Repeat("-", inner+2) + "+"
	return []string{
		branchLine(width, top),
		branchLine(width, mid),
		branchLine(width, bottom),
	}
}

func (m Model) treeOverviewLines(progress statestore.Progress, compact bool) []string {
	lines := []string{"DAG"}
	levels := []skilltree.LevelState{}
	if m.skillTree != nil {
		levels = m.skillTree.Levels(skilltree.StateProgress{Progress: progress})
	}
	for i, level := range levels {
		if compact && i == 5 {
			lines = append(lines, fmt.Sprintf("+%d more", len(levels)-i))
			break
		}
		mastered := 0
		training := 0
		for _, card := range level.Cards {
			switch card.Status {
			case skilltree.StatusMastered:
				mastered++
			case skilltree.StatusTraining:
				training++
			}
		}
		status := "open"
		if len(level.Requirements) > 0 {
			status = "locked"
			allReady := true
			for _, requirement := range level.Requirements {
				if !requirement.Mastered {
					allReady = false
					break
				}
			}
			if allReady {
				status = "open"
			}
		}
		pointer := " "
		if level.ID == m.levelID {
			pointer = ">"
		}
		title := firstNonBlank(m.stationNameForLevel(level.ID), level.Title, level.ID)
		lines = append(lines, fmt.Sprintf("%s %-6s %d/%d %s", pointer, status, mastered, len(level.Cards), title))
		if !compact && training > 0 {
			lines = append(lines, fmt.Sprintf("  charging %d", training))
		}
	}
	lines = append(lines, "", "NEXT")
	next := m.nextRouteLines(progress)
	if compact && len(next) > 2 {
		next = next[:2]
	}
	for _, line := range next {
		lines = append(lines, line)
	}
	return lines
}

func twoColumnLines(left []string, right []string, width int) []string {
	if width < 40 {
		width = 40
	}
	rightWidth := width / 3
	if rightWidth < 18 {
		rightWidth = 18
	}
	if rightWidth > 32 {
		rightWidth = 32
	}
	leftWidth := width - rightWidth - 3
	if leftWidth < 18 {
		leftWidth = 18
		rightWidth = width - leftWidth - 3
	}

	count := len(left)
	if len(right) > count {
		count = len(right)
	}
	lines := make([]string, 0, count)
	for i := 0; i < count; i++ {
		l := ""
		r := ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		lines = append(lines, atoms.FitDisplay(l, leftWidth)+" | "+atoms.FitDisplay(r, rightWidth))
	}
	return lines
}

func exerciseWidth(cardWidth int) int {
	width := cardWidth - 4
	if width > 72 {
		return 42
	}
	if width > 56 {
		return 34
	}
	return width
}

func (m Model) shooterField(enemies []game.Enemy, width int) []string {
	if width < 24 {
		width = 24
	}
	height := 7
	if m.compactViewport() {
		height = 4
	}
	lines := make([]string, height)
	for row := 0; row < height; row++ {
		lines[row] = atoms.StripANSI(atoms.DitherLine(width, m.drill.TickCount()+row*2))
	}

	for _, enemy := range enemies {
		row := clampInt(enemy.Row, 0, height-2)
		col := enemyLane(enemy, width)
		lines[row] = placeText(lines[row], enemy.Text, col, width)
	}

	ship := "        ^"
	shots := "        |"
	lines[height-2] = placeText(lines[height-2], shots, (width-atoms.DisplayWidth(shots))/2, width)
	lines[height-1] = placeText(lines[height-1], ship, (width-atoms.DisplayWidth(ship))/2, width)
	return lines
}

func enemyLane(enemy game.Enemy, width int) int {
	lanes := []int{width / 6, width / 3, width / 2, width * 2 / 3, width * 5 / 6}
	index := 0
	if len(lanes) > 0 {
		index = positiveMod(enemy.ID+enemy.SpawnedAt, len(lanes))
	}
	col := lanes[index] - atoms.DisplayWidth(enemy.Text)/2
	return clampInt(col, 0, width-atoms.DisplayWidth(enemy.Text))
}

func placeText(line string, text string, col int, width int) string {
	line = atoms.FitDisplay(line, width)
	text = atoms.TruncateDisplay(text, width)
	if col < 0 {
		col = 0
	}
	if col+atoms.DisplayWidth(text) > width {
		col = width - atoms.DisplayWidth(text)
	}
	if col < 0 {
		col = 0
	}

	left := atoms.TruncateDisplay(line, col)
	rightStart := col + atoms.DisplayWidth(text)
	right := dropDisplayPrefix(line, rightStart)
	return atoms.FitDisplay(left+text+right, width)
}

func dropDisplayPrefix(value string, width int) string {
	plain := atoms.StripANSI(value)
	used := 0
	for i, r := range plain {
		rw := atoms.DisplayWidth(string(r))
		if used+rw > width {
			return plain[i:]
		}
		used += rw
		if used == width {
			return plain[i+len(string(r)):]
		}
	}
	return ""
}

func positiveMod(value int, mod int) int {
	if mod <= 0 {
		return 0
	}
	value %= mod
	if value < 0 {
		value += mod
	}
	return value
}

func hullMeter(current, max int) string {
	current = clampInt(current, 0, max)
	return "[" + strings.Repeat("#", current) + strings.Repeat(".", max-current) + "]"
}

func (m Model) routeLine() string {
	options := m.levelOptions()
	if len(options) == 0 {
		return ""
	}
	unlocked := 0
	for _, option := range options {
		if !option.Locked {
			unlocked++
		}
	}
	current := indexLevelOption(options, m.levelID)
	return atoms.StripANSI(atoms.StationDots(len(options), current, unlocked))
}

func (m Model) waveProgressLine(progress statestore.Progress) string {
	level, ok := m.currentLevelState(progress)
	if !ok || len(level.Cards) == 0 {
		if total := m.drill.DeckSize(); total > 0 {
			return fmt.Sprintf("set     0/%d learned", total)
		}
		return "set     --"
	}

	mastered := 0
	for _, card := range level.Cards {
		if card.Status == skilltree.StatusMastered {
			mastered++
		}
	}
	return fmt.Sprintf("set     %d/%d learned", mastered, len(level.Cards))
}

func (m Model) levelSetLine(progress statestore.Progress, levelID string) string {
	level, ok := m.levelState(progress, levelID)
	if !ok || len(level.Cards) == 0 {
		return ""
	}
	mastered := 0
	for _, card := range level.Cards {
		if card.Status == skilltree.StatusMastered {
			mastered++
		}
	}
	return fmt.Sprintf("%s %d/%d", firstNonBlank(level.Title, level.ID), mastered, len(level.Cards))
}

func (m Model) masteryLine(cardID string, progress statestore.Progress, width int) string {
	card := progress.Cards[cardID]
	return masteryMeter(cleanStreak(card), statestore.MasteryCleanHitStreak, width)
}

func (m Model) cardByID(cardID string) (content.Card, bool) {
	if m.library == nil {
		return content.Card{}, false
	}
	for _, card := range m.library.Cards {
		if card.ID == cardID {
			return card, true
		}
	}
	return content.Card{}, false
}

func sentenceTreeLines(card content.Card) []string {
	if card.Type != content.CardTypePhrase {
		return nil
	}
	if lines := noteDrivenSentenceTreeLines(card.Notes); len(lines) > 0 {
		return lines
	}
	switch {
	case card.Text == "本日は誠にありがとうございました":
		return []string{
			"tree time     本日 / ほんじつ = today, formal",
			"tree topic    は = marks what the sentence is about",
			"tree adverb   誠に / まことに = truly, sincerely",
			"tree thanks   ありがとう = thanks",
			"tree polite   ございました = polite past finish",
		}
	case card.Text == "日が暮れる":
		return []string{
			"tree subject  日 / ひ = the sun",
			"tree particle が = subject marker",
			"tree verb     暮れる / くれる = sets, grows dark",
		}
	case strings.Contains(card.Text, "日暮里で毎日"):
		return []string{
			"tree place    日暮里で / にっぽりで = in Nippori",
			"tree time     毎日 / まいにち = every day",
			"tree subject  日が / ひが = the sun + subject marker",
			"tree verb     暮れる / くれる = sets",
		}
	case strings.Contains(card.Text, "春日町でも"):
		return []string{
			"tree place    春日町でも / かすがちょうでも = in Kasugacho too",
			"tree subject  日が / ひが = the sun + subject marker",
			"tree verb     暮れる / くれる = sets",
		}
	case card.Text == "日本で毎日勉強します":
		return []string{
			"tree place    日本で / にほんで = in Japan",
			"tree time     毎日 / まいにち = every day",
			"tree verb     勉強します / べんきょうします = study politely",
		}
	case card.Text == "本日は日本で勉強しました":
		return []string{
			"tree time     本日 / ほんじつ = today, formal",
			"tree topic    は = marks the topic",
			"tree place    日本で / にほんで = in Japan",
			"tree verb     勉強しました / べんきょうしました = studied politely",
		}
	case card.Text == "日暮里でも勉強しました":
		return []string{
			"tree place    日暮里でも / にっぽりでも = in Nippori too",
			"tree particle でも = also/even at",
			"tree verb     勉強しました / べんきょうしました = studied politely",
		}
	case card.Text == "今日は学校へ行きます":
		return []string{
			"tree time     今日 / きょう = today",
			"tree topic    は = marks the day as the frame",
			"tree place    学校へ / がっこうへ = toward school",
			"tree verb     行きます / いきます = go politely",
		}
	case card.Text == "明日は日本へ行きます":
		return []string{
			"tree time     明日 / あした = tomorrow",
			"tree topic    は = marks the day as the frame",
			"tree place    日本へ / にほんへ = toward Japan",
			"tree verb     行きます / いきます = go politely",
		}
	case card.Text == "昨日は家へ帰りました":
		return []string{
			"tree time     昨日 / きのう = yesterday",
			"tree topic    は = marks the day as the frame",
			"tree place    家へ / いえへ = toward home",
			"tree verb     帰りました / かえりました = returned politely",
		}
	case card.Text == "先生は本を読みます":
		return []string{
			"tree topic    先生は / せんせいは = teacher as topic",
			"tree object   本を / ほんを = book as object",
			"tree verb     読みます / よみます = read politely",
		}
	case card.Text == "学生は水を飲みます":
		return []string{
			"tree topic    学生は / がくせいは = student as topic",
			"tree object   水を / みずを = water as object",
			"tree verb     飲みます / のみます = drink politely",
		}
	case card.Text == "友達と映画を見ます":
		return []string{
			"tree partner  友達と / ともだちと = with a friend",
			"tree object   映画を / えいがを = movie as object",
			"tree verb     見ます / みます = watch politely",
		}
	default:
		lines := []string{"tree sentence " + card.Text}
		for _, note := range card.Notes {
			lines = append(lines, "tree note     "+note)
		}
		return lines
	}
}

func noteDrivenSentenceTreeLines(notes []string) []string {
	lines := make([]string, 0, len(notes))
	for _, note := range notes {
		note = strings.TrimSpace(note)
		if !strings.HasPrefix(note, "tree ") {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(note, "tree "))
		label := "note"
		if before, after, ok := strings.Cut(body, ":"); ok {
			label = strings.TrimSpace(before)
			body = strings.TrimSpace(after)
		}
		if body == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("tree %-8s %s", label, body))
	}
	return lines
}

func masteryMeter(current, max, width int) string {
	if max <= 0 {
		max = 1
	}
	current = clampInt(current, 0, max)
	if width < 18 {
		width = 18
	}
	label := "STREAK  "
	trail := fmt.Sprintf(" %d/%d", current, max)
	barWidth := width - atoms.DisplayWidth(label) - atoms.DisplayWidth(trail) - 2
	if barWidth < 3 {
		barWidth = 3
	}
	filled := current * barWidth / max
	line := label + "[" + strings.Repeat("#", filled) + strings.Repeat(".", barWidth-filled) + "]" + trail
	return atoms.TruncateDisplay(line, width)
}

func drillHitPoints(enemy game.Enemy, clean bool, combo int) int {
	sequence := kana.TypingSequence(enemy.Kana, enemy.RomajiHint)
	base := pointsCleanHit
	if !clean {
		base = pointsHintedHit
	}
	return base + len([]rune(sequence))*pointsPerRune + combo*pointsComboBonus
}

func drillHitReason(clean bool) string {
	if clean {
		return "clean hit"
	}
	return "hinted hit"
}

func signedPoints(delta int) string {
	if delta >= 0 {
		return fmt.Sprintf("+%d", delta)
	}
	return fmt.Sprintf("%d", delta)
}

func (m Model) currentLevelState(progress statestore.Progress) (skilltree.LevelState, bool) {
	return m.levelState(progress, m.levelID)
}

func (m Model) levelState(progress statestore.Progress, levelID string) (skilltree.LevelState, bool) {
	if m.skillTree == nil {
		return skilltree.LevelState{}, false
	}
	for _, level := range m.skillTree.Levels(skilltree.StateProgress{Progress: progress}) {
		if level.ID == levelID {
			return level, true
		}
	}
	return skilltree.LevelState{}, false
}

func (m Model) nextRouteLines(progress statestore.Progress) []string {
	options := m.levelOptions()
	for _, option := range options {
		if !option.Locked || (len(option.Missing) == 0 && option.MissingPoints == 0) {
			continue
		}
		lines := []string{fmt.Sprintf("NEXT %s", firstNonBlank(option.StationName, option.LevelTitle))}
		if option.MissingPoints > 0 {
			lines = append(lines, fmt.Sprintf("  unlock %d more points", option.MissingPoints))
		}
		for index, requirement := range option.Missing {
			if index == 2 {
				lines = append(lines, fmt.Sprintf("     +%d more locks", len(option.Missing)-index))
				break
			}
			card := progress.Cards[requirement.ID]
			lines = append(lines, fmt.Sprintf("  unlock %s/%s %d/%d", requirement.Text, requirement.Reading.Kana, cleanStreak(card), statestore.MasteryCleanHitStreak))
		}
		return lines
	}
	return []string{"NEXT route clear"}
}

func cleanStreak(card statestore.CardProgress) int {
	if card.Mastered {
		return statestore.MasteryCleanHitStreak
	}
	return clampInt(card.Streak, 0, statestore.MasteryCleanHitStreak)
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func (m Model) stationsCard(width int) string {
	options := m.levelOptions()
	progress := m.progress()
	body := []string{
		"enter open  j/down next  k/up prev  esc stream  q quit",
	}
	body = append(body, m.stationPromptLines(options)...)
	body = append(body, m.kanaMatrixLines(width-4, m.stationKanaRowBudget())...)
	body = append(body, m.kanjiGridLines(progress, options, width-4, m.stationGridRowBudget())...)
	if len(options) == 0 {
		body = append(body, "no stations")
	} else {
		cursor := boundedIndex(m.cursor, len(options))
		start, end := stationOptionWindow(len(options), cursor, m.stationOptionSpan())
		if start > 0 {
			body = append(body, fmt.Sprintf("... %d earlier documents", start))
		}
		for i := start; i < end; i++ {
			option := options[i]
			prefix := " "
			if i == cursor {
				prefix = ">"
			}
			status := "OPEN"
			if option.Locked {
				status = "LOCKED"
			} else if option.Complete {
				status = "DONE"
			}
			name := option.StationName
			if name == "" {
				name = option.LevelTitle
			}
			body = append(body, fmt.Sprintf("%s %02d %-6s %s", prefix, i+1, status, name))
			showDetails := m.height <= 0 || i == cursor
			if !showDetails {
				continue
			}
			body = append(body, fmt.Sprintf("    %s  cards:%d", option.LevelTitle, option.CardCount))
			if option.Locked {
				body = append(body, "    needs:")
				if option.MissingPoints > 0 {
					body = append(body, fmt.Sprintf("      %d more points", option.MissingPoints))
				}
				for _, requirement := range requirementLines(option.Missing) {
					body = append(body, "      "+requirement)
				}
			}
		}
		if end < len(options) {
			body = append(body, fmt.Sprintf("... %d later documents", len(options)-end))
		}
	}
	if m.last != "" {
		body = append(body, m.last)
	}
	if m.logErr != "" {
		body = append(body, "log: "+m.logErr)
	}

	return atoms.Card(atoms.CardSpec{
		Title:     "DOCUMENTS",
		Subtitle:  "Kotoba Beach",
		Body:      body,
		Footer:    "locked documents show tree requirements",
		Width:     width,
		Highlight: true,
	})
}

func (m Model) stationPromptLines(options []levelOption) []string {
	if len(options) == 0 {
		return nil
	}
	option := options[boundedIndex(m.cursor, len(options))]
	name := firstNonBlank(option.StationName, option.LevelTitle)
	if option.Locked {
		if option.MissingPoints > 0 {
			return []string{fmt.Sprintf("next locked  %s  %d more points", name, option.MissingPoints)}
		}
		return []string{"next locked  " + name}
	}
	if option.Complete {
		if next := m.nextOpenLevelID(); next != "" {
			return []string{"continue  " + levelTitle(m.library, next, next)}
		}
		return []string{"route clear"}
	}
	return []string{"continue  " + name}
}

func (m Model) stationKanaRowBudget() int {
	if m.height <= 0 {
		return 6
	}
	switch {
	case m.height <= 24:
		return 1
	case m.height <= 34:
		return 2
	case m.height <= 44:
		return 4
	default:
		return 6
	}
}

func (m Model) stationGridRowBudget() int {
	if m.height <= 0 {
		return 20
	}
	switch {
	case m.height <= 24:
		return 3
	case m.height <= 34:
		return 5
	case m.height <= 44:
		return 8
	case m.height <= 56:
		return 10
	default:
		return 20
	}
}

func (m Model) stationOptionSpan() int {
	if m.height <= 0 {
		return 0
	}
	switch {
	case m.height <= 24:
		return 3
	case m.height <= 34:
		return 5
	case m.height <= 44:
		return 8
	default:
		return 10
	}
}

func stationOptionWindow(length, cursor, span int) (int, int) {
	if span <= 0 || length <= span {
		return 0, length
	}
	start := cursor - 2
	if start < 0 {
		start = 0
	}
	if start+span > length {
		start = length - span
	}
	if start < 0 {
		start = 0
	}
	end := start + span
	if end > length {
		end = length
	}
	return start, end
}

func (m Model) kanaMatrixLines(width int, maxRows int) []string {
	rows := kana.BasicRows()
	if len(rows) == 0 || maxRows == 0 {
		return nil
	}
	lines := []string{
		"KANA MATRIX  row/vowel -> hiragana/katakana",
	}
	startRow, endRow := kanaMatrixWindow(m.levelID, len(rows), maxRows)
	if maxRows > 1 && maxRows < len(rows) {
		lines = append(lines, fmt.Sprintf("kana rows %d-%d of %d", startRow+1, endRow, len(rows)))
	}
	for _, row := range rows[startRow:endRow] {
		lines = append(lines, kanaMatrixRow(row))
	}
	return fitStyledLines(lines, width)
}

func kanaMatrixWindow(levelID string, length int, maxRows int) (int, int) {
	if maxRows <= 0 || length <= maxRows {
		return 0, length
	}
	start := 0
	if strings.Contains(levelID, "late") {
		start = 5
	}
	if start+maxRows > length {
		start = length - maxRows
	}
	if start < 0 {
		start = 0
	}
	return start, start + maxRows
}

func kanaMatrixRow(row kana.Row) string {
	parts := []string{fmt.Sprintf("%-5s", row.Label)}
	for _, cell := range row.Cells {
		parts = append(parts, fmt.Sprintf("%s %s/%s", cell.Romaji, cell.Hiragana, cell.Katakana))
	}
	return strings.Join(parts, "  ")
}

type beginnerKanjiStatus int

const (
	beginnerKanjiLocked beginnerKanjiStatus = iota
	beginnerKanjiAvailable
	beginnerKanjiCurrent
	beginnerKanjiMastered
)

func (m Model) kanjiGridLines(progress statestore.Progress, options []levelOption, width int, maxRows int) []string {
	rows := statestore.Beginner200KanjiRows()
	if len(rows) == 0 {
		return nil
	}
	optionIndex := indexLevelOptions(options)
	activeGroup := activeBeginnerGroup(m.levelID, options)
	mastered := countBeginnerKanjiMastered(progress)
	lines := []string{
		"KANJI GRID  " +
			atoms.Style{Fg: atoms.Seafoam, Bold: true}.Apply("mastered") + "  " +
			atoms.Style{Fg: atoms.Yellow, Bold: true}.Apply("current") + "  " +
			atoms.Style{Fg: atoms.White}.Apply("open") + "  " +
			atoms.Style{Fg: atoms.DeepNavy}.Apply("locked") + "  " +
			atoms.Style{Fg: atoms.Coral, Bold: true}.Apply("gate"),
		fmt.Sprintf("progress %03d/200 lit", mastered),
	}

	startRow, endRow := 0, len(rows)
	if maxRows > 0 && maxRows < len(rows) {
		activeRow := clampInt(activeGroup-1, 0, len(rows)-1)
		startRow = activeRow - maxRows/2
		if startRow < 0 {
			startRow = 0
		}
		endRow = startRow + maxRows
		if endRow > len(rows) {
			endRow = len(rows)
			startRow = endRow - maxRows
			if startRow < 0 {
				startRow = 0
			}
		}
		lines = append(lines, fmt.Sprintf("showing %03d-%03d of 200", startRow*10+1, endRow*10))
	}
	for rowIndex := startRow; rowIndex < endRow; rowIndex++ {
		lines = append(lines, m.kanjiGridRow(rows[rowIndex], rowIndex, progress, activeGroup, optionIndex))
	}
	return fitStyledLines(lines, width)
}

func (m Model) kanjiGridRow(row string, rowIndex int, progress statestore.Progress, activeGroup int, optionIndex map[string]levelOption) string {
	start := rowIndex*10 + 1
	end := start + 9
	labelStyle := atoms.Style{Fg: atoms.Cyan}
	if beginnerSentenceGateActive(rowIndex+1, optionIndex) {
		labelStyle = atoms.Style{Fg: atoms.Coral, Bold: true}
	}
	var b strings.Builder
	b.WriteString(labelStyle.Apply(fmt.Sprintf("%03d-%03d", start, end)))
	b.WriteString("  ")
	for i, r := range row {
		if i > 0 {
			b.WriteRune(' ')
		}
		index := start + i
		status := beginnerKanjiCellStatus(index, progress, activeGroup, optionIndex)
		b.WriteString(beginnerKanjiStyle(status).Apply(string(r)))
	}
	return b.String()
}

func beginnerKanjiCellStatus(index int, progress statestore.Progress, activeGroup int, optionIndex map[string]levelOption) beginnerKanjiStatus {
	if progress.Cards[fmt.Sprintf("lesson-b200-%03d-kanji", index)].Mastered {
		return beginnerKanjiMastered
	}
	group := (index-1)/10 + 1
	option, ok := optionIndex[fmt.Sprintf("lesson-b200-g%02d-core", group)]
	if group == activeGroup && ok && !option.Locked {
		return beginnerKanjiCurrent
	}
	if ok && !option.Locked {
		return beginnerKanjiAvailable
	}
	return beginnerKanjiLocked
}

func beginnerKanjiStyle(status beginnerKanjiStatus) atoms.Style {
	switch status {
	case beginnerKanjiMastered:
		return atoms.Style{Fg: atoms.Seafoam, Bold: true}
	case beginnerKanjiCurrent:
		return atoms.Style{Fg: atoms.Yellow, Bold: true}
	case beginnerKanjiAvailable:
		return atoms.Style{Fg: atoms.White}
	default:
		return atoms.Style{Fg: atoms.DeepNavy}
	}
}

func beginnerSentenceGateActive(group int, optionIndex map[string]levelOption) bool {
	core, coreOK := optionIndex[fmt.Sprintf("lesson-b200-g%02d-core", group)]
	gate, gateOK := optionIndex[fmt.Sprintf("lesson-b200-g%02d-sentences", group)]
	return coreOK && gateOK && core.Complete && !gate.Complete
}

func activeBeginnerGroup(levelID string, options []levelOption) int {
	if group := beginnerGroupFromLevelID(levelID); group > 0 && beginnerGroupInProgress(group, options) {
		return group
	}
	for group := 1; group <= 20; group++ {
		coreID := fmt.Sprintf("lesson-b200-g%02d-core", group)
		gateID := fmt.Sprintf("lesson-b200-g%02d-sentences", group)
		for _, option := range options {
			if (option.LevelID == coreID || option.LevelID == gateID) && !option.Locked && !option.Complete {
				return group
			}
		}
	}
	if group := beginnerGroupFromLevelID(levelID); group > 0 {
		return group
	}
	return 1
}

func beginnerGroupInProgress(group int, options []levelOption) bool {
	coreID := fmt.Sprintf("lesson-b200-g%02d-core", group)
	gateID := fmt.Sprintf("lesson-b200-g%02d-sentences", group)
	for _, option := range options {
		if option.LevelID != coreID && option.LevelID != gateID {
			continue
		}
		if !option.Locked && !option.Complete {
			return true
		}
	}
	return false
}

func beginnerGroupFromLevelID(levelID string) int {
	var group int
	if _, err := fmt.Sscanf(levelID, "lesson-b200-g%d-", &group); err == nil && group > 0 {
		return group
	}
	return 0
}

func countBeginnerKanjiMastered(progress statestore.Progress) int {
	mastered := 0
	for index := 1; index <= 200; index++ {
		if progress.Cards[fmt.Sprintf("lesson-b200-%03d-kanji", index)].Mastered {
			mastered++
		}
	}
	return mastered
}

func fitStyledLines(lines []string, width int) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, atoms.FitStyledDisplay(line, width))
	}
	return out
}

func (m Model) levelOption(levelID string) (levelOption, bool) {
	for _, option := range m.levelOptions() {
		if option.LevelID == levelID {
			return option, true
		}
	}
	return levelOption{}, false
}

func indexLevelOptions(options []levelOption) map[string]levelOption {
	out := make(map[string]levelOption, len(options))
	for _, option := range options {
		out[option.LevelID] = option
	}
	return out
}

func (m Model) levelOptions() []levelOption {
	if m.library == nil {
		return nil
	}
	progress := m.progress()
	cardIndex := indexContentCards(m.library.Cards)
	options := make([]levelOption, 0, len(m.library.Levels))
	for _, level := range m.library.Levels {
		option := levelOption{
			LevelID:        level.ID,
			LevelTitle:     firstNonBlank(level.Title, level.ID),
			StationName:    m.stationNameForLevel(level.ID),
			Description:    level.Description,
			CardCount:      len(level.CardIDs),
			RequiredPoints: level.RequiredPoints,
			Complete:       levelCompleteFromIndex(level, cardIndex, progress),
		}
		explicitlyUnlocked := progress.UnlockedLevels[level.ID]
		if level.RequiredPoints > progress.Points {
			option.MissingPoints = level.RequiredPoints - progress.Points
		}
		for _, cardID := range level.RequiredCardIDs {
			card, ok := cardIndex[cardID]
			if !ok {
				continue
			}
			option.Required = append(option.Required, card)
			if !explicitlyUnlocked && !progress.Cards[cardID].Mastered {
				option.Missing = append(option.Missing, card)
			}
		}
		option.Locked = !levelAvailable(level, progress)
		options = append(options, option)
	}
	return options
}

func (m Model) availableLevelIDs(progress statestore.Progress) map[string]bool {
	available := map[string]bool{}
	if m.library == nil {
		return available
	}
	for _, level := range m.library.Levels {
		if levelAvailable(level, progress) {
			available[level.ID] = true
		}
	}
	return available
}

func levelAvailable(level content.Level, progress statestore.Progress) bool {
	if progress.UnlockedLevels[level.ID] {
		return true
	}
	if progress.Points < level.RequiredPoints {
		return false
	}
	grandfathered := hasNonKanaFoundationProgress(progress)
	for _, cardID := range level.RequiredCardIDs {
		if grandfathered && isKanaFoundationCardID(cardID) {
			continue
		}
		if !progress.Cards[cardID].Mastered {
			return false
		}
	}
	return true
}

func (m Model) stationNameForLevel(levelID string) string {
	if m.stations == nil {
		return ""
	}
	for _, st := range m.stations.Stations {
		if st.LevelID == levelID {
			return st.Name
		}
	}
	return ""
}

func (m Model) documentTitleForLevel(levelID string) string {
	if m.library == nil {
		return firstNonBlank(m.level, levelID)
	}
	documentID := ""
	for _, level := range m.library.Levels {
		if level.ID == levelID {
			documentID = level.DocumentID
			if documentID == "" {
				return firstNonBlank(level.Title, m.level, levelID)
			}
			break
		}
	}
	for _, document := range m.library.Documents {
		if document.ID == documentID && strings.TrimSpace(document.Title) != "" {
			return document.Title
		}
	}
	return firstNonBlank(m.level, levelID)
}

func (m Model) levelDescription(levelID string) string {
	if m.library == nil {
		return ""
	}
	for _, level := range m.library.Levels {
		if level.ID == levelID {
			return strings.TrimSpace(level.Description)
		}
	}
	return ""
}

func (m Model) campaignTitleForLevel(levelID string) string {
	if m.library == nil {
		return ""
	}
	for _, campaign := range m.library.Campaigns {
		for _, id := range campaign.LevelIDs {
			if id == levelID {
				return strings.TrimSpace(campaign.Title)
			}
		}
		if campaign.StartLevelID == levelID {
			return strings.TrimSpace(campaign.Title)
		}
	}
	return ""
}

func (m Model) progress() statestore.Progress {
	if m.eventLog == nil || m.eventLog.Path() == "" {
		return statestore.NewProgress()
	}
	progress, err := m.eventLog.Replay()
	if err != nil {
		return statestore.NewProgress()
	}
	return progress
}

func levelComplete(library *content.Library, levelID string, progress statestore.Progress) bool {
	cards := levelCards(library, levelID)
	if len(cards) == 0 {
		return false
	}
	for _, card := range cards {
		if !progress.Cards[card.ID].Mastered {
			return false
		}
	}
	return true
}

func levelCompleteFromIndex(level content.Level, cardIndex map[string]content.Card, progress statestore.Progress) bool {
	found := false
	for _, cardID := range level.CardIDs {
		card, ok := cardIndex[cardID]
		if !ok {
			continue
		}
		found = true
		if !progress.Cards[card.ID].Mastered {
			return false
		}
	}
	return found
}

func (m Model) nextOpenLevelID() string {
	progress := m.progress()
	options := m.levelOptions()
	if len(options) == 0 {
		return ""
	}
	current := indexLevelOption(options, m.levelID)
	for offset := 1; offset <= len(options); offset++ {
		option := options[(current+offset)%len(options)]
		if option.Locked || levelComplete(m.library, option.LevelID, progress) {
			continue
		}
		return option.LevelID
	}
	return ""
}

func (m Model) bossCard(width int) string {
	progress := m.progress()
	return strings.Join(m.branchRunLines(progress, width, true), "\n")
}

func (m Model) bossExerciseLines(width int) []string {
	b := m.boss.Boss()
	phase := m.boss.Phase()
	body := []string{
		fmt.Sprintf("BOSS %s", b.Title),
		fmt.Sprintf("%s  %s", phase.Title, phase.Glyph),
		atoms.HPBar(m.boss.HP(), b.HP, exerciseWidth(width)),
		centerLine(phase.Glyph, exerciseWidth(width)),
		"esc wave  ? hint  q quit",
	}

	previewTarget := ""
	if target, ok := m.boss.Target(); ok {
		previewTarget = target.Kana
		body = append(body,
			fmt.Sprintf("weak point  %s", target.Text),
			fmt.Sprintf("meaning     %s", target.Meaning),
			atoms.StripANSI(atoms.InputBar("type", m.input, exerciseWidth(width), true)),
		)
	} else {
		body = append(body, "boss cleared", atoms.StripANSI(atoms.InputBar("type", m.input, exerciseWidth(width), false)))
	}
	if preview := kana.PreviewForTarget(previewTarget, m.input); preview != "" {
		body = append(body, fmt.Sprintf("sound  %s", preview))
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
	return body
}

func (m Model) cardWidth() int {
	if m.width == 0 {
		return 80
	}
	if m.width < 40 {
		return 40
	}
	if m.width > 100 {
		return 100
	}
	return m.width
}

func (m Model) compactViewport() bool {
	return m.height == 0 || m.height <= 24
}

func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
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
			{ID: "sealed", Title: "SEALED SUN", Glyph: "日", StartsAtHP: hp},
			{ID: "cracked", Title: "CRACKED EVENING", Glyph: "暮", StartsAtHP: mid},
			{ID: "cleared", Title: "CLEAR WATER", Glyph: "日暮", StartsAtHP: 0},
		},
		Chunks: chunks,
	}
}

func newLevelBoss(library *content.Library, levelID, title string) boss.Boss {
	cards := levelCards(library, levelID)
	chunks := boss.ChunksFromCards(cards)
	if len(chunks) == 0 {
		chunks = fallbackBossChunks(cards)
	}
	hp := len(chunks)
	if hp < 3 {
		hp = 3
	}
	glyph := bossGlyph(cards)
	if glyph == "" {
		glyph = "門"
	}
	bossTitle := strings.TrimSpace(title)
	if bossTitle == "" {
		bossTitle = "Kotoba Gate"
	}
	mid := hp / 2
	if mid < 1 {
		mid = 1
	}
	return boss.Boss{
		ID:    "gate-" + levelID,
		Title: bossTitle,
		Glyph: glyph,
		HP:    hp,
		Phases: []boss.Phase{
			{ID: "shield", Title: "BOSS SHIELD", Glyph: glyph, StartsAtHP: hp},
			{ID: "split", Title: "CRACKED SHIELD", Glyph: glyph, StartsAtHP: mid},
			{ID: "open", Title: "OPEN WATER", Glyph: glyph, StartsAtHP: 0},
		},
		Chunks: chunks,
	}
}

func bossGlyph(cards []content.Card) string {
	for _, card := range cards {
		if strings.TrimSpace(card.Kanji) != "" {
			return strings.TrimSpace(card.Kanji)
		}
		if strings.TrimSpace(card.Text) != "" {
			return strings.TrimSpace(card.Text)
		}
	}
	return ""
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

func loadStationCatalog() (*station.Catalog, station.ValidationReport, error) {
	candidates := contentFileCandidates(filepath.Join("stations", "catalog.json"))
	var lastErr error
	for _, candidate := range candidates {
		catalog, report, err := station.LoadFile(candidate)
		if err == nil {
			return catalog, report, nil
		}
		lastErr = err
	}
	return nil, station.ValidationReport{}, lastErr
}

func contentFileCandidates(name string) []string {
	candidates := []string{filepath.Join("content", name)}
	wd, err := os.Getwd()
	if err != nil {
		return candidates
	}

	for dir := wd; ; dir = filepath.Dir(dir) {
		candidates = append(candidates, filepath.Join(dir, "content", name))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return candidates
}

func newLevelDrill(library *content.Library, levelID string, progress statestore.Progress) game.Drill {
	return game.NewDrillFromCards(unlearnedLevelCards(library, levelID, progress), game.Config{SpawnEvery: 6, MaxEnemies: 3}).Start()
}

func unlearnedLevelCards(library *content.Library, levelID string, progress statestore.Progress) []content.Card {
	cards := levelCards(library, levelID)
	if len(cards) == 0 {
		return cards
	}

	unlearned := make([]content.Card, 0, len(cards))
	for _, card := range cards {
		if !progress.Cards[card.ID].Mastered {
			unlearned = append(unlearned, card)
		}
	}
	if len(unlearned) > 0 {
		return unlearned
	}
	return nil
}

func levelCards(library *content.Library, levelID string) []content.Card {
	if library == nil {
		return nil
	}
	cardIndex := make(map[string]content.Card, len(library.Cards))
	for _, card := range library.Cards {
		cardIndex[card.ID] = card
	}
	for _, level := range library.Levels {
		if level.ID != levelID {
			continue
		}
		cards := make([]content.Card, 0, len(level.CardIDs))
		for _, cardID := range level.CardIDs {
			if card, ok := cardIndex[cardID]; ok {
				cards = append(cards, card)
			}
		}
		return cards
	}
	return library.Cards
}

func levelTitle(library *content.Library, levelID, fallback string) string {
	if library == nil {
		return fallback
	}
	for _, level := range library.Levels {
		if level.ID == levelID && strings.TrimSpace(level.Title) != "" {
			return level.Title
		}
	}
	return fallback
}

func firstPlayableLevelID(library *content.Library, fallback string) string {
	if library == nil {
		return fallback
	}
	for _, campaign := range library.Campaigns {
		if strings.TrimSpace(campaign.StartLevelID) != "" {
			return campaign.StartLevelID
		}
	}
	if len(library.Levels) > 0 && strings.TrimSpace(library.Levels[0].ID) != "" {
		return library.Levels[0].ID
	}
	return fallback
}

func resumeLevelID(library *content.Library, fallback string, progress statestore.Progress) string {
	start := firstPlayableLevelID(library, fallback)
	if library == nil || len(library.Levels) == 0 {
		return start
	}

	skipKanaFoundation := hasNonKanaFoundationProgress(progress)
	lastAvailable := ""
	for _, level := range library.Levels {
		if skipKanaFoundation && isKanaFoundationLevelID(level.ID) {
			continue
		}
		if !levelAvailable(level, progress) {
			continue
		}
		lastAvailable = level.ID
		if !levelComplete(library, level.ID, progress) {
			return level.ID
		}
	}
	if lastAvailable != "" {
		return lastAvailable
	}
	return start
}

func hasNonKanaFoundationProgress(progress statestore.Progress) bool {
	for cardID, card := range progress.Cards {
		if isKanaFoundationCardID(cardID) {
			continue
		}
		if card.Mastered || card.Streak > 0 || card.HintsUsed > 0 || len(card.RevealedHints) > 0 {
			return true
		}
	}
	return false
}

func isKanaFoundationLevelID(levelID string) bool {
	return strings.HasPrefix(levelID, "lesson-kana-")
}

func isKanaFoundationCardID(cardID string) bool {
	return strings.HasPrefix(cardID, "lesson-kana-")
}

func indexContentCards(cards []content.Card) map[string]content.Card {
	index := make(map[string]content.Card, len(cards))
	for _, card := range cards {
		index[card.ID] = card
	}
	return index
}

func indexLevelOption(options []levelOption, levelID string) int {
	for i, option := range options {
		if option.LevelID == levelID {
			return i
		}
	}
	return 0
}

func boundedIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

func requirementLines(cards []content.Card) []string {
	if len(cards) == 0 {
		return []string{"none"}
	}
	limit := 6
	lines := make([]string, 0, limit+1)
	for i, card := range cards {
		if i == limit {
			lines = append(lines, fmt.Sprintf("+%d more locks", len(cards)-limit))
			break
		}
		lines = append(lines, fmt.Sprintf("%s/%s", card.Text, card.Reading.Kana))
	}
	return lines
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
