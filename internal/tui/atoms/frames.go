package atoms

import "strings"

// CardSpec defines a framed bright-ocean card. Width is terminal cell width.
type CardSpec struct {
	Title     string
	Subtitle  string
	Body      []string
	Footer    string
	Width     int
	Highlight bool
}

// Card renders an ASCII frame whose visible width is exactly spec.Width.
func Card(spec CardSpec) string {
	width := clamp(spec.Width, 16)
	inner := width - 4
	frameColor := OceanBlue
	if spec.Highlight {
		frameColor = Yellow
	}

	lines := []string{
		Paint(frameColor, "+"+strings.Repeat("-", width-2)+"+"),
		framedLine(spec.Title, inner, frameColor, Style{Fg: Yellow, Bold: true}),
	}

	if spec.Subtitle != "" {
		lines = append(lines, framedLine(spec.Subtitle, inner, frameColor, Style{Fg: Seafoam}))
	}

	lines = append(lines, Paint(frameColor, "| "+strings.Repeat("-", inner)+" |"))
	for _, body := range spec.Body {
		lines = append(lines, framedLine(body, inner, frameColor, Style{Fg: White}))
	}
	if len(spec.Body) == 0 {
		lines = append(lines, framedLine("", inner, frameColor, Style{Fg: White}))
	}

	if spec.Footer != "" {
		lines = append(lines,
			Paint(frameColor, "| "+strings.Repeat("-", inner)+" |"),
			framedLine(spec.Footer, inner, frameColor, Style{Fg: Cyan}),
		)
	}

	lines = append(lines, Paint(frameColor, "+"+strings.Repeat("-", width-2)+"+"))
	return strings.Join(lines, "\n")
}

func framedLine(text string, inner int, frame Color, style Style) string {
	fit := FitStyledDisplay(text, inner)
	return Paint(frame, "| ") + style.Apply(fit) + Paint(frame, " |")
}

func clamp(value, min int) int {
	if value < min {
		return min
	}
	return value
}
