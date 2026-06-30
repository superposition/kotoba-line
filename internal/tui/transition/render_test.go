package transition

import (
	"strings"
	"testing"

	core "github.com/superposition/kotoba-line/internal/transition"
	"github.com/superposition/kotoba-line/internal/tui/atoms"
)

func TestRenderQueueKeepsSceneAndFrameOrder(t *testing.T) {
	scenes := core.Queue([]core.Event{
		{Kind: core.EventCardMastered, Subject: "card-hi"},
		{Kind: core.EventStationArrival, Subject: "station-01"},
		{Kind: core.EventBossIntro, Subject: "article-one"},
		{Kind: core.EventBossCrack, Subject: "article-one"},
		{Kind: core.EventLevelClear, Subject: "station-01"},
	})

	frames := RenderQueue(scenes, 44)
	if got, want := len(frames), totalFrames(scenes); got != want {
		t.Fatalf("rendered frame count = %d, want %d", got, want)
	}

	joined := strings.Join(stripFrames(frames), "\n")
	assertInOrder(t, joined, []string{
		"NEXT WORD",
		"card-hi",
		"WAVE START",
		"station-01",
		"BOSS",
		"article-one",
		"CRACK",
		"WAVE CLEAR",
	})
	for _, unwanted := range []string{"POWER UP", "weapon charge", "combo flare"} {
		if strings.Contains(joined, unwanted) {
			t.Fatalf("rendered frames should not include old charge copy %q:\n%s", unwanted, joined)
		}
	}
}

func TestRenderFrameWidthsAndNoDebugMetadata(t *testing.T) {
	scene, ok := core.SceneFor(core.SceneBossCrack, "boss-a")
	if !ok {
		t.Fatalf("missing boss crack scene")
	}

	rendered := RenderFrame(scene, scene.Frames()[1], 1, 40)
	for i, line := range strings.Split(rendered, "\n") {
		if got := atoms.DisplayWidth(line); got != 40 {
			t.Fatalf("line %d width = %d, want 40: %q", i+1, got, atoms.StripANSI(line))
		}
	}

	plain := atoms.StripANSI(rendered)
	for _, want := range []string{"CRACK", "fracture", "boss-a"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("rendered frame missing %q in:\n%s", want, plain)
		}
	}
	for _, unwanted := range []string{"queued NES ocean transition", "110ms/440ms"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("rendered frame should not expose debug metadata %q:\n%s", unwanted, plain)
		}
	}
}

func stripFrames(frames []string) []string {
	out := make([]string, 0, len(frames))
	for _, frame := range frames {
		out = append(out, atoms.StripANSI(frame))
	}
	return out
}

func totalFrames(scenes []core.QueuedScene) int {
	total := 0
	for _, scene := range scenes {
		total += len(scene.Definition.Frames)
	}
	return total
}

func assertInOrder(t *testing.T, text string, wants []string) {
	t.Helper()
	offset := 0
	for _, want := range wants {
		next := strings.Index(text[offset:], want)
		if next < 0 {
			t.Fatalf("missing %q after offset %d in:\n%s", want, offset, text)
		}
		offset += next + len(want)
	}
}
