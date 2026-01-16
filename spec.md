# Specification: Limit Playwright/Vitest workers to prevent OOM when running parallel tasks

## Problem Statement

When orc runs multiple tasks in parallel, each task may spawn Playwright MCP servers and run Vitest/Playwright tests with their own worker pools. Without explicit limits, the number of workers scales multiplicatively (N tasks × M workers each), leading to out-of-memory (OOM) conditions on machines with limited RAM.

## Success Criteria

- [ ] Playwright config sets explicit `workers` limit in local (non-CI) mode instead of `undefined`
- [ ] Vitest config includes `pool` and `poolOptions.threads.maxThreads` worker limit
- [ ] Default worker count is sensible for parallel task execution (2 workers per test runner)
- [ ] Worker limits can be overridden via environment variables for CI or user customization
- [ ] Existing E2E tests continue to pass with new worker limits
- [ ] Existing Vitest unit tests continue to pass with new worker limits

## Testing Requirements

- [ ] Integration test: Verify `npm run test` works with new Vitest limits
- [ ] Integration test: Verify `npm run e2e` works with new Playwright limits
- [ ] E2E test: Existing E2E suite passes (`make e2e`)
- [ ] Manual verification: Run `make web-test` to confirm unit tests pass

## Scope

### In Scope
- Modify `web/playwright.config.ts` to set explicit worker limit
- Modify `web/vitest.config.ts` to set worker pool configuration
- Add environment variable support for worker count customization
- Document the changes in comments

### Out of Scope
- Changes to orc's WorkerPool (orchestrator-level parallelism) - that's a different layer
- Memory profiling or monitoring infrastructure
- Dynamic worker scaling based on available memory
- Changes to web-svelte-archive configs (archived, not maintained)

## Technical Approach

The fix involves setting sensible default worker limits that work well when multiple orc tasks run in parallel, while still allowing single-task runs to utilize more resources via environment variables.

### Files to Modify

1. **`web/playwright.config.ts`**:
   - Change `workers: process.env.CI ? 1 : undefined` to `workers: process.env.CI ? 1 : parseInt(process.env.PLAYWRIGHT_WORKERS || '2', 10)`
   - Add comment explaining rationale for the limit

2. **`web/vitest.config.ts`**:
   - Add `pool: 'threads'` to explicitly use thread pool
   - Add `poolOptions.threads.maxThreads` configuration (default 2)
   - Support `VITEST_MAX_THREADS` environment variable override

## Bug Fix Analysis

### Reproduction Steps
1. Start orc orchestrator with multiple parallel tasks that involve UI testing
2. Each task runs UI tests via Playwright MCP
3. Playwright spawns `undefined` (unlimited) workers locally
4. Vitest spawns default thread count (typically based on CPU cores)
5. With N parallel tasks, total workers = N × (Playwright workers + Vitest workers)
6. System runs out of memory

### Current Behavior
- Playwright: `workers: process.env.CI ? 1 : undefined` - unlimited in local development
- Vitest: No explicit limit - defaults to CPU-based calculation (often 8+ threads)
- Result: OOM when running 3+ parallel tasks with UI testing on 64GB RAM system

### Expected Behavior
- Playwright: 2 workers by default locally (configurable via `PLAYWRIGHT_WORKERS` env var)
- Vitest: 2 max threads (configurable via `VITEST_MAX_THREADS` env var)
- Result: Bounded memory usage even with parallel orc tasks

### Root Cause
Configuration files assume single-user development context, not orc's parallel task execution model where multiple test runners compete for resources.

### Verification
1. Run `make e2e` - tests should pass
2. Run `make web-test` - tests should pass
3. Test env var override: `PLAYWRIGHT_WORKERS=4 npm run e2e` should use 4 workers
4. Test env var override: `VITEST_MAX_THREADS=4 npm run test` should use 4 threads
