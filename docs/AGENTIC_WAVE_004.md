# Agentic Wave 004

Started after the drill playability hotfix merged.

## Goal

Move from the starter `日` deck toward document-reading gameplay:

- #15 Constitution campaign as playable levels;
- #16 station level selector;
- #17 skill-tree gates for document sections.

## Current Slice

Issue #17 is the blocking implementation slice for this wave.

Blocking scope:

- Gate document sections from replayed mastery state.
- Keep locked document targets out of normal drill play.
- Emit `level_unlocked` when prerequisite mastery opens a new station.
- Track hinted drill hits as unclean so hints cannot build mastery streaks.

Deferred:

- Hosted Railway SSH remains tracked by #10.

## Validation Plan

- `go test ./internal/content ./internal/station ./internal/tui/app`
- `go test ./...`
- `git diff --check`
- local SSH smoke:
  - open `s` and verify Article 1 starts locked;
  - master the three Article 1 prerequisite cards with clean hits;
  - verify Article 1 unlocks and appears open in the station selector;
  - verify `？` hints do not count as clean mastery hits.

## Issue #16 Closeout Notes

The station selector is the second Wave 004 slice.

Completed:

- `s` opens the station selector.
- `j`/`down` and `k`/`up` move through station levels.
- Enter travels to the selected open station.
- Locked stations stay visible and list missing prerequisite cards.
- Repeated movement keys such as `jjjj` are handled when terminals batch them
  into one key event.

Validation:

- `go test ./internal/tui/app`
- `go test ./...`
- `git diff --check`
- local SSH smoke:
  - opened `s`;
  - selected Constitution Gate with repeated `j` movement and Enter;
  - reopened `s`;
  - selected locked Emperor Symbol / Article 1;
  - verified missing prerequisites render as separate readable lines.

## Issue #17 Closeout Notes

The skill-tree gate is the third Wave 004 slice.

Completed:

- Article 1 availability is computed from replayed progress over its required
  preamble cards.
- The station selector still shows locked levels, but locked levels cannot be
  entered or spawned as drill targets.
- Three clean hits on each Article 1 prerequisite emit one durable
  `level_unlocked` event for `constitution-article-1`.
- Drill hits after `?` or `？` are written as unclean hits and do not advance
  mastery streaks.

Validation:

- `go test ./internal/tui/app ./internal/state ./internal/transition`
- `go test ./...`
- `git diff --check`
- local SSH smoke:
  - opened `s`;
  - verified Emperor Symbol / Article 1 starts locked;
  - replayed clean prerequisite mastery in tests and verified the station opens;
  - replayed hinted prerequisite hits in tests and verified the station remains
    locked.
