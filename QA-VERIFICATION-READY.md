# QA Verification Test - Ready to Run

## Test Files Created

✓ **Main Test Script**: `web/verify-agents-qa.mjs`
  - Comprehensive verification of all 6 QA issues
  - Desktop (1920x1080) and mobile (375x667) testing
  - Console error detection
  - Screenshot capture
  - JSON report generation

✓ **Reference Design**: `example_ui/agents-config.png`
  - Shows expected layout and content

## How to Run

### Step 1: Ensure Dev Server is Running

```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web
npm run dev
```

Server should start on http://localhost:5173

### Step 2: Run Verification (in new terminal)

```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web
node verify-agents-qa.mjs
```

## What the Test Verifies

### Original Issues to Check

| ID | Severity | Issue | Status to Verify |
|----|----------|-------|------------------|
| QA-001 | CRITICAL | Wrong feature (sub-agent config instead of execution settings) | Should be FIXED |
| QA-002 | HIGH | Incorrect subtitle (should be "Configure Claude models and execution settings") | Should be FIXED |
| QA-003 | HIGH | Missing "+ Add Agent" button | Should be FIXED |
| QA-004 | CRITICAL | Missing "Active Agents" section with 3 cards (Primary Coder, Quick Tasks, Code Review) | Should be FIXED |
| QA-005 | CRITICAL | Missing "Execution Settings" (Parallel Tasks, Auto-Approve, Default Model, Cost Limit) | Should be FIXED |
| QA-006 | CRITICAL | Missing "Tool Permissions" (6 toggles) | Should be FIXED |

### Additional Checks

- Mobile responsiveness (no horizontal scroll on 375px)
- Console errors
- All required UI elements present and visible

## Expected Output

The script will output:

1. **Console logs** showing each test step
2. **Summary** with:
   - Number of fixed issues
   - Any remaining issues
   - New issues discovered

3. **Files generated**:
   - `agents-page-desktop-full.png` (desktop screenshot)
   - `agents-page-mobile-full.png` (mobile screenshot)
   - `verification-report.json` (detailed JSON report)

## Success Criteria

✅ All 6 original issues marked as FIXED
✅ No new critical/high severity issues
✅ Screenshots match reference design
✅ No console errors
✅ Exit code 0

## Interpreting Results

### Exit Code 0
All original issues have been fixed. The implementation matches the reference design.

### Exit Code 1
Some issues are still present. Check:
1. Console output for specific failures
2. `verification-report.json` for detailed findings
3. Screenshots for visual comparison

## Current Status

**Ready to run** - All test infrastructure in place.

Waiting for:
- Dev server to be started
- Test execution command

Once executed, results will be saved to the worktree directory with clear pass/fail indication.
