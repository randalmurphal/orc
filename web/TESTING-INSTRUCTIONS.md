# Testing Instructions for Agents Page (TASK-613)

## Overview

This document provides step-by-step instructions to complete QA testing of the Agents page implementation.

## What's Already Been Done

✅ **Code Analysis Complete:**
- Reviewed all component implementations (AgentsView, AgentCard, ExecutionSettings, ToolPermissions)
- Verified all required sections and elements are present
- Confirmed CSS responsive breakpoints exist
- Validated accessibility features (ARIA labels, keyboard nav)
- **Result:** Code structure is complete and correct

## What Needs to Be Done

⏳ **Runtime Testing Required:**
- Visual verification against reference design
- Interactive element testing (sliders, toggles, dropdowns)
- Mobile viewport testing (375x667)
- Console error checking
- Screenshot comparison

## Test Artifacts Created

| File | Purpose | Location |
|------|---------|----------|
| **QA Report** | Code analysis findings | `web/QA-REPORT.md` |
| **QA Script** | Automated browser testing | `web/qa-agents-simple.mjs` |
| **E2E Test** | Playwright test suite | `web/e2e/agents-qa.spec.ts` |
| **Server Check** | Utility script | `web/check-servers.sh` |

## Step-by-Step Testing Instructions

### Prerequisites

Ensure you have:
- Node.js and Bun installed
- Playwright installed (`bun install`)
- Orc binary built (`make build` from root)

### Step 1: Start the Servers

Open **two terminal windows**.

**Terminal 1 - Backend (API Server):**
```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613
./bin/orc serve
```
Expected output:
```
🚀 Orc API server listening on http://localhost:8080
```

**Terminal 2 - Frontend (Dev Server):**
```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web
bun run dev
```
Expected output:
```
VITE v5.x.x ready in XXX ms
➜  Local:   http://localhost:5173/
```

### Step 2: Verify Servers Are Running

```bash
# Check if both servers respond
curl http://localhost:8080/api/health
curl http://localhost:5173
```

Both should return successful responses.

### Step 3: Run Automated QA Tests

```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web
node qa-agents-simple.mjs
```

**What this does:**
1. Launches headless Chrome browser
2. Navigates to http://localhost:5173/agents
3. Takes full-page screenshot (desktop 1440x900)
4. Validates all required sections present
5. Tests interactive elements (buttons, sliders, toggles)
6. Switches to mobile viewport (375x667)
7. Takes mobile screenshot
8. Checks for horizontal scrolling
9. Captures console errors
10. Generates findings report

**Output location:**
```
/tmp/qa-TASK-613/
├── agents-desktop-full.png  # Full page screenshot (desktop)
├── agents-mobile.png         # Mobile viewport screenshot
└── findings.json             # Detailed findings report
```

### Step 4: Manual Visual Comparison

1. **Open reference design:**
   ```bash
   # View the reference design
   xdg-open /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/example_ui/agents-config.png
   ```

2. **Open test screenshots:**
   ```bash
   # View desktop screenshot
   xdg-open /tmp/qa-TASK-613/agents-desktop-full.png

   # View mobile screenshot
   xdg-open /tmp/qa-TASK-613/agents-mobile.png
   ```

3. **Compare:**
   - Colors and theme
   - Spacing and padding
   - Font sizes and weights
   - Component alignment
   - Icon colors (purple, blue, green, amber)
   - Border styles
   - Shadow effects

### Step 5: Manual Interactive Testing

Open http://localhost:5173/agents in your browser and test:

**Header:**
- [ ] Click "Add Agent" button - should trigger action
- [ ] Verify button has plus icon

**Execution Settings:**
- [ ] Drag "Parallel Tasks" slider - value should update
- [ ] Click "Auto-Approve" toggle - should switch on/off
- [ ] Click "Default Model" dropdown - should show 3 options
- [ ] Drag "Cost Limit" slider - should show $ format

**Tool Permissions:**
- [ ] Toggle each permission on/off
- [ ] Disable "File Write" or "Bash Commands" - should show warning dialog
- [ ] Confirm warning - permission should disable
- [ ] Cancel warning - permission should stay enabled

**Agent Cards** (if API returns data):
- [ ] Click an agent card - should trigger selection
- [ ] Verify status badge shows (ACTIVE/IDLE)
- [ ] Verify stats display correctly
- [ ] Verify tool badges display
- [ ] Verify truncation if more than 4 tools

### Step 6: Mobile Responsiveness Testing

In browser DevTools:
1. Press `F12` to open DevTools
2. Click device toolbar (mobile icon) or press `Ctrl+Shift+M`
3. Set viewport to **375 x 667** (iPhone SE)
4. Verify:
   - [ ] No horizontal scrolling
   - [ ] Header stacks vertically
   - [ ] Agent cards show in single column
   - [ ] Settings grid becomes single column
   - [ ] All touch targets are >= 44x44px
   - [ ] Text is readable

### Step 7: Console Error Check

With DevTools open (F12):
1. Go to Console tab
2. Refresh page
3. Verify:
   - [ ] No red errors
   - [ ] No React warnings
   - [ ] No failed API calls (check Network tab)

### Step 8: Edge Case Testing

**Empty State:**
- If no agents configured, should show:
  - Icon with "agents" glyph
  - "Create your first agent" message
  - Description text

**Error State:**
- If API fails (stop backend server), should show:
  - Error icon (alert-circle)
  - "Failed to load agents" message
  - "Retry" button

**Loading State:**
- Simulate slow network (DevTools > Network > Slow 3G)
- Should show skeleton loaders while loading

## Alternative: Playwright E2E Test

If you prefer using the Playwright test framework:

```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web
bunx playwright test e2e/agents-qa.spec.ts --headed
```

**Note:** This uses the E2E sandbox setup, which may create test data.

## Interpreting Results

### Success Criteria

✅ **PASS if:**
- All sections render with correct content
- All interactive elements respond to input
- Mobile viewport displays correctly without horizontal scroll
- No console errors
- Visual appearance matches reference design

❌ **FAIL if:**
- Any section missing or incorrect
- Interactive elements don't work
- Mobile layout broken or requires horizontal scroll
- Console shows JavaScript errors
- Visual appearance significantly differs from reference

### Confidence Thresholds

Only report findings with:
- **Confidence >= 80%** - Clear, reproducible issues
- **Confidence < 80%** - Don't report (uncertain/flaky)

## Reporting Findings

If issues are found, document using this format:

```json
{
  "id": "QA-XXX",
  "severity": "critical|high|medium|low",
  "confidence": 80-100,
  "category": "functional",
  "title": "Brief description",
  "steps_to_reproduce": [
    "Step 1: ...",
    "Step 2: ...",
    "Step 3: ..."
  ],
  "expected": "What should happen",
  "actual": "What actually happened",
  "screenshot_path": "/tmp/qa-TASK-613/bug-XXX.png"
}
```

## Summary

**Estimated Time:** 15-20 minutes for complete testing

**Quick Start (Minimal Testing):**
1. Start servers (2 min)
2. Run `node qa-agents-simple.mjs` (2 min)
3. Review findings report (5 min)
4. Manual spot-check in browser (5 min)

**Thorough Testing (Complete Coverage):**
1. All steps above
2. Full manual interactive testing (10 min)
3. Mobile responsiveness testing (5 min)
4. Edge case testing (5 min)

---

**Need Help?**
- QA Report: `web/QA-REPORT.md`
- Test Script: `web/qa-agents-simple.mjs`
- Reference Design: `example_ui/agents-config.png`

**Questions?**
Check the code implementation:
- Main view: `src/components/agents/AgentsView.tsx`
- Settings: `src/components/agents/ExecutionSettings.tsx`
- Permissions: `src/components/agents/ToolPermissions.tsx`
- Cards: `src/components/agents/AgentCard.tsx`
