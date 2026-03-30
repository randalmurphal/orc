# llmkit V2 Migration And Ownership Plan

**Date:** 2026-03-29
**Status:** Draft for agreement
**Scope:** Move orc from `github.com/randalmurphal/llmkit` v1 to `github.com/randalmurphal/llmkit/v2` and clarify ownership boundaries

## Why This Exists

orc is currently pinned to `llmkit v1.8.3` while local llmkit is already on `v2.0.0`.

The v2 release materially changes the correct boundary:

- shared root `Client` / `Request` / `Response`
- `CompleteTyped` for strict structured output
- provider-native config packages for Claude and Codex
- typed provider definitions should live here
- shared environment lifecycle helpers in `env`
- shared consumer-facing runtime settings should be translated into provider-local config here

This migration should not be treated as a dependency bump only. It is a boundary correction.

## Ownership Rules

## llmkit should own

- runtime provider contracts
- provider support definitions
- request/response typing
- structured output helpers
- runtime client construction
- provider-native config parsing and persistence
- shared-to-provider config translation
- environment mutation and restoration
- harness-specific edge cases
- provider session creation, resume, and opaque session metadata
- stream event normalization

## orc should own

- phase definitions and resolution
- orchestration loops
- orchestration-level config validation
- task/run/project persistence
- transcript ingestion into orc storage
- workflow retry and loop semantics
- quality checks and policy enforcement

## Current Orc Code Likely To Shrink Or Move

### Expected to shrink

- `internal/llmutil`
- `internal/executor/provider_adapter.go`
- `internal/executor/claude_executor.go`
- `internal/executor/codex_executor.go`
- `internal/executor/phase_settings.go`
- `internal/claude`

### Expected to stay in orc

- `internal/executor/transcript_stream.go`
- phase result parsing specific to orc workflows
- task/session persistence logic
- phase requirement validation
- cost accounting integration

## Migration Style

This migration is a hard cutover, not a compatibility rollout.

Rules:

- no long-term dual model
- no read-old/write-new compatibility layer
- no permanent aliases for old Claude-shaped names
- old names and old code paths are deleted when the new model lands

Database and config migrations may be destructive if that produces the correct structure cleanly.

## Migration Order

## Phase 1: llmkit definition contract and import migration

Goals:

- move imports to `/v2`
- replace direct v1 package references
- establish llmkit-owned provider definitions

Deliverables:

- `go.mod` updated to `github.com/randalmurphal/llmkit/v2`
- imports updated across Go packages
- llmkit exposes typed provider support definitions used by orc
- unsupported-provider assumptions removed from orc

Provider-definition contract requirements:

- typed Go API, not a generic schema/meta engine
- list/get provider-definition entrypoints
- support metadata for shared consumer-facing settings
- provider-native config ownership and validation/build entrypoints
- root `Capabilities` should be retired or folded under the typed provider-definition contract so orc has one source of truth

Implementation checklist:

- in llmkit, add typed provider-definition APIs at the root package boundary
- in llmkit, define provider definitions for `claude` and `codex` only
- in llmkit, replace or fold existing root capability tables into those definitions
- in orc, update `go.mod` and imports from `github.com/randalmurphal/llmkit` to `github.com/randalmurphal/llmkit/v2`
- in orc, update imports in:
  - `internal/executor/claude_executor.go`
  - `internal/executor/codex_executor.go`
  - `internal/executor/transcript_stream.go`
  - `internal/llmutil/*`
  - any other direct llmkit call sites found during compile repair
- in orc, remove provider constants and helper logic for unsupported providers from:
  - `internal/executor/provider.go`
  - `internal/executor/provider_adapter.go`
  - any config parsing or validation that still accepts `ollama` or `lmstudio`

Delete as part of this phase:

- unsupported-provider code paths in orc that still normalize or special-case `ollama` or `lmstudio`

## Phase 2: adopt shared root contracts

Goals:

- prefer llmkit root `Client`, `Request`, and `Response` where provider-neutral behavior is intended
- reduce direct provider branching in orc

Deliverables:

- orc-specific wrappers only where orc genuinely needs extra state
- shared request/response mapping narrowed to one place

Implementation checklist:

- in llmkit, confirm the root request/response interfaces are sufficient for both runtimes
- in llmkit, extend root response/stream types to carry opaque session metadata and normalized event data
- in orc, identify all places that build provider requests directly and collapse them toward one translation seam
- in orc, keep provider-native imports behind the thinnest possible adapter/config builder layer
- in orc, avoid spreading root llmkit request mapping across:
  - `internal/executor/claude_executor.go`
  - `internal/executor/codex_executor.go`
  - `internal/executor/provider_adapter.go`

Likely touch points:

- `internal/executor/provider_adapter.go`
- `internal/executor/claude_executor.go`
- `internal/executor/codex_executor.go`
- `internal/llmutil/schema.go`
- llmkit root `types.go` / `client.go`

## Phase 3: structured output cleanup

Goals:

- replace custom strict-schema helpers where llmkit v2 already provides the correct abstraction
- keep orc-specific phase status parsing only where it is truly workflow-specific

Notes:

- generic typed JSON completion should use llmkit strict helpers
- orc-specific status interpretation may still remain above that layer

Implementation checklist:

- in llmkit, make `CompleteTyped` or the equivalent strict typed helper the authoritative generic structured-output path
- in orc, replace generic schema-constrained helper usage with llmkit v2 strict typed helpers where the behavior is not workflow-specific
- in orc, keep only phase-specific status/result interpretation above that layer
- in orc, remove provider-specific malformed-output cleanup from:
  - `internal/executor/phase_response.go`

Delete as part of this phase:

- `unmarshalWithFallback()` if llmkit now owns malformed provider output cleanup
- any duplicated generic schema helper paths in `internal/llmutil`

## Phase 4: runtime config cutover

Goals:

- replace Claude-shaped execution config with provider-neutral runtime config
- rename schema and model fields to `runtime_config`
- remove old `claude_config` naming entirely

Deliverables:

- `PhaseRuntimeConfig` replaces `PhaseClaudeConfig`
- `runtime_config` and `runtime_config_override` replace old storage names
- old names removed from code, schema, and workflow/template handling

Design rules:

- shared runtime intent lives under a `shared` section
- provider-native behavior lives under `providers.<name>`
- shared consumer-facing settings such as `mcp_servers` are translated by llmkit into provider-local formats
- llmkit may support more provider-native fields than orc exposes; orc intentionally exposes only the subset that fits its automation model
- no permanent fallback parsing of old config names

Implementation checklist:

- define the new runtime config types in orc
- replace all in-memory uses of `PhaseClaudeConfig` with `PhaseRuntimeConfig`
- rename DB/model/workflow/template fields from `claude_config` to `runtime_config`
- update parsing, validation, serialization, and persistence in one cutover
- update any prompt/template or workflow loader code that still expects old field names
- support the full intentionally exposed field inventory up front rather than leaving obvious shared/provider-native settings behind

Likely orc touch points:

- `internal/executor/phase_config.go`
- `internal/executor/phase_settings.go`
- `internal/executor/workflow_phase.go`
- `internal/db/*` schema and model files referencing `claude_config`
- workflow/template model and storage code

Delete as part of this phase:

- `PhaseClaudeConfig`
- old `claude_config` and `claude_config_override` names in Go structs and DB/schema code
- old config parsing paths that accept the Claude-shaped model

## Phase 5: environment ownership transfer

Goals:

- migrate provider-local settings mutation toward llmkit `env` and provider-native config packages
- remove direct orc ownership of harness file formats wherever possible
- provide one shared consumer-facing preparation/scope API for supported runtimes

Likely targets:

- hooks
- MCP
- env vars
- skills
- instructions
- provider-local config files

Specific rule:

- if a setting has shared intent for llmkit consumers but provider-specific storage or file formats, llmkit owns the translation layer rather than pushing that branching into orc
- if runtime preparation needs provider-specific steps, llmkit hides that behind one shared facade rather than requiring consumers to branch on provider-specific preparation entrypoints

Implementation checklist:

- in llmkit, add or complete one shared preparation/scope API for runtime setup and cleanup
- in llmkit, move provider-local file/config mutation behind that facade
- in llmkit, own translation for shared settings such as MCP
- in orc, replace direct phase-settings mutation with calls into the llmkit preparation facade
- in llmkit, ensure the preparation facade composes with existing `env.Scope` behavior instead of creating a second environment lifecycle model

Likely llmkit surfaces:

- `env/*`
- provider-native config packages for Claude and Codex
- any root runtime-preparation API added during this work

Likely orc touch points:

- `internal/executor/phase_settings.go`
- `internal/executor/workflow_phase.go`
- any helper under `internal/claude`
- any direct `.claude/*`, `.codex/*`, `AGENTS.md`, or provider-local config mutation code

Delete as part of this phase:

- orc-local harness file editing logic once the llmkit facade covers it

## Phase 6: session and stream contract alignment

Goals:

- move provider session semantics behind llmkit
- consume llmkit-normalized stream events in orc
- persist llmkit-defined opaque session metadata rather than provider-specific fields

Known requirements from current orc behavior:

- session metadata may appear only after streaming has started and must be persisted live
- crash-safe prompt capture must remain possible before provider execution begins
- stalled stream handling must preserve enough state for resume or forced fresh retry
- provider-specific malformed-output cleanup belongs in llmkit, not orc response parsing
- transcript deduplication and stable message identity must remain explicit at the event or mapping layer

Implementation checklist:

- in llmkit, define opaque session metadata returned from start/stream/complete paths
- in llmkit, accept opaque session metadata on resume
- in llmkit, normalize streaming events for assistant output, tool activity, errors, and terminal state
- in orc, persist opaque session metadata instead of provider-specific session fields
- in orc, map normalized stream events into transcript rows and task/run state updates
- in llmkit, make session metadata updates possible before terminal completion so crash-safe resume survives mid-stream failure

Likely orc touch points:

- `internal/executor/claude_executor.go`
- `internal/executor/codex_executor.go`
- `internal/executor/transcript_stream.go`
- `internal/executor/workflow_phase.go`
- `internal/task/execution_helpers.go`
- `internal/task/proto_helpers.go`

Delete as part of this phase:

- provider-specific live session persistence logic embedded in executors
- provider-specific stream decoding logic from executor paths once llmkit normalizes the event model

## Phase 7: llmkit-first ownership enforcement

Rules:

- if behavior belongs to provider contracts, harness config, environment mutation, session semantics, stream normalization, or provider-specific edge cases, it is implemented in llmkit during this work
- orc does not keep harness-specific workaround code as a stopgap for functionality that belongs in llmkit
- if orc uncovers a missing llmkit primitive during migration, the migration includes the llmkit change rather than adding an orc-local escape hatch

Implementation checklist:

- during each workstream review every new provider-specific branch and classify it as llmkit-owned or orc-owned before merging
- if a branch is llmkit-owned, move it immediately rather than leaving a follow-up TODO in orc
- explicitly delete temporary provider-local workaround code before closing the workstream

## Non-Goals

- no compatibility layer that preserves both v1 and v2 paths long-term
- no permanent duplication of environment mutation logic across llmkit and orc
- no runtime fallback logic that silently uses old behavior if the new contract is incomplete
- no preservation of old Claude-centric config naming
- no retention of unsupported providers like `ollama` or `lmstudio` in orc
- no new provider-specific session schema fields in orc

## Decision Rules During Migration

- if behavior is execution-critical, fail explicitly
- if behavior is observability-only, best-effort is acceptable
- if behavior is harness-specific, bias toward llmkit ownership
- if behavior is workflow-specific, bias toward orc ownership

For transcript behavior specifically:

- transcript row persistence is execution-critical
- live event publishing and secondary UI feeds derived from transcript rows are observability-only

## Validation And Test Bar

This migration is not complete when it only works against a local sibling checkout through `go.work`.

Required llmkit coverage:

- provider-definition tests
- shared-setting support metadata tests
- provider-native config validation/build tests
- shared-to-provider translation tests for settings like MCP
- session metadata and stream-normalization tests

Required orc coverage:

- runtime-config parsing and strict-failure tests
- provider-selection and provider-validation tests
- session metadata persistence and resume tests
- transcript ingestion and deduplication tests
- crash-safe prompt capture tests
- fresh-retry vs resume behavior tests
- transcript persistence failure tests

Required regression coverage for identified edge cases:

- live session metadata discovered mid-stream
- malformed provider output cleanup handled in llmkit
- stalled stream state preservation
- prompt persisted before provider execution
- normalized stream events mapped into orc transcript rows correctly

Release gate:

- tag and pin llmkit first
- update orc to the tagged llmkit version
- run orc verification with `GOWORK=off`
- a workstream is not done until both repos have the necessary tests and orc passes against the tagged llmkit version

## Acceptance Criteria

- orc builds and tests against llmkit v2
- provider support definitions are owned in llmkit and consumed by orc
- provider-neutral execution code uses llmkit root contracts where possible
- generic structured output helpers are not duplicated in orc
- provider-local environment mutation is handled by llmkit
- shared settings like MCP are exposed once in the llmkit consumer contract and translated provider-locally inside llmkit
- runtime preparation and cleanup use one shared llmkit consumer-facing API with provider-specific handling hidden behind it
- provider session semantics and stream normalization are llmkit-owned, while transcript persistence remains orc-owned
- harness-specific workaround code is removed from orc rather than relocated
- old Claude-shaped config code and naming are removed
- the migration passes against tagged llmkit with `GOWORK=off`, not only through local `go.work`
