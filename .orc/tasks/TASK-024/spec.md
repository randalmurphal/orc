# TASK-024: Store and Display Playwright Test Results

## Overview

Store Playwright test results separately from general attachments, with dedicated UI for viewing screenshots and test reports.

## Storage Structure

```
.orc/tasks/TASK-XXX/
├── test-results/
│   ├── screenshots/
│   │   ├── dashboard-initial.png
│   │   ├── dashboard-after-action.png
│   │   ├── task-detail-page.png
│   │   └── form-validation-error.png
│   ├── traces/
│   │   └── trace-1.zip           # Playwright trace
│   ├── report/
│   │   └── index.html            # Playwright HTML report
│   ├── results.json              # Structured test results
│   └── summary.yaml              # Quick summary
```

### summary.yaml Format

```yaml
generated_at: 2026-01-12T22:30:00Z
duration_ms: 45000
stats:
  total: 12
  passed: 10
  failed: 1
  skipped: 1
screenshots:
  - name: dashboard-initial.png
    page: /
    description: Dashboard initial load
  - name: task-detail-page.png
    page: /tasks/TASK-001
    description: Task detail with timeline
coverage:
  lines: 85.2
  branches: 78.5
failures:
  - test: "should submit form"
    error: "Element not found: #submit-btn"
    screenshot: form-validation-error.png
```

## API Endpoints

### Get Test Results Summary
```
GET /api/tasks/{id}/test-results

Response: {
  has_results: true,
  summary: { stats, duration_ms, generated_at },
  screenshots: [{ name, page, description }],
  failures: [{ test, error, screenshot }]
}
```

### List Screenshots
```
GET /api/tasks/{id}/test-results/screenshots

Response: [{ filename, page, description, size, created_at }]
```

### Get Screenshot
```
GET /api/tasks/{id}/test-results/screenshots/{filename}

Response: Image file
```

### Get Playwright Report
```
GET /api/tasks/{id}/test-results/report

Response: Redirect to or serve index.html
```

### Get Trace File
```
GET /api/tasks/{id}/test-results/traces/{filename}

Response: File download
```

## Backend Implementation

### New File: `internal/api/handlers_test_results.go`

```go
type TestResultsSummary struct {
    HasResults  bool                 `json:"has_results"`
    GeneratedAt time.Time            `json:"generated_at"`
    DurationMs  int64                `json:"duration_ms"`
    Stats       TestStats            `json:"stats"`
    Screenshots []ScreenshotInfo     `json:"screenshots"`
    Failures    []TestFailure        `json:"failures"`
}

type TestStats struct {
    Total   int `json:"total"`
    Passed  int `json:"passed"`
    Failed  int `json:"failed"`
    Skipped int `json:"skipped"`
}

type ScreenshotInfo struct {
    Filename    string `json:"filename"`
    Page        string `json:"page"`
    Description string `json:"description"`
    Size        int64  `json:"size"`
}

func (s *Server) handleGetTestResults(w, r)
func (s *Server) handleListScreenshots(w, r)
func (s *Server) handleGetScreenshot(w, r)
func (s *Server) handleGetReport(w, r)
```

## Web UI

### Task Detail - Test Results Tab

New tab alongside Timeline/Changes/Transcript/Attachments.

#### Components

**TestResultsTab.svelte**
```svelte
<script>
  let { taskId } = $props()
  let results = $state(null)

  $effect(() => {
    loadTestResults(taskId).then(r => results = r)
  })
</script>

{#if results?.has_results}
  <TestSummaryCard stats={results.stats} duration={results.duration_ms} />
  <ScreenshotGallery screenshots={results.screenshots} {taskId} />
  {#if results.failures.length > 0}
    <FailuresList failures={results.failures} />
  {/if}
  <ReportLink {taskId} />
{:else}
  <EmptyState message="No test results yet" />
{/if}
```

**ScreenshotGallery.svelte**
- Grid layout of screenshot thumbnails
- Click to open lightbox
- Show page name and description
- Before/after comparison if named with -before/-after suffix

**TestSummaryCard.svelte**
- Pass/fail/skip counts with colored indicators
- Duration
- Coverage percentage if available

**FailuresList.svelte**
- Expandable list of failed tests
- Show error message
- Link to associated screenshot

### Screenshot Lightbox

Full-screen image viewer with:
- Zoom controls
- Navigation between screenshots
- Metadata display (page, description)
- Download button

## Integration with Playwright

### Environment Variables

Set during task execution:
```bash
ORC_TASK_ID=TASK-XXX
ORC_TEST_RESULTS_DIR=/path/to/.orc/tasks/TASK-XXX/test-results
```

### Playwright Config Update

Projects can use:
```typescript
export default defineConfig({
    outputDir: process.env.ORC_TEST_RESULTS_DIR
        ? `${process.env.ORC_TEST_RESULTS_DIR}/raw`
        : './test-results',
    reporter: [
        ['html', { outputFolder: process.env.ORC_TEST_RESULTS_DIR
            ? `${process.env.ORC_TEST_RESULTS_DIR}/report`
            : './playwright-report' }],
        ['json', { outputFile: process.env.ORC_TEST_RESULTS_DIR
            ? `${process.env.ORC_TEST_RESULTS_DIR}/results.json`
            : './test-results.json' }],
    ],
});
```

### Post-Test Processing

After Playwright runs, generate summary.yaml from results.json:

```go
func GenerateTestSummary(resultsDir string) error {
    // Read results.json
    // Extract stats, failures, screenshots
    // Write summary.yaml
}
```

## Testing

- Unit tests for summary generation
- API endpoint tests
- E2E: Run task with Playwright, verify results appear in UI
