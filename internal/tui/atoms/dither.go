package atoms

import "strings"

var ditherPattern = []rune{'~', '.', ':', '.', '~', '='}

// DitherLine renders one deterministic ocean-wave fill row.
func DitherLine(width, phase int) string {
	width = clamp(width, 1)
	var b strings.Builder
	for i := 0; i < width; i++ {
		idx := positiveMod(i+phase, len(ditherPattern))
		b.WriteRune(ditherPattern[idx])
	}
	return Paint(Seafoam, b.String())
}

// DitherFill renders a rectangular deterministic fill with visible line widths.
func DitherFill(width, height, phase int) string {
	width = clamp(width, 1)
	height = clamp(height, 1)

	lines := make([]string, 0, height)
	for row := 0; row < height; row++ {
		lines = append(lines, DitherLine(width, phase+row*2))
	}
	return strings.Join(lines, "\n")
}

func positiveMod(n, mod int) int {
	n %= mod
	if n < 0 {
		n += mod
	}
	return n
}
