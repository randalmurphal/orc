# TASK-022: Integrate Playwright MCP Tools in Task Execution

## Overview

When a task requires UI testing, ensure Claude has Playwright MCP tools and guidance to use them.

## Prerequisites

- TASK-020: Attachments (for storing screenshots)
- TASK-021: Frontend detection (for knowing when to enable)
- TASK-024: Test results storage (for output location)

## Implementation

### 1. Detect Playwright MCP Availability

Check if Playwright MCP server is configured:
```go
func hasPlaywrightMCP(claudeSettings *claude.Settings) bool {
    // Check .claude/settings.json or mcp_servers.json
    // Look for "playwright" or similar server name
}
```

### 2. Configure Claude Session for UI Testing

In `internal/executor/task_execution.go`:

```go
func (e *Executor) configureForUITesting(ctx context.Context, t *task.Task) error {
    if !t.UIAffected || !t.TestingRequirements.Visual {
        return nil
    }

    // Ensure Playwright MCP is available
    if !hasPlaywrightMCP(e.claudeSettings) {
        e.logger.Warn("task requires UI testing but Playwright MCP not configured",
            "task", t.ID,
            "hint", "run: /plugin install playwright")
        // Don't fail - just warn
    }

    // Set environment for test output location
    os.Setenv("ORC_TASK_ID", t.ID)
    os.Setenv("ORC_TEST_RESULTS_DIR", filepath.Join(e.taskDir, "test-results"))
}
```

### 3. Enhance Test Phase Prompt

For tasks with `ui_affected: true`, inject additional guidance:

```yaml
# templates/test.yaml additions
{{if .UIAffected}}
## UI Testing Requirements

This task affects the user interface. You MUST:

1. **Run Playwright E2E tests** for any UI changes
2. **Capture screenshots** of affected pages using Playwright MCP tools:
   - Use `browser_snapshot` to capture page state
   - Use `browser_take_screenshot` for visual documentation
3. **Verify visual appearance** matches expected design
4. **Test user interactions** (clicks, form submissions, navigation)

Screenshots will be stored in: {{.TestResultsDir}}

### Playwright MCP Tools Available
- `mcp__playwright__browser_navigate` - Go to URL
- `mcp__playwright__browser_snapshot` - Capture accessibility tree
- `mcp__playwright__browser_take_screenshot` - Save screenshot
- `mcp__playwright__browser_click` - Click element
- `mcp__playwright__browser_type` - Type text
- `mcp__playwright__browser_fill_form` - Fill form fields
{{end}}
```

### 4. Screenshot Capture Workflow

During test phase for UI tasks:

1. Claude navigates to affected pages
2. Claude captures screenshots with descriptive names
3. Screenshots saved to `.orc/tasks/TASK-XXX/test-results/screenshots/`
4. Claude verifies visual appearance
5. Results summarized in test output

### 5. Post-Test Processing

After test phase completes:

```go
func (e *Executor) processTestResults(ctx context.Context, t *task.Task) error {
    if !t.UIAffected {
        return nil
    }

    resultsDir := filepath.Join(e.taskDir, "test-results")

    // Check for screenshots
    screenshots, _ := filepath.Glob(filepath.Join(resultsDir, "screenshots", "*.png"))
    if len(screenshots) > 0 {
        e.logger.Info("captured UI screenshots", "count", len(screenshots))
    }

    // Generate summary
    return e.generateTestResultsSummary(resultsDir)
}
```

## Playwright Config Template

Provide a template for projects without Playwright config:

```typescript
// playwright.config.ts template
export default defineConfig({
    outputDir: process.env.ORC_TEST_RESULTS_DIR || './test-results',
    use: {
        screenshot: 'on',
        trace: 'retain-on-failure',
    },
});
```

## Testing

- Mock Playwright MCP in tests
- Test prompt injection for UI tasks
- Test screenshot storage workflow
- E2E: Create UI task, verify Playwright guidance appears
