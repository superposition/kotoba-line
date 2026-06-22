# Definition of Done

Every Kotoba Line ticket must leave proof that the work is complete. A ticket is
not done because code exists; it is done when the acceptance evidence is recorded
and the next ticket can safely start.

## Required For Every Ticket

- The ticket has a clear user-facing or developer-facing outcome.
- Blocking tasks are complete.
- Nonblocking agentic tasks are either complete or explicitly deferred.
- Acceptance criteria from the ticket are verified.
- Relevant tests or smoke checks were run and recorded.
- The relevant gates from [Testing Gates](./TESTING_GATES.md) passed or have a documented blocker.
- Any known limitation is written in the ticket before close.
- The repo is left runnable or inspectable.

## Evidence Standard

Each ticket closeout should include:

- commands run
- important output or result summary
- files changed
- manual verification notes
- follow-up tickets created for intentional deferrals

## Done Labels

Use these labels consistently in issue comments and closeouts:

- `BLOCKED`: cannot continue without user input, missing credentials, missing platform support, or an upstream failure.
- `NONBLOCKING`: useful work, but not required to merge or continue.
- `DONE`: acceptance criteria are met and verified.
- `DEFERRED`: explicitly moved to a later ticket.
- `RISK`: known technical or product risk that remains.

## Ticket Closeout Template

```md
## Closeout

Status: DONE | BLOCKED | PARTIAL

Blocking tasks completed:
- ...

Nonblocking tasks completed:
- ...

Deferred:
- ...

Verification:
- `command`
- manual check

Files changed:
- ...

Follow-ups:
- #...
```
