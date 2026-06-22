# Agentic Wave 001

Started from the open GitHub issue queue on the foundation branch.

## Goal

Advance the first foundation slice with five parallel lanes while keeping done
gates strict enough for external review.

## Lanes

| Lane | Issue | Role | Blocking Scope | Nonblocking Scope |
| --- | --- | --- | --- | --- |
| A | #1 SSH app skeleton | mainline | Go SSH server, auth, first TUI | Wish/Railway notes |
| B | #2 Content and card model | content | card structs, seed fixtures, validation | extra cards, Constitution candidates |
| C | #4 Ocean arcade TUI atoms | visual | render atoms and smoke screens | palette/storyboard variants |
| D | #7 Constitution fixtures | content | official text fixtures and hints | extra article candidates |
| E | #10 Railway deploy prep | deploy | Docker/Railway config docs | future multi-user notes |

## Integration Rules

- Do not accept overlapping file edits without review.
- Do not mark a ticket done without matching [Testing Gates](./TESTING_GATES.md).
- Prefer tests over manual checks; manual checks must include exact commands.
- If a lane produces useful but incomplete work, classify it as `PARTIAL` and
  leave the issue open.
- Push only after local checks pass on the integrated branch.

## Expected First Merge Boundary

The first merge should prove:

- a local SSH app can boot, or a clear blocker exists;
- seed learning content can load or has a tested fixture;
- visual atoms are deterministic string renderers;
- Constitution content is source-preserving;
- Railway deployment is prepared but not executed until the local server exists.

## Results

| Issue | Status | Evidence |
| --- | --- | --- |
| #1 SSH app skeleton | DONE | Local SSH and Docker SSH smoke passed; wrong passwords and remote commands were rejected. |
| #2 Content and card model | DONE | Seed cards load and validation marks missing-kana cards unplayable. |
| #4 Ocean arcade TUI atoms | DONE | Deterministic string atoms render Japanese-aware card, meter, input, station, flash, and smoke screens. |
| #7 Constitution fixtures | PARTIAL | Source-preserving Constitution fixtures exist, but the import command is not implemented yet. |
| #10 Railway deploy prep | PARTIAL | Docker/Railway config and local container smoke pass, but no Railway service/TCP Proxy is created yet. |

## Verification Run

- `go test ./...`
- `git diff --check`
- JSON parse check for seed, document, and campaign fixtures.
- TOML parse check for `railway.toml`.
- `go run ./cmd/kotoba-ssh`, then SSH correct-password, wrong-password, and remote-command checks.
- `docker build --check .`
- `docker build -t kotoba-line:ssh .`
- Docker run with `KOTOBA_SSH_PASSWORD=local-test-password`, then SSH correct-password, wrong-password, and remote-command checks.
- Docker volume host-key persistence check across container recreation.

## Review Notes

- The development password `kotoba` is accepted only on loopback hosts.
- Non-local binds such as Docker/Railway require explicit `KOTOBA_SSH_PASSWORD`.
- `state/` is ignored and was only created by local SSH smoke tests.
