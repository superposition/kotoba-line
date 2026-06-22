package transition

import (
	"fmt"
	"strings"

	core "github.com/superposition/kotoba-line/internal/transition"
	"github.com/superposition/kotoba-line/internal/tui/atoms"
)

// RenderQueue renders every frame from queued scenes in playback order.
func RenderQueue(scenes []core.QueuedScene, width int) []string {
	rendered := make([]string, 0, frameCount(scenes))
	for _, scene := range scenes {
		for index, frame := range scene.Frames() {
			rendered = append(rendered, RenderFrame(scene, frame, index, width))
		}
	}
	return rendered
}

// RenderFrame renders a single deterministic NES/ocean transition frame.
func RenderFrame(scene core.QueuedScene, frame core.Frame, index, width int) string {
	width = minWidth(width, 32)
	bodyWidth := width - 4
	body := []string{
		atoms.StripANSI(atoms.DitherLine(bodyWidth, index*2)),
	}
	body = append(body, frame.Lines...)
	body = append(body, impactLine(scene.Definition.ID, bodyWidth, index))

	return atoms.Card(atoms.CardSpec{
		Title:     scene.Definition.Title,
		Subtitle:  fmt.Sprintf("%s %03dms/%03dms", frame.ID, scene.Definition.FrameMS, scene.Definition.DurationMS),
		Body:      body,
		Footer:    "queued NES ocean transition",
		Width:     width,
		Highlight: isImpact(scene.Definition.ID),
	})
}

func impactLine(sceneID core.SceneID, width, tick int) string {
	switch sceneID {
	case core.SceneCardMastery:
		return atoms.StripANSI(atoms.Flash("MASTER", width, 1, tick))
	case core.SceneBossIntro:
		return atoms.StripANSI(atoms.Flash("WARNING", width, 1, tick))
	case core.SceneBossCrack:
		return atoms.StripANSI(atoms.Flash("CRACK", width, 1, tick))
	case core.SceneLevelClear:
		return atoms.StripANSI(atoms.Flash("CLEAR", width, 1, tick))
	default:
		return strings.Repeat("=", width)
	}
}

func isImpact(sceneID core.SceneID) bool {
	return sceneID == core.SceneCardMastery ||
		sceneID == core.SceneBossIntro ||
		sceneID == core.SceneBossCrack ||
		sceneID == core.SceneLevelClear
}

func frameCount(scenes []core.QueuedScene) int {
	count := 0
	for _, scene := range scenes {
		count += len(scene.Definition.Frames)
	}
	return count
}

func minWidth(width, min int) int {
	if width < min {
		return min
	}
	return width
}
