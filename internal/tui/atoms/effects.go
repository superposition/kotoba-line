package atoms

import (
	"fmt"
	"strings"
)

// Flash renders a deterministic arcade flash panel for hits, clears, and damage.
func Flash(label string, width, height, tick int) string {
	width = clamp(width, 8)
	height = clamp(height, 1)

	fill := '#'
	color := Coral
	if tick%2 != 0 {
		fill = '*'
		color = Yellow
	}

	lines := make([]string, 0, height)
	mid := height / 2
	for row := 0; row < height; row++ {
		line := strings.Repeat(string(fill), width)
		if row == mid && label != "" {
			line = centerText(label, width)
		}
		lines = append(lines, Paint(color, line))
	}
	return strings.Join(lines, "\n")
}

// StoryboardStep is one labeled beat in an ASCII cutscene storyboard.
type StoryboardStep struct {
	Beat  string
	Lines []string
}

// Storyboard renders ordered cutscene beats as compact fixed-width cards.
func Storyboard(steps []StoryboardStep, width int) string {
	width = clamp(width, 20)
	rendered := make([]string, 0, len(steps))
	for i, step := range steps {
		title := fmt.Sprintf("%02d %s", i+1, step.Beat)
		rendered = append(rendered, Card(CardSpec{
			Title: title,
			Body:  step.Lines,
			Width: width,
		}))
	}
	return strings.Join(rendered, "\n")
}

func centerText(text string, width int) string {
	text = TruncateDisplay(text, width)
	left := (width - DisplayWidth(text)) / 2
	right := width - left - DisplayWidth(text)
	return strings.Repeat(" ", left) + text + strings.Repeat(" ", right)
}
