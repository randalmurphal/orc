# Codex CLI Reference — v0.98.0 (gpt-5.3-codex)

Research findings from hands-on testing and official documentation.
Verified against `codex-cli 0.98.0` on 2026-02-07.

## Verification Method

| Source | Trust Level |
|--------|-------------|
| Binary `--help` output | Ground truth |
| Actual execution tests | Ground truth |
| developers.openai.com docs | High (some gaps/inaccuracies found) |
| GitHub issues/discussions | Context (may be outdated) |
| Web search summaries | Low (often wrong about flag availability) |

**Key finding:** Multiple web sources incorrectly claim `exec resume` accepts flags like `--sandbox`, `--cd`, `--output-schema`. The binary rejects them. Always verify against the actual binary.

---

## Command Structure

```
codex [GLOBAL_OPTIONS] [PROMPT]              # Interactive TUI
codex exec [OPTIONS] [PROMPT]                # Non-interactive (alias: codex e)
codex exec resume [OPTIONS] [SESSION_ID] [PROMPT]   # Non-interactive resume
codex exec review [OPTIONS]                  # Non-interactive code review
codex resume [OPTIONS] [SESSION_ID] [PROMPT] # Interactive resume
```

---

## Flag Comparison: `exec` vs `exec resume`

### Shared (both accept)

| Flag | Purpose |
|------|---------|
| `-c, --config <key=value>` | Config override (TOML parsed) |
| `--enable <FEATURE>` | Enable feature flag |
| `--disable <FEATURE>` | Disable feature flag |
| `-i, --image <FILE>` | Attach image(s) |
| `-m, --model <MODEL>` | Model override |
| `--full-auto` | workspace-write + on-request approvals |
| `--dangerously-bypass-approvals-and-sandbox` | Skip all safety (alias: `--yolo`) |
| `--skip-git-repo-check` | Run outside git repo |
| `--json` | JSONL event stream to stdout |

### `exec` only (rejected by `exec resume`)

| Flag | Purpose | Workaround on resume |
|------|---------|---------------------|
| `-s, --sandbox <MODE>` | Sandbox policy | `--dangerously-bypass...` or `-c sandbox_mode="X"` (untested) |
| `--oss` | OSS provider shorthand | `-c model_provider="oss"` (untested) |
| `--local-provider <P>` | lmstudio / ollama | `-c model_provider="X"` (untested) |
| `-p, --profile <NAME>` | Config profile | Manual `-c` for each key |
| `-C, --cd <DIR>` | Working directory | Set `cmd.Dir` on process |
| `--add-dir <DIR>` | Additional writable dirs | Unknown |
| `--output-schema <FILE>` | JSON schema enforcement | **Session retains it** (verified) |
| `--color <MODE>` | ANSI color control | Not needed for automation |
| `-o, --output-last-message <FILE>` | Write final message to file | Parse from JSONL stream |

### `exec resume` only

| Flag | Purpose |
|------|---------|
| `--last` | Resume most recent session |
| `--all` | Include sessions from all directories |

---

## Session Behavior (Verified by Testing)

### Output schema persists across resume

**Verified 2026-02-07.** A session started with `--output-schema` retains the schema on resume without passing any schema flag:

```bash
# Start with schema
codex exec --json --output-schema schema.json --model gpt-5.3-codex "Reply with test=hello"
# → {"test":"hello"}  ✅ Schema enforced

# Resume same session, NO schema flag
codex exec resume <SESSION_ID> --json "Reply with test=world"
# → {"test":"world"}  ✅ Schema STILL enforced from original session
```

**Important:** `-c output_schema="..."` does NOT work as a config override. Tested on both `exec` and `exec resume` — the config key either doesn't exist or has a different internal name. The only way to set output schema is via the `--output-schema` CLI flag on the initial `exec` run.

### Session storage

- Location: `~/.codex/sessions/YYYY/MM/DD/*.jsonl`
- Sessions persist across process death
- Resume by UUID or `--last`
- Sessions are scoped to working directory by default (`--all` overrides)

### Session edge cases

| Scenario | Behavior |
|----------|----------|
| Resume session that completed normally | Works, continues conversation |
| Resume session interrupted by Ctrl+C | Works, transcript preserved |
| Resume session that crashed before first turn | May fail with "stream disconnected" |
| Resume with different `--model` | Works (fixed in v0.98.0) |
| Long sessions on resume | May get context-clipped, losing early context |

---

## Flag Interactions & Precedence

### Safety flag precedence (higher overrides lower)

1. `--dangerously-bypass-approvals-and-sandbox` → no sandbox, no approvals
2. `--full-auto` → workspace-write + on-request approvals (**overrides `--sandbox`**)
3. `--sandbox <MODE>` → specified sandbox, approval from config
4. Config default → read-only sandbox

**Critical:** `--full-auto` overrides `--sandbox`. Running `codex exec --full-auto --sandbox read-only` results in workspace-write, NOT read-only.

### Config override precedence

1. CLI flags (highest)
2. `-c` config overrides
3. Project `.codex/config.toml` (closest to cwd wins)
4. User `~/.codex/config.toml`
5. Built-in defaults (lowest)

---

## Model Support

### Recommended for orc

| Model | Use Case | Notes |
|-------|----------|-------|
| `gpt-5.3-codex` | Primary coding model | Requires ChatGPT auth (not API key yet) |
| `gpt-5.2-codex` | Fallback | Predecessor, still available |

### Output schema support

`--output-schema` works with gpt-5.3-codex. A previous bug (GitHub #4181) where the model guard was too narrow (`family == "gpt-5"` excluded codex variants) was fixed before v0.98.0.

### OSS provider support

- `--oss` flag → sets `model_provider=oss`
- `--local-provider lmstudio` or `--local-provider ollama`
- OSS providers may not support `--output-schema` (model-dependent)

---

## JSONL Event Stream (`--json`)

Event types emitted to stdout:

| Event | Meaning |
|-------|---------|
| `thread.started` | Session created/resumed, includes `thread_id` |
| `turn.started` | Agent turn begins |
| `item.completed` | Reasoning step or message completed |
| `turn.completed` | Agent turn ends, includes `usage` tokens |
| `error` | Error occurred |

The `item.completed` with `type: "agent_message"` contains the model's response text. When `--output-schema` is active, this text is schema-constrained JSON.

---

## Config Keys (relevant to orc integration)

| Key | Type | Values | Notes |
|-----|------|--------|-------|
| `model` | string | e.g. `"gpt-5.3-codex"` | |
| `model_provider` | string | `"openai"`, `"oss"` | |
| `sandbox_mode` | string | `"read-only"`, `"workspace-write"`, `"danger-full-access"` | |
| `approval_policy` | string | `"untrusted"`, `"on-failure"`, `"on-request"`, `"never"` | |
| `model_reasoning_effort` | string | `"minimal"`, `"low"`, `"medium"`, `"high"`, `"xhigh"` | |
| `model_verbosity` | string | `"low"`, `"medium"`, `"high"` | GPT-5 Responses API |
| `web_search` | string | `"disabled"`, `"cached"`, `"live"` | Default: `"cached"` |
| `history.persistence` | string | `"save-all"`, `"none"` | Session saving |

---

## Implications for llmkit/orc

### llmkit `buildExecArgs()` must split flags by mode

When `sessionID` is set (resume mode), the method currently emits ALL flags after the `resume` subcommand. Flags not in resume's accepted set cause `unexpected argument` errors.

**Fix:** When building resume args, skip flags that `exec resume` rejects. Specifically:
- Skip `--sandbox`, `--cd`, `--add-dir`, `--output-schema`, `--output-last-message`, `--color`, `--oss`, `--local-provider`, `--profile`
- Keep `--json`, `--model`, `--dangerously-bypass...`, `--full-auto`, `-c`, `--enable/--disable`, `--image`, `--skip-git-repo-check`
- For working directory: set `cmd.Dir` on the process instead of `--cd`

### llmkit `Resume()` method is broken and redundant

Only passes `--json`. Missing model, bypass, config overrides, everything. `Complete()` with sessionID already handles resume via `buildExecArgs()`. Delete `Resume()`.

### Output schema on resume is handled automatically

Sessions retain `--output-schema` from the original run. No flag needed on resume. llmkit should simply not pass it (which is correct since resume rejects it anyway).

### Edge case: resuming a session that never started

If codex crashes before creating a session (e.g., invalid flag), the session UUID saved by orc points to nothing. Resume will fail. orc must handle this gracefully — either detect the error and fall back to a fresh session, or clear the stored session ID on phase failure.

---

## Sources

- [Codex CLI Reference](https://developers.openai.com/codex/cli/reference/)
- [Non-interactive mode](https://developers.openai.com/codex/noninteractive/)
- [Configuration Reference](https://developers.openai.com/codex/config-reference/)
- [Codex Security](https://developers.openai.com/codex/security/)
- [Codex Changelog](https://developers.openai.com/codex/changelog/)
- [Codex Models](https://developers.openai.com/codex/models/)
- [GitHub: Resume Discussion #1076](https://github.com/openai/codex/discussions/1076)
- [GitHub: Resume fails #8256](https://github.com/openai/codex/issues/8256)
- [GitHub: output-schema model guard #4181](https://github.com/openai/codex/issues/4181)
- [GitHub: sandbox/approval on resume #5322](https://github.com/openai/codex/issues/5322)
- [GitHub: exec resume --json prompt conflict #6717](https://github.com/openai/codex/issues/6717)
- [Flag interaction details](https://www.vincentschmalbach.com/how-codex-cli-flags-actually-work-full-auto-sandbox-and-bypass/)
