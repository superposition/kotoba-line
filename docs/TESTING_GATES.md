# Testing Gates

Kotoba Line tickets should be closed only after passing the gates that match the
slice. If a gate cannot run yet, the closeout must say why and create a follow-up
or blocker.

## Universal Gates

- `go test ./...` passes.
- `go fmt ./...` has no diff.
- New behavior has either an automated test or a documented manual smoke check.
- The ticket closeout records commands, results, files changed, and known risks.
- The repo remains runnable or inspectable after the ticket.

## SSH App Gate

- Server starts locally with `go run ./cmd/kotoba-ssh`.
- Correct password reaches the TUI.
- Wrong password is rejected.
- SSH session cannot open a shell.
- Terminal resize does not crash the app.

## Content Gate

- Seed content loads from committed fixtures.
- Playable cards have curated kana.
- Cards missing kana are rejected or marked unplayable.
- Official document text is preserved separately from learner hints.

## State Gate

- Event append is durable.
- Reducer replay is deterministic.
- Misses reset streaks.
- Three clean hits master a card.
- Snapshots match event replay.

## TUI Gate

- Screens render at common terminal sizes: 80x24 and 120x40.
- Japanese text remains visible and ungarbled.
- Visual effects are deterministic enough to test.
- ANSI styling degrades without corrupting layout.

## Gameplay Gate

- Kana input destroys only matching targets.
- Wrong answers do not advance mastery.
- Romaji hints are explicit actions and logged.
- Soft failure keeps the learning loop playable.

## Deployment Gate

- Docker build succeeds locally before Railway deploy.
- Service binds `0.0.0.0:$PORT`.
- `/data` is used for persistent state in Railway.
- Hosted SSH works through Railway TCP Proxy.
- A redeploy does not wipe player progress.
