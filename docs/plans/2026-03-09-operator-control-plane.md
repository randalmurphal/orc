# Operator Control Plane

**Date:** 2026-03-09
**Status:** Proposed replacement for `INIT-003 Development OS`
**Scope:** Reframe orc around multi-project oversight, project-scoped discussion, and human-approved recommendations

## Goal

Make orc the control plane for serious CLI-first agent work.

That means:
- a multi-project place to see what needs attention now
- a project-level workspace for execution, review, and handoff
- discussion threads that can turn into decisions or draft tasks
- recommendations that stay out of the real backlog until a human accepts them

It does **not** mean building a browser IDE, an autonomous orchestrator, or a knowledge-graph product looking for a reason to exist.

## Product Model

The system should treat these as different objects with different behavior:

- **Initiatives**: long-lived bodies of work with vision, decisions, and criteria
- **Tasks**: executable work with verification and workflow contracts
- **Threads**: discussion spaces for exploration, review comments, and task shaping
- **Recommendations**: proposed follow-up work, risks, cleanup, or decision requests that require human acceptance

Recommendations are intentionally not tasks. They become tasks only after a human accepts them or a human chooses to discuss them in a thread and then promote them.

## Scope Boundaries

`/` is the multi-project command center. It should answer:
- what is actively running
- what is blocked on me
- what needs discussion
- what recommendations are waiting for acceptance
- what recently completed and whether verification was clean

Project-level surfaces should answer:
- what this project is doing right now
- what changed
- what is blocked
- what discussions and recommendations are tied to this project
- what exact command or context pack I should use next in Codex or Claude Code

Threads, recommendations, and execution remain project-scoped. The multi-project layer is for triage and navigation, not for mixing execution state from unrelated repos into one giant soup.

## Interaction Principles

- Orc is the control plane. Codex/Claude CLI plus the editor remain the authoring environment.
- Browser live streams are useful as observability, not as a replacement for the CLI session.
- Every active surface should have one obvious next action.
- Every blocked item should say exactly why it is blocked.
- Every recommendation should support `accept`, `reject`, and `discuss`.
- `Discuss` should prepare a structured context pack or bootstrap prompt that can be copied into a real agent CLI session.
- Agents should emit structured blockers, decision requests, and recommendations instead of burying them in prose.
- New control-plane context must flow through the existing workflow variable system and prompt variable resolver. Do not build a second ad hoc prompt-assembly path for recommendations, attention signals, or handoff packs.

## Architecture Rules

- Recommendations, thread links, and operator attention artifacts live in ProjectDB, not GlobalDB.
- The multi-project command center reads per-project summaries through existing project routing and cache layers.
- Cross-object references should use typed links or typed provenance fields, not JSON blobs or a pile of nullable columns.
- Prompt-facing control-plane context should be exposed as variables such as recommendation summaries, attention summaries, and handoff packs so workflows can include them intentionally.

## Non-Goals

- No browser IDE
- No terminal emulator as the primary workflow
- No autonomous assess/decide/act orchestrator
- No automatic creation of real backlog tasks from AI recommendations
- No heavyweight knowledge-graph UX as a product surface

## High-Leverage Work

The smallest useful loop is:

1. Work runs in CLI-first agent sessions
2. Orc shows execution, review state, and verification evidence
3. Orc captures discussion and recommendations around that work
4. A human accepts, rejects, or discusses recommendations
5. Accepted items become tasks or initiative decisions

The first wave of implementation should focus on:
- recommendation model and state machine
- structured operator attention signals
- multi-project command center and project home
- recommendation inbox and human acceptance flow
- discussion-to-task / discussion-to-decision flow
- copyable CLI bootstrap prompts and exact commands
- post-completion recommendation generation
- variable-backed prompt/context contracts for all of the above

## Migration Rule

`INIT-003 Development OS` is superseded by this plan. Existing completed foundation work can stay. Remaining planned work should either be migrated into this model or closed if it only serves the abandoned orchestrator/browser-IDE direction.
