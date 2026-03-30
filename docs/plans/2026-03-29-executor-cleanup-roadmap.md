# Executor Cleanup Roadmap

**Date:** 2026-03-29
**Status:** Draft for agreement
**Scope:** Break down executor cleanup into implementation-ready chunks after the runtime boundary is agreed

## Summary

The executor cleanup should happen in this order:

1. freeze target architecture
2. migrate to llmkit v2
3. replace the runtime/config contracts in one cutover
4. align session and stream contracts
5. decompose executor files and responsibilities
6. tighten correctness and remove silent fallback behavior

Doing this in the opposite order will create rework.

## Workstream 1: Architecture Freeze

Purpose:

- agree on the provider model before code churn starts

Outputs:

- approved runtime provider vs backend route terminology
- approved provider support boundary: only `claude` and `codex` in orc
- approved config rename direction
- approved ownership boundary between orc and llmkit

Done when:

- we are no longer debating whether Ollama is a runtime provider or a backend route
- we are no longer debating whether unsupported providers remain in orc
- we are no longer debating whether harness setup belongs in orc

## Workstream 2: llmkit V2 Upgrade

Purpose:

- move the dependency base first

Focus:

- import path updates
- compile fixes
- test stabilization
- llmkit-owned provider definitions
- typed provider-definition APIs that orc can consume directly

Avoid:

- broad executor redesign in the same change

Done when:

- orc builds cleanly against llmkit v2
- the current behavior still works at a high level
- orc consumes llmkit as the source of truth for provider support definitions
- provider definitions are exposed as typed Go APIs rather than an ad hoc bool map or schema engine
- orc passes against the tagged llmkit version with `GOWORK=off`

Execution checklist:

- update `go.mod` to llmkit `/v2`
- fix compile fallout in direct llmkit consumers
- add llmkit typed provider-definition APIs
- collapse the current llmkit root capability model into provider definitions
- wire orc to consume llmkit provider definitions instead of local support assumptions
- remove unsupported provider handling from orc while doing the import cutover

Likely files:

- `go.mod`
- `internal/executor/provider.go`
- `internal/executor/provider_adapter.go`
- `internal/executor/claude_executor.go`
- `internal/executor/codex_executor.go`
- `internal/llmutil/*`

## Workstream 3: Runtime Contract Cleanup

Purpose:

- stop passing Claude-shaped config through the system

Focus:

- replace `PhaseRuntimeConfig` with provider-neutral runtime config
- replace `runtime_config` naming in memory and storage in one cutover
- separate `shared` fields from `providers.<name>` sections

Key rule:

- no new functionality should be added to the old config shape
- old names and old code are removed, not preserved

Done when:

- config naming no longer encodes Claude-first architecture
- provider-specific settings are nested, not leaked into shared fields
- old config names and compatibility code are gone

Execution checklist:

- define `PhaseRuntimeConfig`
- move shared intent into `shared`
- move provider-native settings into `providers.claude` and `providers.codex`
- carry over every setting we intentionally want to support now rather than leaving partial field coverage
- rename persisted config fields to `runtime_config`
- update DB schema, models, loaders, and validators in one cutover
- delete all old Claude-shaped config code

Likely files:

- `internal/executor/phase_config.go`
- `internal/executor/phase_settings.go`
- `internal/executor/workflow_phase.go`
- DB schema/model files referencing `runtime_config`
- workflow/template storage and parsing code

## Workstream 4: Provider Support Validation

Purpose:

- move provider support definitions and provider-native validation out of ad hoc branching

Focus:

- consume llmkit provider definitions for shaping orc behavior
- validate provider-native config through llmkit
- return explicit configuration errors before execution

Examples:

- unsupported provider name in orc
- provider-native config invalid for Codex
- provider-native config invalid for Claude

Done when:

- orc is not maintaining an independent provider support matrix
- provider-specific if-statements are not the primary way unsupported config is detected

Execution checklist:

- consume llmkit provider definitions at config-validation and runtime-selection seams
- surface llmkit validation errors directly and clearly
- remove ad hoc provider support branches that duplicate llmkit decisions

Likely files:

- `internal/executor/provider.go`
- `internal/executor/provider_adapter.go`
- `internal/executor/workflow_phase.go`
- any config validation code that hardcodes provider support

## Workstream 5: Environment Lifecycle Cleanup

Purpose:

- remove direct harness file mutation from orc where llmkit can own it

Focus:

- phase settings application
- hooks
- MCP
- env vars
- skills
- instructions
- restoration and orphan recovery
- one shared llmkit environment/preparation path for supported runtimes

Done when:

- environment setup for supported runtimes is scoped and reversible through llmkit
- orc no longer hand-edits provider-local files
- shared settings such as MCP flow through one consumer-facing path and are translated inside llmkit
- orc does not branch on provider-specific preparation APIs during normal runtime setup

Execution checklist:

- add or finalize llmkit shared runtime-preparation scope API
- replace orc phase-settings mutation with the llmkit preparation call
- move MCP translation and provider-local file edits fully into llmkit
- remove orc-local harness mutation helpers
- ensure the new facade composes with existing llmkit `env.Scope` behavior and orphan recovery

Likely files:

- `internal/executor/phase_settings.go`
- `internal/executor/workflow_phase.go`
- `internal/claude/*`
- any orc code mutating `.claude/*`, `.codex/*`, or `AGENTS.md`

## Workstream 6: Executor Decomposition

Purpose:

- break the current god files into smaller units with single responsibilities

Initial targets:

- `workflow_executor.go`
- `workflow_phase.go`

Desired slices:

- phase config resolution
- runtime selection
- provider support validation
- environment setup
- session metadata persistence
- turn execution
- transcript ingestion
- phase persistence
- post-phase handling

Done when:

- runtime execution logic is readable without scanning a thousand-line file
- a provider-related change touches a small number of focused files

Execution checklist:

- extract runtime-config resolution from `workflow_phase.go`
- extract runtime selection and executor construction from `workflow_phase.go` / `provider_adapter.go`
- extract session metadata persistence from executor paths
- extract transcript persistence mapping from provider execution flow
- reduce direct cross-coupling between execution loop, persistence, and provider details

Likely files:

- `internal/executor/workflow_phase.go`
- `internal/executor/workflow_executor.go`
- `internal/executor/provider_adapter.go`
- `internal/executor/claude_executor.go`
- `internal/executor/codex_executor.go`
- `internal/executor/transcript_stream.go`

## Workstream 7: Correctness Tightening

Purpose:

- remove warn-and-continue behavior from correctness-critical paths

Target categories:

- config parse failure
- agent resolution failure
- invalid structured output handling
- environment setup failure
- task state persistence where correctness depends on it
- transcript row persistence

Allowed best-effort zones:

- live transcript publishing after persistence
- telemetry
- optional summaries
- non-authoritative UI artifacts

Done when:

- execution-critical failures are explicit errors
- best-effort behavior is limited to observability concerns
- fallback and legacy compatibility logic removed during the cutover does not reappear

Execution checklist:

- remove warn-and-continue handling for config parse and agent resolution
- fail explicitly on transcript row persistence failure
- remove generic malformed-output fallback from orc if llmkit covers it
- audit retries so they are policy-driven, not accidental fallback

Likely files:

- `internal/executor/workflow_phase.go`
- `internal/executor/phase_response.go`
- `internal/executor/transcript_stream.go`
- any execution helper that currently logs and continues on correctness-critical failures

## Workstream 8: Session And Stream Contract Alignment

Purpose:

- remove provider-specific session semantics and stream normalization from orc executor code

Focus:

- opaque llmkit-defined session metadata persistence
- live session metadata updates discovered mid-stream
- normalized stream event consumption
- crash-safe transcript capture boundaries
- retry and fresh-retry decisions based on llmkit session semantics

Done when:

- orc does not interpret provider-specific session internals
- live session metadata can be persisted safely during streaming
- transcript storage stays in orc without provider-specific stream decoding logic
- malformed provider output cleanup is not handled ad hoc in orc

Execution checklist:

- define llmkit stream event contract
- adapt Claude and Codex llmkit runtimes to emit that contract
- persist opaque session metadata in orc execution state
- remove provider-specific session update logic from executors
- keep prompt capture and transcript dedupe behavior intact during the refactor
- allow live session metadata updates before terminal completion

Likely files:

- `internal/executor/claude_executor.go`
- `internal/executor/codex_executor.go`
- `internal/executor/transcript_stream.go`
- `internal/task/execution_helpers.go`
- `internal/task/proto_helpers.go`

## Workstream 9: Provider Expansion Readiness

Purpose:

- make future provider additions predictable instead of bespoke

Definition of ready:

- adding a new runtime provider requires implementing llmkit provider contracts and provider-definition support reporting
- orc consumes the new provider through the same runtime selection and validation path
- no copy-paste executor branch is required
- provider support is explicit and tested, never implied by leftover code paths

## Workstream 10: Validation And Regression Coverage

Purpose:

- ensure the cutover is proven through both llmkit and orc tests rather than local happy-path execution

Focus:

- llmkit provider-definition and translation coverage
- llmkit session and stream normalization coverage
- orc runtime-config and strict-failure coverage
- orc session persistence, resume, retry, and transcript coverage
- regression tests for the identified edge cases
- tagged-version verification with `GOWORK=off`

Done when:

- both repos have targeted coverage for the migrated boundary
- edge cases identified during investigation have explicit regression tests
- orc passes against tagged llmkit with `GOWORK=off`

Execution checklist:

- add llmkit tests for provider definitions, validation, translation, session metadata, and stream normalization
- add orc tests for runtime-config parsing, provider validation, transcript persistence, resume, and retry behavior
- add regression tests for the identified mid-stream session and malformed-output cases
- run tagged llmkit verification with `GOWORK=off` before calling the migration complete

## Recommended Session Sequence

### Session 1

- finalize architecture docs
- lock terminology and migration rules

### Session 2

- upgrade llmkit dependency to v2
- add llmkit provider definitions consumed by orc
- repair compile/test fallout only

### Session 3

- introduce provider-neutral runtime config types
- replace old Claude-shaped config names and storage in one cutover
- remove old config code entirely

### Session 4

- wire llmkit-backed provider validation
- replace ad hoc unsupported-feature checks

### Session 5

- move environment lifecycle toward llmkit ownership
- implement missing llmkit features as part of the migration where needed

### Session 6+

- align llmkit-backed session and stream contracts
- split executor responsibilities
- tighten correctness and remove legacy fallback behavior

### Final validation

- tag llmkit and pin orc to that version
- run the targeted test suites in both repos
- run orc verification with `GOWORK=off`

## Acceptance Criteria

- cleanup work is sequenced so each step reduces ambiguity rather than introducing more
- orc becomes thinner around provider/harness concerns
- llmkit becomes the single home for harness-specific behavior
- Claude and Codex follow the same orchestration path with llmkit-defined provider differences
