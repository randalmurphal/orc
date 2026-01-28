# Quick Start - Agents Page QA Verification

## Prerequisites

The dev server must be running on http://localhost:5173

## Option 1: Start Dev Server + Run Test (Recommended)

```bash
# Terminal 1: Start dev server
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613
make web-dev

# Terminal 2: Run verification (wait for server to start first)
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web
node verify-agents-qa.mjs
```

## Option 2: Use Existing Server

If server is already running:

```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web
node verify-agents-qa.mjs
```

## Output

The script will:
- Test desktop viewport (1920x1080)
- Test mobile viewport (375x667)
- Save screenshots to worktree root:
  - `agents-page-desktop-full.png`
  - `agents-page-mobile-full.png`
- Generate JSON report: `verification-report.json`

## Expected Results

All 6 original QA issues should be FIXED:
- QA-001: Correct feature (model execution settings, not sub-agents)
- QA-002: Correct subtitle
- QA-003: "+ Add Agent" button present
- QA-004: Active Agents section with 3 cards
- QA-005: Execution Settings section with 4 controls
- QA-006: Tool Permissions section with 6 toggles

Exit code 0 = All fixed
Exit code 1 = Some issues still present
