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
	"github.com/superposition/kotoba-line/internal/station"
	coretransition "github.com/superposition/kotoba-line/internal/transition"
	"github.com/superposition/kotoba-line/internal/tui/atoms"
	tuitransition "github.com/superposition/kotoba-line/internal/tui/transition"
)

const (
	starterLevelID      = "journal-2026-06-22-key-readings"
	constitutionLevelID = "constitution-preamble-1"
)

var playableContentFiles = []string{
	"seed-2026-06-22.json",
	"constitution-preamble-article1-playable.json",
}

type screenMode string

const (
	modeDrill    screenMode = "drill"
	modeBoss     screenMode = "boss"
	modeStations screenMode = "stations"
)

type Options struct {
	Username        string
	Library         *content.Library
	StationCatalog  *station.Catalog
	EventLogPath    string
	DisableEventLog bool
}

type levelOption struct {
	LevelID     string
	LevelTitle  string
	StationName string
	Description string
	CardCount   int
	Required    []content.Card
	Missing     []content.Card
	Locked      bool
}

type Model struct {
	username string
	width    int
	height   int
	library  *content.Library
	stations *station.Catalog
	mode     screenMode
	drill    game.Drill
	boss     boss.Fight
	levelID  string
	level    string
	cursor   int
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
	stations := opts.StationCatalog
	loadErr := ""
	if library == nil {
		loaded, report, err := loadPlayableContent()
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

	model := Model{
		username: username,
		library:  library,
		stations: stations,
		mode:     modeDrill,
		drill:    newLevelDrill(library, starterLevelID),
		boss:     boss.NewFight(newDocumentBoss(library)),
		levelID:  starterLevelID,
		level:    levelTitle(library, starterLevelID, "Tide Gate"),
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
		if m.mode == modeDrill {
			m.drill = m.drill.Tick()
		}
		return m, drillTick()
	case tea.KeyMsg:
		if m.mode == modeStations && msg.Type == tea.KeyRunes && msg.String() != "q" {
			m = m.handleStationRunes(msg.Runes)
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "s":
			m = m.openStations()
		case "b":
			m = m.enterBoss()
		case "c":
			m = m.switchLevel(constitutionLevelID)
		case "j":
			if m.mode == modeStations {
				m = m.moveStationCursor(1)
			} else {
				m = m.switchLevel(starterLevelID)
			}
		case "k":
			if m.mode == modeStations {
				m = m.moveStationCursor(-1)
			} else if msg.Type == tea.KeyRunes {
				m.input += string(msg.Runes)
				m.last = ""
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
			m.mode = modeDrill
			m.input = ""
			m.hint = ""
			m.bossHint = ""
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
	width := m.cardWidth()
	lines := []string{
		"Kotoba Line",
		"",
		fmt.Sprintf("Player: %s", m.username),
		"",
		fmt.Sprintf("Station: %s", m.level),
	}

	if m.loadErr != "" {
		lines = append(lines, "", m.loadErr)
	} else if m.mode == modeStations {
		lines = append(lines, "", m.stationsCard(width))
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
			m.last = fmt.Sprintf("MISS %s  target %s wants %s", result.Input, result.Enemy.Text, result.Enemy.Kana)
		} else {
			m.last = fmt.Sprintf("MISS %s", result.Input)
		}
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
	m.hint = fmt.Sprintf("hint: %s = %s (%s)", hint.Enemy.Text, hint.Enemy.Kana, hint.Romaji)
	return m
}

func (m Model) openStations() Model {
	m.mode = modeStations
	m.input = ""
	m.hint = ""
	m.bossHint = ""
	options := m.levelOptions()
	m.cursor = indexLevelOption(options, m.levelID)
	m.last = "STATION SELECT"
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
	drill := newLevelDrill(m.library, levelID)
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
	m.scene = renderScene(coretransition.SceneStationArrival, m.level, m.cardWidth())
	m.last = fmt.Sprintf("LEVEL %s", m.level)
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
		"s stations  c constitution  j starter  b boss  ?/？ hint  q quit",
	}

	enemies := m.drill.Enemies()
	if len(enemies) == 0 {
		body = append(body, "no targets")
	} else {
		target := enemies[0]
		body = append(body,
			fmt.Sprintf("target  %s", target.Text),
			fmt.Sprintf("note    %s", target.Meaning),
		)
		if len(enemies) > 1 {
			body = append(body, "queue")
			for _, enemy := range enemies[1:] {
				body = append(body, fmt.Sprintf("  %s  %s", enemy.Text, enemy.Meaning))
			}
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

func (m Model) stationsCard(width int) string {
	options := m.levelOptions()
	body := []string{
		"enter travel  j/down next  k/up prev  esc drill  q quit",
	}
	if len(options) == 0 {
		body = append(body, "no stations")
	} else {
		cursor := boundedIndex(m.cursor, len(options))
		for i, option := range options {
			prefix := " "
			if i == cursor {
				prefix = ">"
			}
			status := "OPEN"
			if option.Locked {
				status = "LOCKED"
			}
			name := option.StationName
			if name == "" {
				name = option.LevelTitle
			}
			body = append(body, fmt.Sprintf("%s %02d %-6s %s", prefix, i+1, status, name))
			body = append(body, fmt.Sprintf("    %s  cards:%d", option.LevelTitle, option.CardCount))
			if option.Locked {
				body = append(body, "    needs:")
				for _, requirement := range requirementLines(option.Missing) {
					body = append(body, "      "+requirement)
				}
			}
		}
	}
	if m.last != "" {
		body = append(body, m.last)
	}
	if m.logErr != "" {
		body = append(body, "log: "+m.logErr)
	}

	return atoms.Card(atoms.CardSpec{
		Title:     "STATIONS",
		Subtitle:  "Kotoba Line",
		Body:      body,
		Footer:    "locked stations show required cards",
		Width:     width,
		Highlight: true,
	})
}

func (m Model) levelOption(levelID string) (levelOption, bool) {
	for _, option := range m.levelOptions() {
		if option.LevelID == levelID {
			return option, true
		}
	}
	return levelOption{}, false
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
			LevelID:     level.ID,
			LevelTitle:  firstNonBlank(level.Title, level.ID),
			StationName: m.stationNameForLevel(level.ID),
			Description: level.Description,
			CardCount:   len(level.CardIDs),
		}
		for _, cardID := range level.RequiredCardIDs {
			card, ok := cardIndex[cardID]
			if !ok {
				continue
			}
			option.Required = append(option.Required, card)
			if !progress.Cards[cardID].Mastered {
				option.Missing = append(option.Missing, card)
			}
		}
		option.Locked = len(option.Missing) > 0
		options = append(options, option)
	}
	return options
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

func (m Model) progress() statestore.Progress {
	if m.eventLog.Path() == "" {
		return statestore.NewProgress()
	}
	progress, err := m.eventLog.Replay()
	if err != nil {
		return statestore.NewProgress()
	}
	return progress
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

func loadPlayableContent() (*content.Library, content.ValidationReport, error) {
	var merged content.Library
	for _, name := range playableContentFiles {
		library, report, err := loadPlayableContentFile(name)
		if err != nil {
			return nil, content.ValidationReport{}, err
		}
		if report.HasErrors() {
			return nil, report, nil
		}
		appendLibrary(&merged, library)
	}
	report := content.ValidateLibrary(&merged)
	return &merged, report, nil
}

func loadPlayableContentFile(name string) (*content.Library, content.ValidationReport, error) {
	candidates := contentFileCandidates(name)
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

func appendLibrary(dst *content.Library, src *content.Library) {
	if src == nil {
		return
	}
	dst.Cards = append(dst.Cards, src.Cards...)
	dst.Documents = append(dst.Documents, src.Documents...)
	dst.Levels = append(dst.Levels, src.Levels...)
	dst.Campaigns = append(dst.Campaigns, src.Campaigns...)
}

func newLevelDrill(library *content.Library, levelID string) game.Drill {
	return game.NewDrillFromCards(levelCards(library, levelID), game.Config{SpawnEvery: 6, MaxEnemies: 3}).Start()
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
	lines := make([]string, 0, len(cards))
	for _, card := range cards {
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
