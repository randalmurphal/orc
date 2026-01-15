# spec - Iteration 1

## Prompt

Create a specification for this large task:

**Task**: Phase 4: Dual-run validation - Run all E2E tests against React

**Description**: ## Purpose
Run the comprehensive E2E test suite against the React implementation to verify feature parity.

## Validation Process

### Test Configuration
- Configure Playwright to run against React app (:5174)
- Use same test files as Svelte validation
- Compare results side-by-side

### Test Categories to Validate
- Board interactions (18 tests)
- Task Detail tabs (15 tests)
- Initiative management (20 tests)
- WebSocket real-time updates (12 tests)
- Filters and URL persistence (14 tests)
- Keyboard shortcuts (12 tests)
- Finalize workflow (10 tests)
- Accessibility (8 tests)

### Visual Regression
- Run visual comparison against Svelte baselines
- Document any intentional differences
- Update baselines if changes are acceptable

### Performance Comparison
- Measure initial load time
- Measure navigation transitions
- Measure WebSocket event processing
- Compare bundle sizes

## Success Criteria
- ALL E2E tests pass (100%)
- Visual regression < 0.5% diff per page
- Performance within 10% of Svelte
- Accessibility audit passes

Define:
1. Requirements and scope
2. Technical approach
3. Component breakdown (backend, frontend if applicable)
4. API design (if applicable)
5. Success criteria with explicit checkboxes
6. Testing strategy:
   - Unit tests
   - Integration tests
   - E2E tests (using Playwright MCP tools if frontend exists)

Include clear completion criteria:
- What code must be written
- What tests must pass
- What E2E scenarios must work
- What documentation must exist

Keep iterating until the specification is clear and complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

The specification is now complete. Let me provide a summary:

## Specification Complete

I've created a comprehensive specification for **Phase 4: Dual-run E2E Validation for React** at `.orc/tasks/TASK-179/spec.md`.

### Summary

**Objective:** Run all 179 E2E tests from `web/e2e/` against the React app (`:5174`) to verify feature parity with Svelte (`:5173`).

**Key Components:**

1. **Playwright Configuration** - New `web-react/playwright.config.ts` that:
   - Points to React on port 5174
   - Shares test files with Svelte (`../web/e2e`)
   - Separate snapshot directory for React baselines

2. **Test Categories** (179 total):
   - Board: 18 tests
   - Task Detail: 15 tests
   - Initiatives: 20 tests
   - WebSocket: 17 tests
   - Filters: 16 tests
   - Keyboard shortcuts: 13 tests
   - And 7 more categories...

3. **Success Criteria:**
   - 100% functional tests pass (163 tests)
   - Visual regression < 0.5% diff per page
   - Performance within 10% of Svelte
   - All axe accessibility audits pass

4. **Implementation Phases:**
   - Phase 1: Configuration setup
   - Phase 2: Selector audit
   - Phase 3: Selector fixes (main work)
   - Phase 4: Visual baseline capture
   - Phase 5: Performance analysis
   - Phase 6: Final validation

**Known Risks:**
- Environment pages are partially implemented in React (high risk)
- InitiativeDetail needs verification (medium risk)
- Selector mismatches may require React component fixes

<phase_complete>true</phase_complete>

---
Tokens: 1019822 input, 9731 output, 104536 cache_creation, 912933 cache_read
Complete: true
Blocked: false
