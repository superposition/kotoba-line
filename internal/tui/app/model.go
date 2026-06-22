package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Options struct {
	Username string
}

type Model struct {
	username string
	width    int
	height   int
}

func New(opts Options) Model {
	username := strings.TrimSpace(opts.Username)
	if username == "" {
		username = "player"
	}
	return Model{username: username}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	lines := []string{
		"Kotoba Line",
		"",
		fmt.Sprintf("Player: %s", m.username),
		"",
		"Station 01: Tide Gate",
		"Cards online: SSH skeleton",
		"Next route: content model and kana drills",
		"",
		"Press q to disconnect.",
	}

	if m.width > 0 && m.height > 0 {
		lines = append(lines, "", fmt.Sprintf("Terminal: %dx%d", m.width, m.height))
	}

	return strings.Join(lines, "\n")
}
