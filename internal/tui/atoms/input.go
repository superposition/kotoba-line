package atoms

import "strings"

// InputBar renders a fixed-width kana entry field. The prompt and value may
// contain Japanese text; visible width remains stable.
func InputBar(prompt, value string, width int, focused bool) string {
	width = clamp(width, 16)
	inner := width - 4
	cursor := " "
	frame := OceanBlue
	textStyle := Style{Fg: White}
	if focused {
		cursor = "_"
		frame = Yellow
		textStyle = Style{Fg: White, Bold: true}
	}

	content := prompt
	if content != "" {
		content += " "
	}
	content += value + cursor

	return Paint(frame, "[ ") +
		textStyle.Apply(FitDisplay(content, inner)) +
		Paint(frame, " ]")
}

// StationDots renders route progress using ASCII dots that remain legible over SSH.
func StationDots(total, current, unlocked int) string {
	total = clamp(total, 1)
	current = bounded(current, 0, total-1)
	unlocked = bounded(unlocked, 0, total)

	parts := make([]string, 0, total)
	for i := 0; i < total; i++ {
		switch {
		case i == current:
			parts = append(parts, Paint(Yellow, "O"))
		case i < unlocked:
			parts = append(parts, Paint(Seafoam, "o"))
		default:
			parts = append(parts, Paint(DeepNavy, "."))
		}
	}

	return Paint(Cyan, "STATION ") + strings.Join(parts, Paint(OceanBlue, "-"))
}
