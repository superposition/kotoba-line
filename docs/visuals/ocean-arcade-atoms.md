# Ocean Arcade TUI Atoms

Issue #4 adds dependency-free terminal atoms under `internal/tui/atoms`.

The renderers are pure string functions so later Bubble Tea screens can compose
them without owning layout math:

- `Card` for NES-style ASCII frames with Japanese-aware padding.
- `DitherLine` and `DitherFill` for deterministic ocean texture.
- `HPBar`, `ComboMeter`, `InputBar`, and `StationDots` for drill HUD pieces.
- `Flash`, `Storyboard`, and `OceanStoryboard` for arcade transition beats.
- `MapSmoke`, `CardSmoke`, `DrillSmoke`, and `JoinedSmoke` for placeholder
  screen inspection.

Palette coverage is ANSI terminal-native: ocean blue, seafoam, cyan, yellow,
coral, white, and deep navy. Tests strip ANSI before checking visible widths, so
kana and kanji like `日`, `日本`, and `かな` remain readable in fixed-width
frames.

