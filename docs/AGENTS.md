# Docs

Documentation should explain the system that exists today and the decisions that are still active.

## What belongs here

- architecture that is still true
- public or maintainer-facing specs
- active implementation plans
- durable operational guidance

## What does not belong here

- file-by-file code snapshots
- line-number references
- stale plans presented like current architecture
- duplicated details that already live in code or tests

## Update Rules

- If behavior changes and a durable doc describes it, update the doc in the same work.
- Prefer short, opinionated documents over exhaustive catalogs.
- If a plan is historical, make that obvious or remove it from active references.
- Keep terminology aligned with current architecture, especially around `runtime_config`, `llmkit`, providers, and task/workflow concepts.

## Suggested Structure

Use progressive disclosure:

1. short summary
2. core invariants or decisions
3. deeper detail only where it helps a maintainer make safe changes

## Verification

- Check links and examples you touched.
- Make sure docs do not describe removed codepaths or renamed fields.
