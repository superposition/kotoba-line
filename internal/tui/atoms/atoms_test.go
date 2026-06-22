package atoms

import (
	"strings"
	"testing"
)

func TestDisplayWidthCountsJapaneseGlyphs(t *testing.T) {
	if got := DisplayWidth("日本語ABC"); got != 9 {
		t.Fatalf("DisplayWidth() = %d, want 9", got)
	}

	styled := Paint(Cyan, "かな")
	if got := StripANSI(styled); got != "かな" {
		t.Fatalf("StripANSI() = %q, want kana text", got)
	}
	if got := DisplayWidth(styled); got != 4 {
		t.Fatalf("styled DisplayWidth() = %d, want 4", got)
	}
}

func TestCardKeepsDeterministicWidthsAndJapaneseText(t *testing.T) {
	out := Card(CardSpec{
		Title:    "CARD",
		Subtitle: "日本",
		Body:     []string{"reading: にほん", "ocean route"},
		Footer:   "OK",
		Width:    28,
	})

	assertLineWidths(t, out, 28)
	plain := StripANSI(out)
	for _, want := range []string{"CARD", "日本", "にほん"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("card missing %q in:\n%s", want, plain)
		}
	}
}

func TestDitherFillIsDeterministic(t *testing.T) {
	a := DitherFill(12, 3, 2)
	b := DitherFill(12, 3, 2)
	if a != b {
		t.Fatalf("DitherFill should be deterministic")
	}
	assertLineWidths(t, a, 12)
}

func TestMetersAndInputWidths(t *testing.T) {
	for name, out := range map[string]string{
		"hp":     HPBar(5, 10, 24),
		"combo":  ComboMeter(4, 8, 24),
		"input":  InputBar("かな", "にほん", 24, true),
		"flash":  Flash("HIT", 24, 3, 1),
		"routes": StationDots(5, 2, 3),
	} {
		plain := StripANSI(out)
		if strings.TrimSpace(plain) == "" {
			t.Fatalf("%s rendered empty output", name)
		}
	}

	if got := DisplayWidth(HPBar(5, 10, 24)); got != 24 {
		t.Fatalf("HPBar width = %d, want 24", got)
	}
	if got := DisplayWidth(ComboMeter(4, 8, 24)); got != 24 {
		t.Fatalf("ComboMeter width = %d, want 24", got)
	}
	if got := DisplayWidth(InputBar("かな", "にほん", 24, true)); got != 24 {
		t.Fatalf("InputBar width = %d, want 24", got)
	}

	for _, want := range []string{"HP", "05/10", "COMBO x04", "かな", "STATION"} {
		got := StripANSI(HPBar(5, 10, 24) + ComboMeter(4, 8, 24) + InputBar("かな", "にほん", 24, true) + StationDots(5, 2, 3))
		if !strings.Contains(got, want) {
			t.Fatalf("combined atoms missing %q in %q", want, got)
		}
	}
}

func TestSmokeScreensExposeExpectedPlaceholders(t *testing.T) {
	out := JoinedSmoke(42)
	assertLineWidthsAtMost(t, out, 42)

	plain := StripANSI(out)
	for _, want := range []string{"MAP", "CARD", "DRILL", "日", "かな", "station", "mastery"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("smoke output missing %q in:\n%s", want, plain)
		}
	}
}

func assertLineWidths(t *testing.T, out string, want int) {
	t.Helper()
	for i, line := range strings.Split(out, "\n") {
		if got := DisplayWidth(line); got != want {
			t.Fatalf("line %d width = %d, want %d: %q", i+1, got, want, StripANSI(line))
		}
	}
}

func assertLineWidthsAtMost(t *testing.T, out string, want int) {
	t.Helper()
	for i, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if got := DisplayWidth(line); got > want {
			t.Fatalf("line %d width = %d, want <= %d: %q", i+1, got, want, StripANSI(line))
		}
	}
}
