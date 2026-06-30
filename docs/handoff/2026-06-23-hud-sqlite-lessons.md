# Handoff - HUD, SQLite Lessons, And Next Gameplay Slice

Date: 2026-06-23
Repo: `/Users/ericmanganaro/nihon`
Remote: `https://github.com/superposition/kotoba-line`

## Current State

This repo is mid-slice and intentionally dirty. Do not reset or discard local
changes. The current local game is an SSH TUI vocabulary shooter for Japanese
reading practice, with a document/skill-tree route driving progression.

The user direction is clear:

- The exercise and input must be front and center.
- Supporting information belongs in shelves/HUD context, not in the main play
  lane.
- The document is the source of the route. Lessons and words should feel like
  requirements for reading the loaded document, not a random debug deck.
- Do not show raw Mermaid/source-like tree code in the UI.
- Do not rely on romaji as the primary interaction. Romaji is a hint only.
- Do not add fake mode keys as the main progression path. The player should
  progress by answering exercises.
- The current visual standard is an ocean/NES-ish HUD, not a business dashboard.

## What Was Implemented

### HUD Run Screen

`internal/tui/app/model.go` now renders the run screen as:

- top status line: player, drill/boss mode, score, hull, combo;
- central `EXERCISE` panel:
  - task;
  - target;
  - meaning;
  - fixed-width input bar;
  - kana sound preview;
  - hint line;
  - feedback;
  - mastery meter;
- lower `SHELVES` area:
  - `DOCUMENT`: document title, current level, campaign path;
  - `TREE`: current/open/locked route nodes and next unlock requirement.

This replaced the confusing layout where map/progress/debug text came before
the actual prompt.

### SQLite Lesson Content

SQLite now stores real lessons, not just user progress:

- `internal/state/sqlite.go`
  - `users`
  - `events`
  - `lessons`
  - `lesson_cards`
- `internal/state/lessons.go`
  - seeds `日 Foundation`;
  - loads a `content.Library` from SQLite.

Seeded lessons:

- `Lesson 1 - 日 Readings`
  - `日/ひ`
  - `日/にち`
  - `日/び`
  - `日/か`
- `Lesson 2 - 日 Words And Dates`
  - `日本/にほん`
  - `日本/にっぽん`
  - `本日/ほんじつ`
  - `毎日/まいにち`
  - date forms like `1日/ついたち`, `2日/ふつか`, `20日/はつか`
- `Lesson 3 - 日 Sentences`
  - `日が暮れる`
  - `日暮里で毎日日が暮れる`
  - `春日町でも日が暮れる`
  - `本日は誠にありがとうございました`

The SSH server seeds/loads these SQLite lessons on startup and starts from the
campaign start level instead of hardcoding the older JSON starter level when a
library is provided.

### Gameplay/State Improvements

- Drill spawning now has pseudo-random variety and avoids repeating the current
  or immediately previous card when alternatives exist.
- `?` and `？` both reveal hints.
- Keyboard romaji input previews kana but still submits against kana matching.
- Hits, misses, hints, unlocks, boss events, and mastery remain replayable.
- The app uses SQLite as the default event store for the SSH server path.

## Verification Already Run

Commands run successfully:

```sh
go test ./internal/tui/app ./internal/sshapp
go test -count=1 ./...
git diff --check
```

Local SSH server was rebuilt and restarted on:

```sh
127.0.0.1:2222
```

Login:

```sh
ssh -tt -o StrictHostKeyChecking=no -o UserKnownHostsFile=/tmp/kotoba-line-known-hosts -p 2222 player@localhost
```

Password:

```text
kotoba
```

Final smoke output showed:

```text
KOTOBA HUD  player  |  DRILL  |  score 00000  hull [#####]  combo x00

+-- EXERCISE ------------------------------------------------------------------+
| task    type kana reading; enter fires                                       |
| target  日                                                                   |
| meaning date counter reading                                                 |
| [ type _                                                                   ] |
| hint    ? or ？ reveals kana/romaji                                          |
| feedback ready                                                               |
| MASTERY [..............................................................] 0/3 |
+------------------------------------------------------------------------------+

SHELVES
DOCUMENT
doc    SQLite Lesson - 日 Foundation
level  Lesson 1 - 日 Readings
path   日 Foundation
TREE
=> Lesson 1 - 日 Readings  open  0/4
   Lesson 2 - 日 Words And Dates  locked  0/12
   Lesson 3 - 日 Sentences  locked  0/4
next lesson 2 - 日 words and dates
unlock 日/ひ 0/3
```

Opening and quitting did not mutate progress:

```text
event_lines_before=75 after=75
```

## Dirty Files At Handoff

Known modified/untracked files:

```text
go.mod
go.sum
internal/boss/model.go
internal/boss/model_test.go
internal/game/drill.go
internal/game/drill_test.go
internal/sshapp/config.go
internal/sshapp/config_test.go
internal/sshapp/server.go
internal/sshapp/server_test.go
internal/state/log.go
internal/transition/scene.go
internal/transition/scene_test.go
internal/tui/app/model.go
internal/tui/app/model_test.go
internal/tui/transition/render.go
internal/tui/transition/render_test.go
internal/kana/
internal/state/lessons.go
internal/state/sqlite.go
internal/state/sqlite_test.go
docs/handoff/2026-06-23-hud-sqlite-lessons.md
```

Do not assume these are all from one tiny UI change. The current branch includes
earlier waves: kana input, SQLite state, lessons, SSH config, boss/transition
adjustments, and the HUD redesign.

## Next Agent Slice

The next useful slice is not another layout rewrite. It should make progression
feel like reading through a document.

Recommended ticket:

```text
Document Route Exercise Flow
```

Acceptance:

- The center HUD still shows only exercise/input/feedback.
- The shelf shows the current document branch and next unlock, compactly.
- New words are introduced from the current document/lesson path with weighted
  pseudo-random selection, not a fixed same-order deck.
- A mastered card should visibly move the route forward.
- Level completion should advance naturally into the next open lesson or boss
  without requiring a fake keypress.
- Tests cover:
  - first screen starts on SQLite lessons;
  - card variety does not repeat the same first target every session;
  - mastering Lesson 1 unlocks Lesson 2;
  - the HUD does not render raw Mermaid/source syntax;
  - SSH PTY smoke still sees `KOTOBA HUD`, `EXERCISE`, `[ type _`, `SHELVES`.

Suggested implementation steps:

1. Add a document-route exercise selector.
   - Prefer cards from the current level.
   - Weight undiscovered/training cards above already mastered cards.
   - Allow occasional prerequisite review, but do not let it dominate.
2. Add a compact route progress shelf.
   - Keep it shelf-only.
   - Do not move it above the exercise.
   - Render structural ASCII, not Mermaid source.
3. Make completion transitions gameplay-driven.
   - On level completion, show feedback and switch to the next unlocked level or
     boss automatically.
   - Avoid introducing `b`, `c`, or other hidden mode keys as normal play.
4. Expand SQLite lesson seeding toward Constitution reading.
   - The point is to prepare the learner for the document.
   - Keep the 日 foundation as the first route, then bridge into Constitution
     phrases like `日本国民は`, `主権`, `憲法`, `天皇`, `象徴`.
5. Re-run and record proof:
   - `go test -count=1 ./...`
   - `git diff --check`
   - local SSH smoke on port `2222`.

## Do Not Do

- Do not show raw Mermaid code in the terminal UI.
- Do not bury the prompt under the skill tree.
- Do not make a separate landing page or explainer screen.
- Do not make romaji the default answer path.
- Do not reset the SQLite state unless the user explicitly asks.
- Do not commit real copyrighted station MIDI files. Keep audio hooks optional
  and use local-only assets if needed.

## Useful Files

- `internal/tui/app/model.go` - main Bubble Tea model and HUD rendering.
- `internal/game/drill.go` - spawning and answer matching.
- `internal/kana/` - kana preview/matching helpers.
- `internal/state/sqlite.go` - SQLite event/user/lesson schema.
- `internal/state/lessons.go` - default lesson seed and SQLite library loader.
- `internal/sshapp/server.go` - SSH app startup, SQLite seed/load path.
- `docs/DEFINITION_OF_DONE.md` - closeout standard.
- `docs/AGENTIC_PHASES.md` - agent lane rules.

## Local Server Notes

The server was running at handoff as a `screen` session named:

```text
kotoba-line-ssh
```

If it needs to be refreshed:

```sh
go build -o /tmp/kotoba-line-ssh-current ./cmd/kotoba-ssh
screen -S kotoba-line-ssh -X quit 2>/dev/null || true
lsof -tiTCP:2222 -sTCP:LISTEN
```

Then kill any old listener if needed and start:

```sh
screen -dmS kotoba-line-ssh env \
  KOTOBA_SSH_HOST=127.0.0.1 \
  KOTOBA_SSH_PORT=2222 \
  KOTOBA_SSH_USER=player \
  KOTOBA_SSH_PASSWORD=kotoba \
  KOTOBA_SSH_HOST_KEY_PATH=/Users/ericmanganaro/nihon/state/ssh_host_ed25519 \
  /tmp/kotoba-line-ssh-current
```

Because future agents may be sandboxed, they may need approval to manage the
process or access host-level sockets.
