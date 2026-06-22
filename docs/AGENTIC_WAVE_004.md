# Agentic Wave 004

Started after the drill playability hotfix merged.

## Goal

Move from the starter `日` deck toward document-reading gameplay:

- #15 Constitution campaign as playable levels;
- #16 station level selector;
- #17 skill-tree gates for document sections.

## Current Slice

Issue #15 is the blocking implementation slice for this wave.

Blocking scope:

- Convert the existing Constitution prep fixture into `content.Library`
  gameplay data.
- Keep official source text separate from learner cards and hints.
- Add reachable Constitution station levels to the SSH app.
- Keep the starter `日` deck available.
- Support full-width `？` as a hint key so Japanese IME users do not need to
  switch keyboards.

Deferred:

- Full level selector UI is tracked by #16.
- Mastery-gated document sections are tracked by #17.
- Hosted Railway SSH remains tracked by #10.

## Validation Plan

- `go test ./internal/content ./internal/station ./internal/tui/app`
- `go test ./...`
- `git diff --check`
- local SSH smoke:
  - press `c` to enter Constitution Gate;
  - press `？` to reveal the kana hint;
  - answer `にほんこくみんは`;
  - press `j` to return to the starter deck.
