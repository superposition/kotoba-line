package atoms

import (
	"fmt"
	"strings"
)

// HPBar renders a fixed-width health bar.
func HPBar(current, max, width int) string {
	width = clamp(width, 14)
	if max <= 0 {
		max = 1
	}
	current = bounded(current, 0, max)

	value := fmt.Sprintf(" %02d/%02d", current, max)
	barWidth := width - DisplayWidth("HP []") - DisplayWidth(value)
	if barWidth < 1 {
		barWidth = 1
	}

	filled := current * barWidth / max
	empty := barWidth - filled
	line := Paint(Yellow, "HP") +
		" [" +
		Paint(Coral, strings.Repeat("#", filled)) +
		Paint(DeepNavy, strings.Repeat("-", empty)) +
		"]" +
		value

	return line + strings.Repeat(" ", width-DisplayWidth(line))
}

// ComboMeter renders a fixed-width combo charge meter.
func ComboMeter(combo, max, width int) string {
	width = clamp(width, 18)
	if max <= 0 {
		max = 1
	}
	combo = bounded(combo, 0, max)

	label := fmt.Sprintf("COMBO x%02d ", combo)
	barWidth := width - DisplayWidth(label) - DisplayWidth("[]")
	if barWidth < 1 {
		barWidth = 1
	}

	filled := combo * barWidth / max
	empty := barWidth - filled
	line := Paint(Cyan, label) +
		"[" +
		Paint(Yellow, strings.Repeat("*", filled)) +
		Paint(DeepNavy, strings.Repeat(".", empty)) +
		"]"

	return line + strings.Repeat(" ", width-DisplayWidth(line))
}

func bounded(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
