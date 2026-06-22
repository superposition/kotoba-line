# Kotoba Line

Kotoba Line is a Japanese document-reading arcade game served as an SSH app.

The player connects with a normal terminal, types kana with the macOS Japanese
input method, masters vocab cards, and unlocks document boss levels. The first
campaign target is a small slice of the Japanese Constitution.

## Design Pillars

- SSH-native: `ssh player@host -p PORT` opens the game directly.
- Japanese-first input: kana answers are normal; romaji is only a hint.
- Document campaigns: important Japanese texts become station-route levels.
- Skill cards: kanji, words, special readings, and phrase chunks level up.
- Ocean arcade style: bright ANSI colors, wave dithering, and dramatic cutscenes.

## Planned Stack

- Go
- Wish for the SSH app server
- Bubble Tea for TUI state/update/view
- Railway TCP Proxy for hosted SSH access
- Railway Volume for persistent event-log state

## Current Seed Content

- [2026-06-22.md](./2026-06-22.md): first journal lesson focused on readings of `日`.

## Ticket Roadmap

See [ROADMAP.md](./ROADMAP.md).

## Execution Contract

- [Definition of Done](./docs/DEFINITION_OF_DONE.md)
- [Agentic Phases](./docs/AGENTIC_PHASES.md)
