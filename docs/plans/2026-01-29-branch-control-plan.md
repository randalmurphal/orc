# Branch Control Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add task-level branch naming and PR settings overrides with project-level defaults and frontend discoverability.

**Architecture:** Proto fields on Task for branch/PR overrides → resolution logic in executor picks task or project default → CLI flags for creation → Web UI for settings page + task edit modal.

**Tech Stack:** Go (proto, API, CLI, executor), TypeScript/React (frontend), Connect RPC

---

## Task 1: Proto - Add Branch Control Fields to Task Message

**Files:**
- Modify: `proto/orc/v1/task.proto:185-224`

**Step 1: Add fields to Task message**

Add after line 211 (after `metadata` field):

```protobuf
  // Branch control overrides
  optional string branch_name = 31;       // User-specified branch name (empty = auto from task ID)
  optional bool pr_draft = 32;            // Override PR draft mode
  repeated string pr_labels = 33;         // Override PR labels
  repeated string pr_reviewers = 34;      // Override PR reviewers
  bool pr_labels_set = 35;                // True = use pr_labels (even if empty)
  bool pr_reviewers_set = 36;             // True = use pr_reviewers (even if empty)
```

**Step 2: Commit**

```bash
git add proto/orc/v1/task.proto
git commit -m "proto(task): add branch control fields to Task message"
```

---

## Task 2: Proto - Add Fields to CreateTaskRequest

**Files:**
- Modify: `proto/orc/v1/task.proto:498-512`

**Step 1: Add fields to CreateTaskRequest**

Add after line 512 (after `metadata` field):

```protobuf
  // Branch control
  optional string branch_name = 14;
  optional bool pr_draft = 15;
  repeated string pr_labels = 16;
  repeated string pr_reviewers = 17;
  optional bool pr_labels_set = 18;
  optional bool pr_reviewers_set = 19;
```

**Step 2: Commit**

```bash
git add proto/orc/v1/task.proto
git commit -m "proto(task): add branch control fields to CreateTaskRequest"
```

---

## Task 3: Proto - Add Fields to UpdateTaskRequest

**Files:**
- Modify: `proto/orc/v1/task.proto:519-530` (approximate, after existing fields)

**Step 1: Find UpdateTaskRequest and add fields**

Add after existing fields in UpdateTaskRequest:

```protobuf
  // Branch control (branch_name only modifiable before execution)
  optional string branch_name = 14;
  optional bool pr_draft = 15;
  repeated string pr_labels = 16;
  repeated string pr_reviewers = 17;
  optional bool pr_labels_set = 18;
  optional bool pr_reviewers_set = 19;
```

**Step 2: Commit**

```bash
git add proto/orc/v1/task.proto
git commit -m "proto(task): add branch control fields to UpdateTaskRequest"
```

---

## Task 4: Regenerate Proto

**Files:**
- Generated: `gen/proto/orc/v1/task.pb.go`, `web/src/gen/orc/v1/task_pb.ts`

**Step 1: Run proto generation**

```bash
make proto
```

**Step 2: Verify generation succeeded**

```bash
grep -n "BranchName" gen/proto/orc/v1/task.pb.go | head -5
grep -n "branchName" web/src/gen/orc/v1/task_pb.ts | head -5
```

Expected: Lines showing the new fields in generated code.

**Step 3: Commit**

```bash
git add gen/ web/src/gen/
git commit -m "chore: regenerate proto (branch control fields)"
```

---

## Task 5: Backend - Add Proto Helper Functions

**Files:**
- Modify: `internal/task/proto_helpers.go`

**Step 1: Add getter/setter helpers**

Add near other branch-related helpers (around line 588):

```go
// GetBranchNameProto returns the user-specified branch name or empty string.
func GetBranchNameProto(t *orcv1.Task) string {
	if t.BranchName != nil {
		return *t.BranchName
	}
	return ""
}

// SetBranchNameProto sets the user-specified branch name.
func SetBranchNameProto(t *orcv1.Task, name string) {
	t.BranchName = &name
}

// GetPRDraftProto returns the PR draft override or nil if not set.
func GetPRDraftProto(t *orcv1.Task) *bool {
	return t.PrDraft
}

// SetPRDraftProto sets the PR draft override.
func SetPRDraftProto(t *orcv1.Task, draft bool) {
	t.PrDraft = &draft
}

// GetPRLabelsProto returns PR label overrides. Check PrLabelsSet to determine if set.
func GetPRLabelsProto(t *orcv1.Task) []string {
	return t.PrLabels
}

// SetPRLabelsProto sets PR label overrides.
func SetPRLabelsProto(t *orcv1.Task, labels []string) {
	t.PrLabels = labels
	t.PrLabelsSet = true
}

// ClearPRLabelsProto clears PR label overrides (reverts to project default).
func ClearPRLabelsProto(t *orcv1.Task) {
	t.PrLabels = nil
	t.PrLabelsSet = false
}

// GetPRReviewersProto returns PR reviewer overrides. Check PrReviewersSet to determine if set.
func GetPRReviewersProto(t *orcv1.Task) []string {
	return t.PrReviewers
}

// SetPRReviewersProto sets PR reviewer overrides.
func SetPRReviewersProto(t *orcv1.Task, reviewers []string) {
	t.PrReviewers = reviewers
	t.PrReviewersSet = true
}

// ClearPRReviewersProto clears PR reviewer overrides (reverts to project default).
func ClearPRReviewersProto(t *orcv1.Task) {
	t.PrReviewers = nil
	t.PrReviewersSet = false
}
```

**Step 2: Run tests**

```bash
go test ./internal/task/... -v -run TestProto
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/task/proto_helpers.go
git commit -m "feat(task): add branch control proto helper functions"
```

---

## Task 6: Backend - Add Branch Name Resolution

**Files:**
- Modify: `internal/executor/branch.go`

**Step 1: Add ResolveBranchName function**

Add after existing resolution functions:

```go
// ResolveBranchName returns the branch name for a task.
// Priority: task.BranchName > auto-generated from task ID.
func ResolveBranchName(t *orcv1.Task, gitSvc *git.Git, initiativePrefix string) string {
	// Check for user-specified branch name
	if t.BranchName != nil && *t.BranchName != "" {
		return *t.BranchName
	}
	// Fall back to auto-generated name
	return gitSvc.BranchNameWithInitiativePrefix(t.Id, initiativePrefix)
}
```

**Step 2: Write test**

Create test in `internal/executor/branch_test.go` (or add to existing):

```go
func TestResolveBranchName(t *testing.T) {
	tests := []struct {
		name             string
		taskBranchName   *string
		initiativePrefix string
		want             string
	}{
		{
			name:           "uses task branch name when set",
			taskBranchName: ptr("feature/custom-branch"),
			want:           "feature/custom-branch",
		},
		{
			name:           "falls back to auto-generated when not set",
			taskBranchName: nil,
			want:           "orc/TASK-001", // default pattern
		},
		{
			name:             "falls back with initiative prefix",
			taskBranchName:   nil,
			initiativePrefix: "auth/",
			want:             "auth/TASK-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &orcv1.Task{Id: "TASK-001", BranchName: tt.taskBranchName}
			// Create mock git service or use real one with temp dir
			// ... test implementation
		})
	}
}

func ptr(s string) *string { return &s }
```

**Step 3: Run test**

```bash
go test ./internal/executor/... -v -run TestResolveBranchName
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/executor/branch.go internal/executor/branch_test.go
git commit -m "feat(executor): add ResolveBranchName for task branch resolution"
```

---

## Task 7: Backend - Add PR Options Resolution

**Files:**
- Modify: `internal/executor/workflow_completion.go`

**Step 1: Add ResolvePROptions function**

Add before `createPR` function:

```go
// ResolvePROptions builds PR creation options with task overrides applied.
func ResolvePROptions(t *orcv1.Task, cfg *config.Config) hosting.PRCreateOptions {
	prCfg := cfg.Completion.PR

	opts := hosting.PRCreateOptions{
		Draft:               prCfg.Draft,
		Labels:              prCfg.Labels,
		Reviewers:           prCfg.Reviewers,
		TeamReviewers:       prCfg.TeamReviewers,
		Assignees:           prCfg.Assignees,
		MaintainerCanModify: prCfg.MaintainerCanModify,
	}

	// Apply task-level overrides
	if t.PrDraft != nil {
		opts.Draft = *t.PrDraft
	}
	if t.PrLabelsSet {
		opts.Labels = t.PrLabels
	}
	if t.PrReviewersSet {
		opts.Reviewers = t.PrReviewers
	}

	return opts
}
```

**Step 2: Update createPR to use ResolvePROptions**

Replace the existing PR options construction in `createPR` (around line 224-234):

```go
	// Build PR options from config with task overrides
	prOpts := ResolvePROptions(t, we.orcConfig)

	description := task.GetDescriptionProto(t)
	body := fmt.Sprintf("## Task: %s\n\n%s\n\n---\nCreated by orc workflow execution.",
		t.Title, description)
	prTitle := fmt.Sprintf("[orc] %s: %s", t.Id, t.Title)

	pr, err := provider.CreatePR(ctx, hosting.PRCreateOptions{
		Title:               prTitle,
		Body:                body,
		Head:                t.Branch,
		Base:                targetBranch,
		Draft:               prOpts.Draft,
		Labels:              prOpts.Labels,
		Reviewers:           prOpts.Reviewers,
		TeamReviewers:       prOpts.TeamReviewers,
		Assignees:           prOpts.Assignees,
		MaintainerCanModify: prOpts.MaintainerCanModify,
	})
```

**Step 3: Write test**

Add to `internal/executor/workflow_completion_test.go`:

```go
func TestResolvePROptions(t *testing.T) {
	cfg := &config.Config{
		Completion: config.CompletionConfig{
			PR: config.PRConfig{
				Draft:     false,
				Labels:    []string{"automated"},
				Reviewers: []string{"alice"},
			},
		},
	}

	tests := []struct {
		name      string
		task      *orcv1.Task
		wantDraft bool
		wantLabels []string
		wantReviewers []string
	}{
		{
			name:          "uses config defaults",
			task:          &orcv1.Task{},
			wantDraft:     false,
			wantLabels:    []string{"automated"},
			wantReviewers: []string{"alice"},
		},
		{
			name: "task overrides draft",
			task: &orcv1.Task{PrDraft: ptr(true)},
			wantDraft: true,
			wantLabels: []string{"automated"},
			wantReviewers: []string{"alice"},
		},
		{
			name: "task overrides labels (empty clears)",
			task: &orcv1.Task{PrLabels: []string{}, PrLabelsSet: true},
			wantDraft: false,
			wantLabels: []string{},
			wantReviewers: []string{"alice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ResolvePROptions(tt.task, cfg)
			if opts.Draft != tt.wantDraft {
				t.Errorf("Draft = %v, want %v", opts.Draft, tt.wantDraft)
			}
			// ... more assertions
		})
	}
}

func ptr(b bool) *bool { return &b }
```

**Step 4: Run tests**

```bash
go test ./internal/executor/... -v -run TestResolvePROptions
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/executor/workflow_completion.go internal/executor/workflow_completion_test.go
git commit -m "feat(executor): add ResolvePROptions with task-level overrides"
```

---

## Task 8: Backend - Update Worktree Setup to Use Branch Name

**Files:**
- Modify: `internal/executor/workflow_completion.go:279-315`

**Step 1: Update setupWorktree to use ResolveBranchName**

In `setupWorktree`, replace the branch name calculation (around line 300):

```go
	// Calculate and set task branch using resolution logic
	var initiativePrefix string
	initiativeID := task.GetInitiativeIDProto(t)
	if initiativeID != "" {
		if init, loadErr := we.backend.LoadInitiative(initiativeID); loadErr == nil && init != nil {
			initiativePrefix = init.BranchPrefix
		}
	}

	// Use resolution function that respects task.BranchName override
	t.Branch = ResolveBranchName(t, we.gitOps, initiativePrefix)
	if err := we.backend.SaveTask(t); err != nil {
		we.logger.Warn("failed to save task branch", "task_id", t.Id, "error", err)
	}
```

**Step 2: Run existing tests**

```bash
go test ./internal/executor/... -v -run TestSetup
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/executor/workflow_completion.go
git commit -m "feat(executor): use ResolveBranchName in worktree setup"
```

---

## Task 9: Backend - Update API CreateTask Handler

**Files:**
- Modify: `internal/api/task_server.go`

**Step 1: Find CreateTask handler and add field copying**

In the `CreateTask` function, after creating the task and before saving, add:

```go
	// Branch control fields
	if req.Msg.BranchName != nil && *req.Msg.BranchName != "" {
		// Validate branch name
		if err := git.ValidateBranchName(*req.Msg.BranchName); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("invalid branch name: %w", err))
		}
		t.BranchName = req.Msg.BranchName
	}
	if req.Msg.PrDraft != nil {
		t.PrDraft = req.Msg.PrDraft
	}
	if req.Msg.PrLabelsSet != nil && *req.Msg.PrLabelsSet {
		t.PrLabels = req.Msg.PrLabels
		t.PrLabelsSet = true
	}
	if req.Msg.PrReviewersSet != nil && *req.Msg.PrReviewersSet {
		t.PrReviewers = req.Msg.PrReviewers
		t.PrReviewersSet = true
	}
```

**Step 2: Add git import if not present**

```go
import (
	// ... existing imports
	"github.com/randalmurphal/orc/internal/git"
)
```

**Step 3: Run tests**

```bash
go test ./internal/api/... -v -run TestCreateTask
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/api/task_server.go
git commit -m "feat(api): handle branch control fields in CreateTask"
```

---

## Task 10: Backend - Update API UpdateTask Handler

**Files:**
- Modify: `internal/api/task_server.go:324-384`

**Step 1: Add branch control field handling in UpdateTask**

After existing field updates (around line 371), add:

```go
	// Branch control fields
	if req.Msg.BranchName != nil {
		// Only allow branch name change before execution starts
		if t.Status != orcv1.TaskStatus_TASK_STATUS_CREATED &&
			t.Status != orcv1.TaskStatus_TASK_STATUS_PLANNED {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("cannot change branch name after task execution has started"))
		}
		if *req.Msg.BranchName != "" {
			if err := git.ValidateBranchName(*req.Msg.BranchName); err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument,
					fmt.Errorf("invalid branch name: %w", err))
			}
		}
		t.BranchName = req.Msg.BranchName
	}
	if req.Msg.PrDraft != nil {
		t.PrDraft = req.Msg.PrDraft
	}
	if req.Msg.PrLabelsSet != nil {
		if *req.Msg.PrLabelsSet {
			t.PrLabels = req.Msg.PrLabels
			t.PrLabelsSet = true
		} else {
			t.PrLabels = nil
			t.PrLabelsSet = false
		}
	}
	if req.Msg.PrReviewersSet != nil {
		if *req.Msg.PrReviewersSet {
			t.PrReviewers = req.Msg.PrReviewers
			t.PrReviewersSet = true
		} else {
			t.PrReviewers = nil
			t.PrReviewersSet = false
		}
	}
```

**Step 2: Run tests**

```bash
go test ./internal/api/... -v -run TestUpdateTask
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/api/task_server.go
git commit -m "feat(api): handle branch control fields in UpdateTask"
```

---

## Task 11: CLI - Add Branch Control Flags to cmd_new.go

**Files:**
- Modify: `internal/cli/cmd_new.go`

**Step 1: Add flag definitions**

Find the flag definitions section and add:

```go
	cmd.Flags().String("branch", "", "Custom branch name (default: auto-generated from task ID)")
	cmd.Flags().Bool("draft", false, "Create PR as draft")
	cmd.Flags().StringSlice("reviewers", nil, "PR reviewers (comma-separated)")
	cmd.Flags().StringSlice("labels", nil, "PR labels (comma-separated)")
```

**Step 2: Add flag parsing in RunE**

After existing flag parsing (around line 207), add:

```go
	branchName, _ := cmd.Flags().GetString("branch")
	prDraft, _ := cmd.Flags().GetBool("draft")
	prReviewers, _ := cmd.Flags().GetStringSlice("reviewers")
	prLabels, _ := cmd.Flags().GetStringSlice("labels")
	prDraftSet := cmd.Flags().Changed("draft")
	prReviewersSet := cmd.Flags().Changed("reviewers")
	prLabelsSet := cmd.Flags().Changed("labels")
```

**Step 3: Add branch name validation**

After target branch validation (around line 214):

```go
	// Validate custom branch name if specified
	if branchName != "" {
		if err := git.ValidateBranchName(branchName); err != nil {
			return fmt.Errorf("invalid branch name: %w", err)
		}
	}
```

**Step 4: Apply fields to task**

After task creation (around line 270), add:

```go
	// Branch control fields
	if branchName != "" {
		t.BranchName = &branchName
	}
	if prDraftSet {
		t.PrDraft = &prDraft
	}
	if prLabelsSet {
		t.PrLabels = prLabels
		t.PrLabelsSet = true
	}
	if prReviewersSet {
		t.PrReviewers = prReviewers
		t.PrReviewersSet = true
	}
```

**Step 5: Run tests**

```bash
go test ./internal/cli/... -v -run TestNew
```

Expected: PASS

**Step 6: Commit**

```bash
git add internal/cli/cmd_new.go
git commit -m "feat(cli): add --branch, --draft, --reviewers, --labels flags to orc new"
```

---

## Task 12: CLI - Add Branch Control Flags to cmd_go.go

**Files:**
- Modify: `internal/cli/cmd_go.go`

**Step 1: Add same flag definitions**

```go
	cmd.Flags().String("branch", "", "Custom branch name (default: auto-generated from task ID)")
	cmd.Flags().Bool("draft", false, "Create PR as draft")
	cmd.Flags().StringSlice("reviewers", nil, "PR reviewers (comma-separated)")
	cmd.Flags().StringSlice("labels", nil, "PR labels (comma-separated)")
```

**Step 2: Add flag parsing and task field assignment**

Follow same pattern as cmd_new.go.

**Step 3: Run tests**

```bash
go test ./internal/cli/... -v -run TestGo
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/cli/cmd_go.go
git commit -m "feat(cli): add branch control flags to orc go"
```

---

## Task 13: Frontend - Update TaskEditModal with Branch Fields

**Files:**
- Modify: `web/src/components/task-detail/TaskEditModal.tsx`

**Step 1: Add state for new fields**

After existing state declarations (around line 100):

```tsx
const [branchName, setBranchName] = useState(task.branchName ?? '');
const [prDraft, setPrDraft] = useState<boolean | undefined>(task.prDraft);
const [prLabels, setPrLabels] = useState<string[]>(task.prLabels ?? []);
const [prReviewers, setPrReviewers] = useState<string[]>(task.prReviewers ?? []);
const [prLabelsSet, setPrLabelsSet] = useState(task.prLabelsSet ?? false);
const [prReviewersSet, setPrReviewersSet] = useState(task.prReviewersSet ?? false);
```

**Step 2: Update reset effect**

In the useEffect that resets form (around line 134-146), add:

```tsx
setBranchName(task.branchName ?? '');
setPrDraft(task.prDraft);
setPrLabels(task.prLabels ?? []);
setPrReviewers(task.prReviewers ?? []);
setPrLabelsSet(task.prLabelsSet ?? false);
setPrReviewersSet(task.prReviewersSet ?? false);
```

**Step 3: Add Git section to form JSX**

After existing form fields, before the footer:

```tsx
{/* Git Section */}
<div className="task-edit-section">
  <h3 className="task-edit-section__title">Git</h3>

  <div className="task-edit-field">
    <label htmlFor="branchName">Branch Name</label>
    <input
      id="branchName"
      type="text"
      value={branchName}
      onChange={(e) => setBranchName(e.target.value)}
      placeholder="auto-generated from task ID"
      disabled={task.status !== TaskStatus.CREATED && task.status !== TaskStatus.PLANNED}
    />
    {task.status !== TaskStatus.CREATED && task.status !== TaskStatus.PLANNED && (
      <span className="task-edit-field__hint">Cannot change after execution starts</span>
    )}
  </div>

  <div className="task-edit-field">
    <label htmlFor="targetBranch">Target Branch</label>
    <input
      id="targetBranch"
      type="text"
      value={targetBranch}
      onChange={(e) => setTargetBranch(e.target.value)}
      placeholder="main"
    />
  </div>
</div>

{/* PR Settings Section (collapsible) */}
<details className="task-edit-section task-edit-section--collapsible">
  <summary className="task-edit-section__title">PR Settings</summary>

  <div className="task-edit-field task-edit-field--checkbox">
    <label>
      <input
        type="checkbox"
        checked={prDraft ?? false}
        onChange={(e) => setPrDraft(e.target.checked)}
      />
      Create as Draft
    </label>
  </div>

  <div className="task-edit-field">
    <label htmlFor="prLabels">Labels (comma-separated)</label>
    <input
      id="prLabels"
      type="text"
      value={prLabels.join(', ')}
      onChange={(e) => {
        const labels = e.target.value.split(',').map(l => l.trim()).filter(Boolean);
        setPrLabels(labels);
        setPrLabelsSet(true);
      }}
      placeholder="Use project default"
    />
  </div>

  <div className="task-edit-field">
    <label htmlFor="prReviewers">Reviewers (comma-separated)</label>
    <input
      id="prReviewers"
      type="text"
      value={prReviewers.join(', ')}
      onChange={(e) => {
        const reviewers = e.target.value.split(',').map(r => r.trim()).filter(Boolean);
        setPrReviewers(reviewers);
        setPrReviewersSet(true);
      }}
      placeholder="Use project default"
    />
  </div>
</details>
```

**Step 4: Update save handler**

In the handleSave function, add the new fields to the update request:

```tsx
const response = await taskClient.updateTask({
  taskId: task.id,
  // ... existing fields ...
  branchName: branchName || undefined,
  prDraft: prDraft,
  prLabels: prLabels,
  prReviewers: prReviewers,
  prLabelsSet: prLabelsSet,
  prReviewersSet: prReviewersSet,
});
```

**Step 5: Add CSS**

Add to `TaskEditModal.css`:

```css
.task-edit-section--collapsible {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: var(--space-3);
}

.task-edit-section--collapsible summary {
  cursor: pointer;
  user-select: none;
}

.task-edit-field__hint {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  margin-top: var(--space-1);
}

.task-edit-field--checkbox label {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
```

**Step 6: Run tests**

```bash
cd web && bun run test -- TaskEditModal
```

Expected: PASS

**Step 7: Commit**

```bash
git add web/src/components/task-detail/TaskEditModal.tsx web/src/components/task-detail/TaskEditModal.css
git commit -m "feat(web): add branch control fields to TaskEditModal"
```

---

## Task 14: Frontend - Create GitSettings Page

**Files:**
- Create: `web/src/pages/settings/GitSettings.tsx`
- Create: `web/src/pages/settings/GitSettings.css`

**Step 1: Create GitSettings component**

```tsx
// web/src/pages/settings/GitSettings.tsx
import { useState, useEffect } from 'react';
import { configClient } from '@/lib/client';
import { Button } from '@/components/ui/Button';
import { toast } from '@/stores/uiStore';
import type { Config } from '@/gen/orc/v1/config_pb';
import './GitSettings.css';

export function GitSettings() {
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  // Form state
  const [targetBranch, setTargetBranch] = useState('main');
  const [branchPrefix, setBranchPrefix] = useState('orc/');
  const [prDraft, setPrDraft] = useState(false);
  const [prLabels, setPrLabels] = useState<string[]>([]);
  const [prReviewers, setPrReviewers] = useState<string[]>([]);

  useEffect(() => {
    const loadConfig = async () => {
      try {
        const response = await configClient.getConfig({});
        setConfig(response.config ?? null);
        if (response.config) {
          setTargetBranch(response.config.completion?.targetBranch ?? 'main');
          setBranchPrefix(response.config.git?.branchPrefix ?? 'orc/');
          setPrDraft(response.config.completion?.pr?.draft ?? false);
          setPrLabels(response.config.completion?.pr?.labels ?? []);
          setPrReviewers(response.config.completion?.pr?.reviewers ?? []);
        }
      } catch (e) {
        console.error('Failed to load config:', e);
        toast({ type: 'error', message: 'Failed to load configuration' });
      } finally {
        setLoading(false);
      }
    };
    loadConfig();
  }, []);

  const handleSave = async () => {
    setSaving(true);
    try {
      await configClient.updateConfig({
        completion: {
          targetBranch,
          pr: {
            draft: prDraft,
            labels: prLabels,
            reviewers: prReviewers,
          },
        },
        git: {
          branchPrefix,
        },
      });
      toast({ type: 'success', message: 'Settings saved' });
    } catch (e) {
      console.error('Failed to save config:', e);
      toast({ type: 'error', message: 'Failed to save settings' });
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <div className="git-settings__loading">Loading...</div>;
  }

  return (
    <div className="git-settings">
      <header className="git-settings__header">
        <h1>Git & Pull Requests</h1>
        <p>Configure default git branch and PR settings for tasks.</p>
      </header>

      <section className="git-settings__section">
        <h2>Branches</h2>

        <div className="git-settings__field">
          <label htmlFor="targetBranch">Default Target Branch</label>
          <input
            id="targetBranch"
            type="text"
            value={targetBranch}
            onChange={(e) => setTargetBranch(e.target.value)}
            placeholder="main"
          />
          <span className="git-settings__hint">
            Branch where PRs will be merged. Can be overridden per task.
          </span>
        </div>

        <div className="git-settings__field">
          <label htmlFor="branchPrefix">Branch Prefix</label>
          <input
            id="branchPrefix"
            type="text"
            value={branchPrefix}
            onChange={(e) => setBranchPrefix(e.target.value)}
            placeholder="orc/"
          />
          <span className="git-settings__hint">
            Prefix for auto-generated branch names (e.g., "orc/TASK-001").
          </span>
        </div>
      </section>

      <section className="git-settings__section">
        <h2>Pull Request Defaults</h2>

        <div className="git-settings__field git-settings__field--checkbox">
          <label>
            <input
              type="checkbox"
              checked={prDraft}
              onChange={(e) => setPrDraft(e.target.checked)}
            />
            Create as Draft
          </label>
        </div>

        <div className="git-settings__field">
          <label htmlFor="prLabels">Default Labels</label>
          <input
            id="prLabels"
            type="text"
            value={prLabels.join(', ')}
            onChange={(e) => setPrLabels(e.target.value.split(',').map(l => l.trim()).filter(Boolean))}
            placeholder="automated, orc"
          />
        </div>

        <div className="git-settings__field">
          <label htmlFor="prReviewers">Default Reviewers</label>
          <input
            id="prReviewers"
            type="text"
            value={prReviewers.join(', ')}
            onChange={(e) => setPrReviewers(e.target.value.split(',').map(r => r.trim()).filter(Boolean))}
            placeholder="alice, bob"
          />
        </div>
      </section>

      <footer className="git-settings__footer">
        <Button onClick={handleSave} disabled={saving}>
          {saving ? 'Saving...' : 'Save Changes'}
        </Button>
      </footer>
    </div>
  );
}
```

**Step 2: Create CSS**

```css
/* web/src/pages/settings/GitSettings.css */
.git-settings {
  max-width: 600px;
  padding: var(--space-6);
}

.git-settings__header {
  margin-bottom: var(--space-6);
}

.git-settings__header h1 {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  margin-bottom: var(--space-2);
}

.git-settings__header p {
  color: var(--color-text-muted);
}

.git-settings__section {
  margin-bottom: var(--space-6);
  padding-bottom: var(--space-6);
  border-bottom: 1px solid var(--color-border);
}

.git-settings__section h2 {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-medium);
  margin-bottom: var(--space-4);
}

.git-settings__field {
  margin-bottom: var(--space-4);
}

.git-settings__field label {
  display: block;
  font-weight: var(--font-weight-medium);
  margin-bottom: var(--space-1);
}

.git-settings__field input[type="text"] {
  width: 100%;
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  color: var(--color-text);
}

.git-settings__field--checkbox label {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-weight: normal;
}

.git-settings__hint {
  display: block;
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
  margin-top: var(--space-1);
}

.git-settings__footer {
  display: flex;
  justify-content: flex-end;
}

.git-settings__loading {
  padding: var(--space-6);
  text-align: center;
  color: var(--color-text-muted);
}
```

**Step 3: Commit**

```bash
git add web/src/pages/settings/GitSettings.tsx web/src/pages/settings/GitSettings.css
git commit -m "feat(web): create GitSettings page"
```

---

## Task 15: Frontend - Add Git Settings to SettingsLayout

**Files:**
- Modify: `web/src/components/settings/SettingsLayout.tsx`
- Modify: `web/src/App.tsx` (or routes file)

**Step 1: Add nav item to SettingsLayout**

In the ORC NavGroup (around line 111-116), add:

```tsx
<NavItem to="/settings/git" icon="git-branch" label="Git & PRs" />
```

**Step 2: Add route to App.tsx**

Find the settings routes and add:

```tsx
import { GitSettings } from '@/pages/settings/GitSettings';

// In routes:
<Route path="git" element={<GitSettings />} />
```

**Step 3: Run dev server and verify**

```bash
cd web && bun run dev
```

Navigate to `/settings/git` and verify the page renders.

**Step 4: Commit**

```bash
git add web/src/components/settings/SettingsLayout.tsx web/src/App.tsx
git commit -m "feat(web): add Git settings to settings nav"
```

---

## Task 16: Integration Test - CLI Branch Flags

**Files:**
- Modify: `internal/cli/cmd_new_test.go` (or create)

**Step 1: Write integration test**

```go
func TestNewCmd_BranchFlags(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize orc
	err := config.InitAt(tmpDir, false)
	require.NoError(t, err)

	// Create task with branch flags
	cmd := newNewCmd()
	cmd.SetArgs([]string{
		"Test task",
		"--branch", "feature/custom-123",
		"--draft",
		"--labels", "bug,urgent",
		"--reviewers", "alice,bob",
	})

	err = cmd.Execute()
	require.NoError(t, err)

	// Load and verify task
	backend, err := storage.NewDatabaseBackend(tmpDir)
	require.NoError(t, err)
	defer backend.Close()

	tasks, err := backend.LoadAllTasks()
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	task := tasks[0]
	assert.Equal(t, "feature/custom-123", *task.BranchName)
	assert.True(t, *task.PrDraft)
	assert.Equal(t, []string{"bug", "urgent"}, task.PrLabels)
	assert.Equal(t, []string{"alice", "bob"}, task.PrReviewers)
}
```

**Step 2: Run test**

```bash
go test ./internal/cli/... -v -run TestNewCmd_BranchFlags
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/cli/cmd_new_test.go
git commit -m "test(cli): add integration test for branch control flags"
```

---

## Task 17: Final - Run Full Test Suite

**Step 1: Run backend tests**

```bash
make test
```

Expected: All PASS

**Step 2: Run frontend tests**

```bash
cd web && bun run test
```

Expected: All PASS

**Step 3: Run build**

```bash
make build
```

Expected: Success

**Step 4: Final commit (if any remaining changes)**

```bash
git status
# If clean, done. If not, commit remaining changes.
```

---

## Summary

| Task | Component | Description |
|------|-----------|-------------|
| 1-4 | Proto | Add fields to Task, CreateTaskRequest, UpdateTaskRequest, regenerate |
| 5 | Backend | Proto helper functions |
| 6-8 | Backend | Branch name resolution, PR options resolution, worktree setup |
| 9-10 | Backend | API handlers (Create/Update) |
| 11-12 | CLI | Flags for `orc new`, `orc go` |
| 13 | Frontend | TaskEditModal updates |
| 14-15 | Frontend | GitSettings page + nav |
| 16 | Test | Integration test |
| 17 | Final | Full test suite |
