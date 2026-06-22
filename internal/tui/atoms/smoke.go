package atoms

import "strings"

// MapSmoke is an inspectable placeholder for the station-map screen.
func MapSmoke(width int) string {
	width = clamp(width, 32)
	return Card(CardSpec{
		Title:    "MAP",
		Subtitle: "Kotoba Line / Ocean Route",
		Body: []string{
			StationDots(6, 2, 4),
			StripANSI(DitherLine(width-4, 0)),
			"next: 日本 bridge",
		},
		Footer: "cyan tide / yellow stop",
		Width:  width,
	})
}

// CardSmoke is an inspectable placeholder for the skill-card screen.
func CardSmoke(width int) string {
	width = clamp(width, 32)
	return Card(CardSpec{
		Title:     "CARD",
		Subtitle:  "日",
		Body:      []string{"reading: にち / ひ", "meaning: day, sun", HPBar(7, 10, width-4)},
		Footer:    "Japanese stays double-width",
		Width:     width,
		Highlight: true,
	})
}

// DrillSmoke is an inspectable placeholder for the kana drill screen.
func DrillSmoke(width int) string {
	width = clamp(width, 40)
	body := []string{
		HPBar(8, 12, width-4),
		ComboMeter(5, 8, width-4),
		InputBar("かな", "に", width-4, true),
		Flash("HIT", width-4, 1, 1),
	}
	return Card(CardSpec{
		Title:    "DRILL",
		Subtitle: "type kana to fire",
		Body:     stripLines(body),
		Footer:   "romaji hint hidden",
		Width:    width,
	})
}

func stripLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, StripANSI(line))
	}
	return out
}

// OceanStoryboard sketches the first set of visual beats for later cutscenes.
func OceanStoryboard(width int) string {
	return Storyboard([]StoryboardStep{
		{Beat: "station", Lines: []string{"waves open", "next card rises"}},
		{Beat: "mastery", Lines: []string{"日 glows coral", "combo spills seafoam"}},
		{Beat: "clear", Lines: []string{"route dots pulse", "gate unlocks"}},
	}, width)
}

// JoinedSmoke renders all placeholder screens for quick manual inspection.
func JoinedSmoke(width int) string {
	return strings.Join([]string{
		MapSmoke(width),
		CardSmoke(width),
		DrillSmoke(width),
		OceanStoryboard(width),
	}, "\n\n")
}
