# LLM Runtime Architecture

**Date:** 2026-03-29
**Status:** Draft for agreement
**Scope:** Go runtime architecture for provider execution, provider-local environment handling, and future provider expansion

## Goal

Make orc consume a provider platform instead of embedding provider-specific harness behavior throughout the executor.

This means:

- Claude and Codex are first-class through the same orchestration path
- unsupported providers and invalid provider-native config fail early and explicitly
- harness-specific config files, hooks, skills, MCP wiring, instructions, and session semantics live in llmkit
- orc keeps only orchestration concerns: workflow execution, phase persistence, transcript storage, quality gates, retry, and task state
- adding a future provider follows the same shape instead of creating another ad hoc execution branch

## Concrete Orc Cleanup Targets

The current orc code areas most directly affected by this redesign are:

- `internal/executor/workflow_phase.go`
- `internal/executor/workflow_executor.go`
- `internal/executor/provider.go`
- `internal/executor/provider_adapter.go`
- `internal/executor/phase_config.go`
- `internal/executor/phase_settings.go`
- `internal/executor/claude_executor.go`
- `internal/executor/codex_executor.go`
- `internal/executor/transcript_stream.go`
- `internal/executor/phase_response.go`
- `internal/task/execution_helpers.go`
- `internal/task/proto_helpers.go`
- DB/schema/model code referencing `runtime_config`

Expected deletions during the cutover:

- unsupported-provider handling for `ollama` and `lmstudio`
- `PhaseRuntimeConfig`
- `runtime_config` / `runtime_config_override` naming
- orc-local harness file mutation once llmkit owns preparation
- provider-specific malformed-output fallback in orc
- duplicated generic structured-output helpers in orc once llmkit v2 covers them

## Core Decision

The current `provider` concept in orc mixes together three different things:

1. the execution harness
2. the model backend
3. the local environment/config ecosystem

Those must be separated.

### Proposed Terms

- **Runtime provider**: the thing that executes turns and streams events
  - examples: `claude`, `codex`
- **Backend route**: where a runtime provider sends model traffic
  - examples may exist inside a runtime provider, but orc does not model or support them independently
- **Environment provider**: the local config ecosystem and file conventions for a runtime provider
  - examples: `.claude/*`, `.codex/*`, `.agents/*`

### Supported Providers In Orc

orc supports only these runtime providers:

- `claude`
- `codex`

orc does not support:

- `ollama`
- `lmstudio`
- any other provider or route not explicitly adopted and tested in orc

If llmkit later grows additional runtime providers, they are not automatically supported by orc. Orc adopts new providers only through an explicit implementation and validation pass.

## Problems in Current Orc

### 1. The config model is Claude-shaped but cross-provider in behavior

`PhaseRuntimeConfig` currently contains Codex settings. That causes naming drift and hides which fields are shared vs provider-specific.

### 2. Runtime selection and provider support are partly hardcoded

The executor knows too much about:

- session persistence differences
- model normalization rules
- `.claude/settings.json`
- `AGENTS.md` / Codex config handling
- inline agent restrictions

Those are harness concerns, not orchestration concerns.

### 3. Environment mutation is duplicated in orc

orc currently owns worktree-local configuration mutation for:

- hooks
- skills
- MCP servers
- environment variables
- Codex instructions

llmkit v2 already has a shared `env` layer and provider-native config packages. Orc should stop re-implementing those concerns.

### 4. Provider support definitions are owned in the wrong place

orc currently embeds too much provider knowledge directly in executor code. The desired model is:

- llmkit defines what each provider supports
- llmkit validates provider-native config
- orc consumes those definitions and surfaces the errors

## Target Model

## Layering

### llmkit owns

- provider runtime clients
- provider support definitions
- shared request/response contracts
- strict structured output helpers
- provider-native environment/config mutation
- provider support reporting
- provider-native session semantics
- provider-native file and config formats

### orc owns

- workflow graph execution
- phase resolution and variable interpolation
- orchestration-level config validation
- transcript persistence into ProjectDB
- task/run/session state persistence
- retry, loop, gate, and quality-check orchestration
- cost attribution and global reporting

## Runtime Interfaces

orc should operate against provider-neutral concepts supplied by llmkit:

- `Client`
- `Request`
- `Response`
- normalized stream events
- provider definition metadata

orc may still use provider-native llmkit packages where needed, but only behind thin translation seams.

### Proposed llmkit Root Additions

The current llmkit root types are close, but the migration needs a few deliberate additions so orc can stop carrying provider-specific behavior:

- `ProviderDefinition` at the root package
- `ListProviders()` and `GetProviderDefinition(name)` at the root package
- a shared runtime-preparation facade at the root package
- opaque session metadata on root response/stream types
- a normalized stream-event contract at the root package

The current root `Capabilities` type should either be retired or narrowed under `ProviderDefinition`. The source of truth for orc should be provider definitions, not a separate free-floating capability table.

## Provider Definitions And Validation

llmkit should expose typed provider definitions for supported runtime providers. Orc consumes those definitions for:

- supported provider selection
- config shaping
- API and UI affordances
- error messaging

llmkit remains authoritative for:

- what each provider supports
- which provider-native config fields are valid
- which combinations are invalid

Upstream-first rule:

- if later work uncovers runtime or harness behavior that belongs in llmkit, implement it in llmkit first
- after that change, cut a new llmkit tag before updating orc
- do not leave orc depending on long-lived local `replace` wiring for llmkit-owned behavior

The contract should be typed Go APIs, not a generic schema engine.

Target direction:

- `ListProviders()`
- `GetProviderDefinition(name)`
- typed `ProviderDefinition` values with:
  - provider identity and support status
  - consumer-facing support metadata for shared settings
  - ownership of provider-native config types
  - validation and request-build entrypoints for provider-native config

Concrete direction:

```go
type ProviderDefinition struct {
    Name         string
    Supported    bool
    Shared       SharedSupport
    Environment  EnvironmentSupport
    NativeConfig NativeConfigDefinition
}

type SharedSupport struct {
    SystemPrompt       bool
    AppendSystemPrompt bool
    AllowedTools       bool
    DisallowedTools    bool
    Tools              bool
    Env                bool
    AddDirs            bool
    MCPServers         bool
    MaxTurns           bool
    MaxBudgetUSD       bool
}
```

This metadata should be detailed enough for orc to shape config and reject obviously unsupported combinations before execution, while llmkit remains the authoritative validator when building the request.

Design rule:

- orc should use provider definitions for shaping config and UX
- llmkit should still be the authoritative validator when a request or provider-native config is built
- shared-setting support metadata should be detailed enough for orc to avoid offering obviously invalid config, without turning llmkit into a generic reflective schema system

The enforcement model should be:

1. orc validates orchestration-level structure
2. orc selects a provider supported by orc
3. orc passes shared and provider-native runtime config to llmkit
4. llmkit validates provider support and provider-native config
5. llmkit returns explicit errors
6. orc surfaces those errors as execution/setup failures

orc should not maintain an independent provider support matrix.

## Proposed Config Shape

Replace Claude-centric naming with provider-neutral naming.

Current direction:

- `runtime_config`
- `runtime_config_override`
- `PhaseRuntimeConfig`

Target direction:

- `runtime_config`
- `runtime_config_override`
- `PhaseRuntimeConfig`

`PhaseRuntimeConfig` should contain:

- a `shared` section for genuinely provider-neutral intent
- a `providers` section for provider-native behavior

Example shape:

```json
{
  "shared": {
    "system_prompt": "...",
    "append_system_prompt": "...",
    "allowed_tools": ["Read", "Write"],
    "disallowed_tools": ["Bash"],
    "env": {"FOO": "bar"},
    "add_dirs": ["/tmp"],
    "mcp_servers": {}
  },
  "providers": {
    "claude": {
      "hooks": {},
      "agent_ref": "reviewer",
      "inline_agents": {},
      "skill_refs": []
    },
    "codex": {
      "reasoning_effort": "high"
    }
  }
}
```

The exact field list can change, but the ownership rule should not:

- genuinely provider-neutral semantics under `shared`
- harness/provider-native semantics under `providers.<name>`
- no Claude-shaped naming in the long-term model

### Proposed `PhaseRuntimeConfig` Field Inventory

This is the concrete target unless implementation uncovers a better provider-neutral split.

Important rule:

- llmkit may support more provider-native knobs than orc exposes
- orc should expose only the provider-native settings that make sense inside orc's automation model
- raw passthrough for every provider-native option is not the goal
- approval/sandbox policy remains orchestrator-owned rather than freely phase-configurable

```go
type PhaseRuntimeConfig struct {
    Shared    SharedRuntimeConfig             `json:"shared,omitempty"`
    Providers PhaseRuntimeProviderConfig      `json:"providers,omitempty"`
}

type SharedRuntimeConfig struct {
    SystemPrompt             string                            `json:"system_prompt,omitempty"`
    AppendSystemPrompt       string                            `json:"append_system_prompt,omitempty"`
    AllowedTools             []string                          `json:"allowed_tools,omitempty"`
    DisallowedTools          []string                          `json:"disallowed_tools,omitempty"`
    Tools                    []string                          `json:"tools,omitempty"`
    MCPServers               map[string]llmkit.MCPServerConfig `json:"mcp_servers,omitempty"`
    StrictMCPConfig          bool                              `json:"strict_mcp_config,omitempty"`
    MaxBudgetUSD             float64                           `json:"max_budget_usd,omitempty"`
    MaxTurns                 int                               `json:"max_turns,omitempty"`
    Env                      map[string]string                 `json:"env,omitempty"`
    AddDirs                  []string                          `json:"add_dirs,omitempty"`
}

type PhaseRuntimeProviderConfig struct {
    Claude *PhaseClaudeRuntimeConfig `json:"claude,omitempty"`
    Codex  *PhaseCodexRuntimeConfig  `json:"codex,omitempty"`
}

type PhaseClaudeRuntimeConfig struct {
    SystemPromptFile       string                     `json:"system_prompt_file,omitempty"`
    AppendSystemPromptFile string                     `json:"append_system_prompt_file,omitempty"`
    SkillRefs              []string                   `json:"skill_refs,omitempty"`
    AgentRef               string                     `json:"agent_ref,omitempty"`
    InlineAgents           map[string]InlineAgentDef  `json:"inline_agents,omitempty"`
    Hooks                  map[string][]HookMatcher   `json:"hooks,omitempty"`
}

type PhaseCodexRuntimeConfig struct {
    ReasoningEffort string `json:"reasoning_effort,omitempty"`
    WebSearchMode   string `json:"web_search_mode,omitempty"`
}
```

Rules for this inventory:

- keep only fields we intentionally support in orc
- do not expose `local_provider`, `UseOSS`, `ollama`, or `lmstudio` through orc
- do not expose raw sandbox/approval/profile/config-override escape hatches through orc unless orc deliberately adopts those behaviors end to end
- do not duplicate native repo guidance file behavior in config
- if a field belongs in llmkit but is not yet cleanly expressible, add the llmkit API during implementation rather than bending the orc config shape

### Shared-vs-Provider Rule

Use this rule when deciding where a field belongs:

- if the consumer-facing intent is the same across runtimes, it belongs in `shared`
- if the shape, lifecycle, or semantics are harness-specific, it belongs in `providers.<name>`

Applied to current known features:

- `mcp_servers` belongs in `shared`
- llmkit materializes that shared MCP intent into Claude-specific or Codex-specific config files and scoped environment state
- `hooks`, `agent_ref`, `inline_agents`, and `skill_refs` remain provider-specific because they are not yet true shared runtime concepts
- native repo guidance like `CLAUDE.md` and `AGENTS.md` remains harness-native behavior handled by llmkit, not duplicated as explicit shared runtime config

## Environment Lifecycle

orc should not manually patch provider-local environments directly once llmkit exposes the needed lifecycle primitives.

llmkit should expose a single shared consumer-facing environment/preparation facade for supported runtimes. Orc should not call separate Claude-prep and Codex-prep entrypoints directly as part of normal execution.

Target lifecycle:

1. resolve runtime requirements from phase config
2. ask llmkit to prepare a scoped runtime environment for the selected provider
3. llmkit performs any provider-specific config, file, or environment mutation behind that facade
4. execute the turn(s)
5. restore the provider-local environment through llmkit scope cleanup

This must be:

- scoped
- reversible
- idempotent
- recoverable after crash or interruption
- simple and intuitive for llmkit consumers while hiding provider-specific mechanics internally

Concrete direction:

```go
type PrepareRequest struct {
    Provider      string
    WorkDir       string
    RuntimeConfig PhaseRuntimeConfig
    Tag           string
    RecoverOrphans bool
}

type PreparedRuntime struct {
    Scope     io.Closer
    Provider  string
    Metadata  map[string]any
}

func PrepareRuntime(ctx context.Context, req PrepareRequest) (*PreparedRuntime, error)
```

Design rules:

- this facade is consumer-facing and provider-neutral
- provider-specific file/config mutation happens behind it
- cleanup must be idempotent and safe to call on partial setup
- the API should compose with existing `env.Scope` rather than bypassing it

## Session And Transcript Ownership

llmkit should own provider session semantics and the normalized streaming event contract.

orc should own:

- persisting workflow run state
- persisting phase state
- persisting transcript rows in ProjectDB
- publishing UI and event-stream updates derived from persisted transcript data

orc should not model provider-specific session internals in its schema. It should persist provider identity plus llmkit-defined opaque session metadata needed for resume.

Design rules:

- llmkit starts or resumes provider sessions
- llmkit returns opaque session metadata for persistence
- llmkit accepts that same opaque metadata on resume
- llmkit emits normalized stream events for assistant output, chunks, tool activity, errors, and terminal results
- orc maps those normalized events into its own transcript/task/run persistence model

Concrete direction:

```go
type SessionMetadata struct {
    Provider string          `json:"provider"`
    Data     json.RawMessage `json:"data"`
}

type StreamEvent struct {
    Type      StreamEventType `json:"type"`
    Session   *SessionMetadata `json:"session,omitempty"`
    MessageID string          `json:"message_id,omitempty"`
    Role      string          `json:"role,omitempty"`
    Content   string          `json:"content,omitempty"`
    ToolCall  *ToolCall       `json:"tool_call,omitempty"`
    ToolResult *ToolResult    `json:"tool_result,omitempty"`
    Usage     *TokenUsage     `json:"usage,omitempty"`
    Model     string          `json:"model,omitempty"`
    Done      bool            `json:"done,omitempty"`
    Metadata  map[string]any  `json:"metadata,omitempty"`
    Error     error           `json:"-"`
}
```

Rules:

- orc stores `SessionMetadata` opaquely
- llmkit may update session metadata mid-stream
- message identity must be explicit in the event contract
- tool calls and tool results should be normalized before they reach orc
- if the existing root `StreamChunk` is kept, it should grow into this shape rather than forcing orc to decode provider-native events itself

Known edge cases that must be preserved during the move:

- session identity may not be known until streaming has already started
- live-discovered session metadata must be persistable before the turn ends so resume survives crashes
- stalled or interrupted streams must preserve enough session state for retry or fresh-retry policy
- transcript ingestion must remain crash-safe, including user prompt capture before the provider call
- transcript deduplication and message identity rules must remain explicit
- provider-specific malformed output cleanup should move under llmkit rather than leaking into orc parsing
- transcript row persistence is execution-critical; derived live publishing is not

Rule:

- if an edge case is about how a provider session is created, resumed, streamed, or normalized, it belongs in llmkit
- if an edge case is about how orc stores, retries, displays, or gates around that output, it belongs in orc

## Future Provider Expansion Rule

A new provider is considered supported only when llmkit supplies:

- a runtime client implementing the shared root client contract
- declared provider definitions and validation
- provider-native config/environment support if the provider has local project state

Orc should not add a new provider by copying Claude/Codex executor logic and branching on strings.

llmkit support alone is not enough. Orc support additionally requires:

- explicit adoption in orc
- tests in orc
- documentation in orc

## Acceptance Criteria

- orc execution paths do not branch on provider names except at a small runtime selection seam
- environment mutation is delegated to llmkit for supported providers
- unsupported providers and invalid provider-native config fail through llmkit-backed validation before execution
- configuration naming no longer implies Claude-first behavior
- unsupported local/provider route variants are removed from orc rather than loosely modeled
- shared intent such as MCP is exposed once to llmkit consumers and materialized provider-locally inside llmkit
- provider session state is persisted in orc only as llmkit-defined opaque metadata
- transcript persistence remains in orc while stream normalization and provider session behavior move to llmkit
