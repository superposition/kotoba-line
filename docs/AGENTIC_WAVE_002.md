# Agentic Wave 002

Started after PR #11 merged into `main`.

## Goal

Move from SSH skeleton to first playable foundations:

- transactional learning state;
- document import path;
- deployed SSH endpoint where possible;
- minimal kana shooter model;
- skill tree/mastery representation.

## Lanes

| Lane | Issue | Role | Blocking Scope | Nonblocking Scope |
| --- | --- | --- | --- | --- |
| A | #3 Transactional state | mainline | event log, reducer, mastery replay | snapshot/compaction notes |
| B | #7 Document import | content | local `.md`/`.txt` importer, Constitution split | extra fixtures/source notes |
| C | #10 Railway deploy | deploy | Railway service, volume, TCP Proxy, hosted SSH proof | public deployment docs |
| D | #5 Kana shooter loop | gameplay | pure drill-wave model and kana matching | pacing/visual variants |
| E | #6 Skill tree | progression | dependency graph and deterministic tree render | binder/card flavor |

## Integration Rules

- #3 owns durable progress semantics; #5 and #6 may import state but should not
  rewrite it.
- #7 owns importer behavior; content fixtures should stay source-preserving.
- #10 must not leak deployment passwords into public files, issue comments, or
  PR text.
- #5 can be marked done only if kana matching is tested and romaji remains a
  hint action.
- #6 can be marked done only if locked/discovered/training/mastered behavior is
  test-covered.

## Done Gates

- `go test ./...`
- `git diff --check`
- Local SSH smoke still passes.
- Docker build still passes.
- If Railway deploy succeeds: hosted SSH smoke and persistence proof.
- If Railway blocks: exact CLI/dashboard blocker recorded on #10.

## Expected First Merge Boundary

Prefer one integrated PR if the lanes compose cleanly. If deployment is the only
blocked lane, merge code/gameplay work separately and keep #10 open with exact
Railway follow-up steps.

## Results

| Issue | Status | Evidence |
| --- | --- | --- |
| #3 Transactional state | DONE | Event log, replay reducer, snapshots, mastery, hints, unlocks, and `/data` state-dir handling are tested. |
| #5 Kana shooter loop | DONE | SSH TUI accepts kana, reveals romaji hint on `?`, records hint/hit events, and keeps romaji as hint-only. |
| #6 Skill tree and mastery | DONE | Dependency graph, card states, next unlocks, renderer, and state adapter are tested. |
| #7 Document import | DONE | `kotoba-import` imports local `.md`/`.txt` into source-preserving document JSON and campaign JSON. |
| #10 Railway SSH deployment | BLOCKED | Railway project/service/volume exist, but source connection/deploy failed and TCP Proxy is dashboard-only with current CLI. |

## Verification Run

- `go test ./...`
- `git diff --check`
- `docker build --check .`
- `docker build -t kotoba-line:ssh .`
- `go run ./cmd/kotoba-import ...` against `internal/importer/testdata/constitution-small.txt`
- local SSH smoke: `?`, `ひ`, Enter, `q`
- local event-log check: `hint_revealed` followed by `enemy_hit`
- Docker SSH smoke with `KOTOBA_SSH_PASSWORD=local-test-password`
- Docker `/data/events.jsonl` check with `hint_revealed` followed by `enemy_hit`

## Railway State

- Project: `kotoba-line` (`b3d1fa80-9e54-4ec2-a88d-420bec09b3b7`)
- Service: `kotoba-line-ssh` (`1e700d31-3245-49d5-9087-f7d279198f52`)
- Volume: `kotoba-line-ssh-volume`, mounted at `/data`, state `READY`
- Latest deployment: `87743ddc-50e7-49a1-bfd2-da86d4e81d1d`, status `FAILED`
- Current blocker details: [Railway Wave 002 Worker C Attempt](./deploy/railway-wave-002-worker-c.md)
