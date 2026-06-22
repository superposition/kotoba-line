# Agentic Wave 003

Started after PR #12 merged into `main`.

## Goal

Finish the remaining gameplay shell and keep Railway deploy pressure separate
from local playability:

- boss fights;
- transition/fadeaway scenes;
- station MIDI hooks and safe fallbacks;
- Railway SSH deployment unblock.

## Lanes

| Lane | Issue | Role | Blocking Scope | Nonblocking Scope |
| --- | --- | --- | --- | --- |
| A | #8 Boss fights | gameplay | boss model, phrase answers, HP/damage/clear tests | future boss balancing |
| B | #8 Fadeaways | visual | queued transition scenes and deterministic frames | extra cutscene variants |
| C | #9 Station MIDI hooks | audio/content | station metadata, MIDI registration, silent fallback | real MIDI asset collection |
| D | #10 Railway deploy | deploy | hosted SSH proof or exact blocker | dashboard handoff notes |

## Integration Rules

- Boss and transition models should stay deterministic and test-first.
- MIDI hooks must not make audio a runtime requirement.
- No copyrighted station MIDI files should be committed in this wave.
- #10 must not leak deployment passwords into public files, issue comments, or
  PR text.
- If Railway remains blocked, merge local gameplay work and keep #10 open.

## Done Gates

- `go test ./...`
- `git diff --check`
- local SSH smoke still starts and accepts kana hits;
- Docker SSH smoke still starts and writes `/data/events.jsonl`;
- boss model tests prove hit, miss, damage, phase, and clear behavior;
- transition tests prove queue ordering and scene content;
- station/audio tests prove missing MIDI/tooling is safe.

## Expected Merge Boundary

Merge #8 and #9 when tests pass. Close #10 only if hosted Railway SSH has a TCP
Proxy host/port and a successful SSH smoke check.

## Closeout

Wave 003 merged three local gameplay lanes and left Railway deployment open as
an external setup blocker.

Completed:

- #8 boss state model, TUI boss mode, kana damage, boss clear, and replayable
  boss intro/crack/clear transition events.
- #8 queued transition definitions and deterministic ocean/NES frame rendering.
- #9 station metadata, optional MIDI registration, local playback planning, and
  silent visual-pulse fallbacks for missing files/tooling and Railway mode.

Railway status:

- service `kotoba-line-ssh` still has `source: null`;
- latest deployment failed at `BUILD_IMAGE`;
- no TCP Proxy exists yet;
- no hosted SSH smoke was possible.

Validation run:

- `go test ./internal/state ./internal/transition ./internal/tui/app ./internal/boss`
- `go test ./...`
- `git diff --check`
- `docker build --check .`
- `docker build -t kotoba-line:ssh .`
- local SSH smoke on `127.0.0.1:2222`:
  - `?`
  - `ひ`
  - `b`
  - `?`
  - `ひがくれる` three times
  - verified `state/events.jsonl` contained `boss_intro`, `boss_damaged`, and
    `boss_cleared`.
- Docker SSH smoke on host port `2223` with `/data` volume:
  - same scripted input sequence;
  - verified `/data/events.jsonl` contained the same boss clear sequence.
