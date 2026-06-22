package atoms

import "strings"

// Color is an ANSI SGR sequence for terminal-native styling.
type Color string

const (
	Reset Color = "\x1b[0m"

	OceanBlue Color = "\x1b[38;5;33m"
	Seafoam   Color = "\x1b[38;5;121m"
	Cyan      Color = "\x1b[38;5;51m"
	Yellow    Color = "\x1b[38;5;226m"
	Coral     Color = "\x1b[38;5;203m"
	White     Color = "\x1b[38;5;15m"
	DeepNavy  Color = "\x1b[38;5;17m"

	BGDeepNavy Color = "\x1b[48;5;17m"
)

// Style describes a tiny ANSI style without pulling in a TUI dependency.
type Style struct {
	Fg   Color
	Bg   Color
	Bold bool
}

// Apply wraps text in ANSI style sequences. Empty styles leave text untouched.
func (s Style) Apply(text string) string {
	if text == "" || (s.Fg == "" && s.Bg == "" && !s.Bold) {
		return text
	}

	var b strings.Builder
	if s.Bold {
		b.WriteString("\x1b[1m")
	}
	if s.Fg != "" {
		b.WriteString(string(s.Fg))
	}
	if s.Bg != "" {
		b.WriteString(string(s.Bg))
	}
	b.WriteString(text)
	b.WriteString(string(Reset))
	return b.String()
}

// Paint applies a single foreground or background color to text.
func Paint(color Color, text string) string {
	return Style{Fg: color}.Apply(text)
}
