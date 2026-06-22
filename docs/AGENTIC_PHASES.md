# Agentic Phases

Kotoba Line should move through tickets with a main agent on the critical path
and helper agents on isolated, non-conflicting work. The goal is faster progress
without letting parallel work blur the definition of done.

## Phase 0: Ticket Intake

Purpose: make the ticket implementable before coding.

Blocking tasks:

- Identify the concrete outcome.
- Confirm acceptance criteria.
- Identify files or subsystems likely to change.
- Split obvious follow-ups out of scope.

Nonblocking agentic tasks:

- Research library/platform options.
- Draft visual copy, prompts, or ASCII concepts.
- Audit related docs or examples.

Exit criteria:

- Ticket has a clear blocking path and can be assigned.

## Phase 1: Mainline Implementation

Purpose: land the smallest useful slice.

Blocking tasks:

- Implement the core behavior.
- Keep the repo runnable.
- Avoid broad refactors outside the ticket.
- Preserve unrelated user work.

Nonblocking agentic tasks:

- Build fixtures or sample content in separate files.
- Draft docs for the feature.
- Explore optional polish paths.

Exit criteria:

- Core behavior exists and can be manually exercised.

## Phase 2: Verification

Purpose: prove the ticket works.

Blocking tasks:

- Run targeted tests or create a smoke path if tests do not exist yet.
- Manually verify the ticket's acceptance criteria.
- Record commands and results.

Nonblocking agentic tasks:

- Try alternate terminal sizes.
- Review edge cases.
- Check docs for stale instructions.
- Suggest follow-up test coverage.

Exit criteria:

- Evidence is strong enough to close or clearly classify blockers.

## Phase 3: Polish And Handoff

Purpose: make the work easy to continue from.

Blocking tasks:

- Update ticket closeout.
- Create follow-up issues for deferred work.
- Keep the roadmap honest.

Nonblocking agentic tasks:

- Improve screenshots/ASCII storyboards.
- Add extra examples.
- Draft release notes.

Exit criteria:

- The next ticket can start without rediscovering context.

## Agent Roles

Use these roles when splitting work:

- `mainline`: owns the critical code path and final integration.
- `research`: answers one concrete uncertainty without editing code.
- `content`: prepares Japanese lesson/document data in isolated files.
- `visual`: drafts ASCII, palette, animation, and transition ideas.
- `verification`: runs smoke tests and reports failures with exact commands.
- `deploy`: handles Railway/GitHub deployment once local proof exists.

## Blocking Vs Nonblocking Rule

Blocking work is anything required for the ticket acceptance criteria. It must be
finished before close.

Nonblocking work is useful but optional. It should run in parallel only when it
has a separate file scope or read-only scope. It must not hold the ticket open
unless it discovers a real blocker.

## Recommended Ticket Flow

1. Main agent claims the blocking path.
2. Helper agents take isolated nonblocking tasks.
3. Main agent integrates only results needed for acceptance.
4. Verification agent or main agent runs checks.
5. Main agent closes with evidence and follow-ups.
