# Kotoba Line Ticket Roadmap

The project should land in small playable slices. Each ticket should leave the
repo in a runnable or inspectable state.

Each ticket must follow:

- [Definition of Done](./docs/DEFINITION_OF_DONE.md)
- [Agentic Phases](./docs/AGENTIC_PHASES.md)

GitHub issue queue:

- [#1 SSH App Skeleton](https://github.com/superposition/kotoba-line/issues/1)
- [#2 Content And Card Model](https://github.com/superposition/kotoba-line/issues/2)
- [#3 Transactional State](https://github.com/superposition/kotoba-line/issues/3)
- [#4 Ocean Arcade TUI Atoms](https://github.com/superposition/kotoba-line/issues/4)
- [#5 Kana Input Shooter Loop](https://github.com/superposition/kotoba-line/issues/5)
- [#6 Skill Tree And Mastery](https://github.com/superposition/kotoba-line/issues/6)
- [#7 Document Import And Constitution Campaign](https://github.com/superposition/kotoba-line/issues/7)
- [#8 Boss Fights And Fadeaways](https://github.com/superposition/kotoba-line/issues/8)
- [#9 Station MIDI Hooks](https://github.com/superposition/kotoba-line/issues/9)
- [#10 Railway SSH Deployment](https://github.com/superposition/kotoba-line/issues/10)

## 1. SSH App Skeleton

Create the Go project, Wish SSH server, password auth, Bubble Tea session boot,
and localhost smoke path.

Acceptance:

- `go run ./cmd/kotoba-ssh` starts a server on `127.0.0.1:2222`.
- `ssh player@localhost -p 2222` opens a TUI screen.
- Wrong password is rejected.
- No shell access is exposed.

Blocking tasks:

- Scaffold Go module, server command, auth, and first Bubble Tea screen.
- Verify localhost SSH login.

Nonblocking agentic tasks:

- Research Wish deployment examples.
- Draft Railway runtime notes for the deploy ticket.

## 2. Content And Card Model

Define document, level, skill-card, reading, and campaign data models. Seed the
first cards from `2026-06-22.md`.

Acceptance:

- Seed cards exist for `日`, `日本`, `本日`, `毎日`, dates, and time words.
- Cards store kanji/text, kana reading, romaji hint, meaning, and type.
- Cards without curated kana are marked unplayable.

Blocking tasks:

- Define card/campaign data structs and seed content fixtures.
- Add loader validation for curated kana.

Nonblocking agentic tasks:

- Expand seed cards from the journal into a larger review set.
- Draft future Constitution card candidates.

## 3. Transactional State

Implement event-log persistence and deterministic reducer state.

Acceptance:

- Events append to `state/events.jsonl` locally or `/data/events.jsonl` on Railway.
- Hits, misses, streaks, mastery, unlocks, and hints are replayable.
- Snapshot generation matches event replay.

Blocking tasks:

- Implement append-only event log and reducer.
- Add replay tests for mastery and unlock state.

Nonblocking agentic tasks:

- Draft analytics events for later learning review.
- Explore compaction format for large event logs.

## 4. Ocean Arcade TUI Atoms

Build the bright ocean/NES terminal visual system.

Acceptance:

- Shared atoms exist for card frames, dither fills, HP bars, combo meter, input bar, station dots, and flashes.
- Screens use ocean blue, seafoam, cyan, yellow, coral, white, and deep navy contrast.
- Japanese text remains readable.

Blocking tasks:

- Implement reusable renderer atoms for cards, bars, dithering, and input.
- Add smoke screens for map/card/drill placeholders.

Nonblocking agentic tasks:

- Create alternate palette samples.
- Draft ocean cutscene ASCII storyboards.

## 5. Kana Input Shooter Loop

Implement the first drill-wave gameplay loop.

Acceptance:

- Falling skill-card enemies spawn.
- Player types kana with the macOS Japanese IME.
- Correct kana destroys matching enemies.
- Wrong answers do not destroy enemies.
- Romaji hint is available on demand only.

Blocking tasks:

- Implement enemy spawning, answer matching, hits, misses, and hint action.
- Verify kana input over SSH.

Nonblocking agentic tasks:

- Tune wave pacing.
- Draft additional enemy/card visual variants.

## 6. Skill Tree And Mastery

Represent card progression and unlock dependencies.

Acceptance:

- Cards progress `locked -> discovered -> training -> mastered`.
- Three correct kana hits with no miss masters a card.
- Miss resets the card streak.
- Skill tree screen shows prerequisites and next unlocks.

Blocking tasks:

- Implement dependency graph and card state transitions.
- Render skill tree progress.

Nonblocking agentic tasks:

- Draft card flavor/rank labels.
- Explore deck/binder UI variants for later.

## 7. Document Import And Constitution Campaign

Load document text as a campaign and create article-based levels.

Acceptance:

- Import command accepts local `.md` or `.txt`.
- Japanese Constitution preamble section 1 and Article 1 are seeded.
- Document levels depend on prerequisite skill cards.
- Official text is preserved; learner hints are layered separately.

Blocking tasks:

- Implement local document import.
- Seed Constitution preamble section 1 and Article 1 as campaign levels.

Nonblocking agentic tasks:

- Research official source formatting quirks.
- Prepare additional article candidates.

## 8. Boss Fights And Fadeaways

Add document bosses and major state-transition cutscenes.

Acceptance:

- Boss fights use large kanji/phrase emblems.
- Correct kana readings damage bosses.
- Card mastery, station arrival, boss intro, boss crack, and level clear have queued transitions.
- Transitions are brief and replayable from state events.

Blocking tasks:

- Implement boss state, damage, and queued transitions.
- Add card mastery and boss intro cutscenes.

Nonblocking agentic tasks:

- Draft more fadeaway effects.
- Tune explosion frames and screen-shake intensity.

## 9. Station MIDI Hooks

Add station metadata and optional MIDI playback hooks.

Acceptance:

- Local MIDI files can be registered to stations.
- Missing audio tooling does not break gameplay.
- Railway mode uses visual pulses without requiring client audio.

Blocking tasks:

- Add station metadata and optional local MIDI registration.
- Ensure missing audio tooling is safe.

Nonblocking agentic tasks:

- Collect station MIDI file candidates.
- Draft station/world mappings.

## 10. Railway SSH Deployment

Deploy the SSH app to Railway using GitHub deploys and TCP Proxy.

Acceptance:

- Public GitHub repo is connected to Railway.
- Railway service builds from the repo.
- TCP Proxy exposes the SSH port.
- Railway Volume persists `/data`.
- Hosted connection works with `ssh player@<domain> -p <port>`.

Blocking tasks:

- Create Railway service, volume, env vars, and TCP Proxy.
- Verify hosted SSH login and persistence.

Nonblocking agentic tasks:

- Draft public README deployment instructions.
- Explore future multi-user storage options.
