// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/detect"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/template"
	"github.com/randalmurphal/orc/internal/workflow"
)

// newNewCmd creates the new task command
func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new task to be orchestrated by orc",
		Long: `Create a task that will be executed by Claude through orc's phased workflow.

═══════════════════════════════════════════════════════════════════════════════
CRITICAL: WHAT MAKES TASKS SUCCEED
═══════════════════════════════════════════════════════════════════════════════

For non-trivial tasks, orc REQUIRES a specification with three mandatory sections:

  1. INTENT        - What problem are you solving? Why does this matter?
  2. SUCCESS CRITERIA - Testable conditions that prove the work is done
  3. TESTING       - How to verify the implementation works

Without these, the task WILL fail or produce incomplete results. The spec phase
generates this, but YOU provide the foundation through title + description.

GOOD task creation (leads to good spec → good implementation):
  orc new "Add rate limiting to API endpoints" -d "Prevent abuse by limiting
  requests to 100/min per user. Should return 429 when exceeded. Must not
  affect authenticated admin users."

BAD task creation (vague → vague spec → poor implementation):
  orc new "Fix API"

═══════════════════════════════════════════════════════════════════════════════
WEIGHT SELECTION (Determines phases & quality gates)
═══════════════════════════════════════════════════════════════════════════════

Weight determines which phases run. Choose based on COMPLEXITY, not time:

  trivial    One-liner fixes, typos, config tweaks
             → tiny_spec → implement
             Example: "Fix typo in error message"

  small      Bug fixes, small features, isolated changes
             → tiny_spec → implement → review
             Example: "Add validation for email field"

  medium     Features requiring design thought (DEFAULT)
             → spec → tdd_write → implement → review → docs
             Example: "Add password reset flow"

  large      Complex features, multi-file changes, new systems
             → spec → tdd_write → breakdown → implement → review → docs
             Example: "Implement caching layer for API"

Key phases:
  • spec/tiny_spec  Creates Success Criteria + Testing requirements (REQUIRED for quality)
  • tdd_write       Writes failing tests BEFORE implementation (context isolation)
  • breakdown       Decomposes large tasks into checkboxed steps
  • review          Multi-agent code review with specialized reviewers (includes verification)

Use 'orc finalize TASK-XXX' to manually sync with target branch before merge.

⚠️  COMMON MISTAKE: Under-weighting tasks. If unsure, go ONE weight heavier.
    A "medium" task run as "small" skips the spec phase → Claude guesses
    requirements → implementation misses the mark.

═══════════════════════════════════════════════════════════════════════════════
WORKFLOW OVERRIDE (--workflow)
═══════════════════════════════════════════════════════════════════════════════

Use --workflow to assign a workflow (required for task execution):

  orc new "Verify auth flow works" --workflow qa-e2e
  orc new "Review the refactor" --workflow review

The workflow is stored with the task and used when you run 'orc run TASK-XXX'.
List available workflows: orc workflows

═══════════════════════════════════════════════════════════════════════════════
THE DESCRIPTION FIELD (-d) IS YOUR LEVERAGE
═══════════════════════════════════════════════════════════════════════════════

The description flows into EVERY phase prompt. It's how you communicate:
  • What problem exists (the pain point)
  • What success looks like (acceptance criteria hints)
  • Constraints or requirements (performance, compatibility, etc.)
  • Context Claude needs (related systems, edge cases)

Example of description that produces excellent results:
  orc new "Add user avatar upload" -w medium -d "Users should be able to
  upload a profile picture. Requirements: Accept PNG/JPG up to 5MB, resize
  to 200x200, store in S3, display in navbar. Must work on mobile. Related
  to existing User model in models/user.go."

═══════════════════════════════════════════════════════════════════════════════
INITIATIVES: SHARED CONTEXT ACROSS TASKS
═══════════════════════════════════════════════════════════════════════════════

When tasks are part of a larger feature, link them to an initiative:

  orc initiative new "User Authentication" -V "JWT-based auth with refresh tokens"
  orc initiative decide INIT-001 "Use bcrypt for password hashing"
  orc new "Create login endpoint" -i INIT-001 -w medium
  orc new "Create logout endpoint" -i INIT-001 -w small --blocked-by TASK-001

The initiative's VISION and DECISIONS flow into every linked task's prompts.
This keeps Claude aligned across multiple related tasks.

═══════════════════════════════════════════════════════════════════════════════
CATEGORY SELECTION
═══════════════════════════════════════════════════════════════════════════════

Category affects how Claude approaches the work:

  feature    New functionality (default) - focus on user value
  bug        Broken behavior - focus on root cause & regression prevention
  refactor   Code improvement - focus on preserving behavior while improving
  chore      Maintenance - focus on operational concerns
  docs       Documentation - focus on clarity and accuracy
  test       Test coverage - focus on edge cases and assertions

═══════════════════════════════════════════════════════════════════════════════
DEPENDENCIES: ORDERING WORK
═══════════════════════════════════════════════════════════════════════════════

  --blocked-by TASK-XXX   Hard dependency - task won't run until blocker completes
  --related-to TASK-XXX   Informational link - no execution blocking

Example multi-task workflow:
  orc new "Design database schema" -w medium
  orc new "Implement data models" -w medium --blocked-by TASK-001
  orc new "Create API endpoints" -w large --blocked-by TASK-002
  orc new "Build frontend" -w large --blocked-by TASK-003

═══════════════════════════════════════════════════════════════════════════════
EXAMPLES
═══════════════════════════════════════════════════════════════════════════════

# Good: Clear title, appropriate weight, detailed description
orc new "Add pagination to user list API" -w medium -c feature \
  -d "The /api/users endpoint returns all users. Add limit/offset pagination
  with default limit=20, max=100. Return total count in response header."

# Good: Bug with context about the problem
orc new "Fix login failing silently on timeout" -w small -c bug \
  -d "When auth service times out, login form shows no error. User sees
  nothing. Should show 'Service unavailable, try again' message."

# Good: Part of initiative with dependency
orc new "Implement refresh token rotation" -w medium -i INIT-001 \
  --blocked-by TASK-005 \
  -d "After login endpoint is done, add refresh token rotation per RFC 6749."

# Trivial: Simple fix, no spec needed
orc new "Fix typo: 'recieve' → 'receive'" -w trivial

See also:
  orc run      - Execute a task (uses assigned workflow_id)
  orc show     - View task details and spec content
  orc deps     - View task dependencies
  orc initiative - Group related tasks with shared context`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			title := args[0]
			weight, _ := cmd.Flags().GetString("weight")
			workflowID, _ := cmd.Flags().GetString("workflow")
			category, _ := cmd.Flags().GetString("category")
			priority, _ := cmd.Flags().GetString("priority")
			description, _ := cmd.Flags().GetString("description")
			templateName, _ := cmd.Flags().GetString("template")
			varsFlag, _ := cmd.Flags().GetStringSlice("var")
			attachments, _ := cmd.Flags().GetStringSlice("attach")
			initiativeID, _ := cmd.Flags().GetString("initiative")
			blockedBy, _ := cmd.Flags().GetStringSlice("blocked-by")
			relatedTo, _ := cmd.Flags().GetStringSlice("related-to")
			targetBranch, _ := cmd.Flags().GetString("target-branch")
			beforeImages, _ := cmd.Flags().GetStringSlice("before-images")
			qaMaxIterations, _ := cmd.Flags().GetInt("qa-max-iterations")
			gateOverrides, _ := cmd.Flags().GetStringSlice("gate")
			specContent, _ := cmd.Flags().GetString("spec-content")
			// Branch control flags
			branchName, _ := cmd.Flags().GetString("branch")
			prDraft, _ := cmd.Flags().GetBool("pr-draft")
			prDraftSet := cmd.Flags().Changed("pr-draft")
			prLabels, _ := cmd.Flags().GetStringSlice("pr-labels")
			prLabelsSet := cmd.Flags().Changed("pr-labels")
			prReviewers, _ := cmd.Flags().GetStringSlice("pr-reviewers")
			prReviewersSet := cmd.Flags().Changed("pr-reviewers")

			// Validate target branch if specified
			if targetBranch != "" {
				if err := git.ValidateBranchName(targetBranch); err != nil {
					return fmt.Errorf("invalid target branch: %w", err)
				}
			}

			// Validate custom branch name if specified
			if branchName != "" {
				if err := git.ValidateBranchName(branchName); err != nil {
					return fmt.Errorf("invalid branch name: %w", err)
				}
			}

			// Validate workflow if specified
			var pdb *db.ProjectDB
			if workflowID != "" {
				// Need project DB for gate overrides later
				projectRoot, rootErr := ResolveProjectPath()
				if rootErr != nil {
					return rootErr
				}
				pdb, err = db.OpenProject(projectRoot)
				if err != nil {
					return fmt.Errorf("open project database: %w", err)
				}
				defer func() { _ = pdb.Close() }()

				// Open global DB for workflows
				gdb, err := db.OpenGlobal()
				if err != nil {
					return fmt.Errorf("open global database: %w", err)
				}
				defer func() { _ = gdb.Close() }()

				// Seed built-in workflows to ensure they exist (into global DB)
				if _, err := workflow.SeedBuiltins(gdb); err != nil {
					return fmt.Errorf("seed workflows: %w", err)
				}

				// Verify workflow exists (in global DB)
				wf, wfErr := gdb.GetWorkflow(workflowID)
				if wfErr != nil {
					return fmt.Errorf("get workflow: %w", wfErr)
				}
				if wf == nil {
					return fmt.Errorf("workflow not found: %s\n\nRun 'orc workflows' to see available workflows", workflowID)
				}
			}

			// Parse variable flags
			vars := make(map[string]string)
			for _, v := range varsFlag {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) == 2 {
					vars[parts[0]] = parts[1]
				}
			}

			// Generate next task ID
			id, err := backend.GetNextTaskID()
			if err != nil {
				return fmt.Errorf("generate task ID: %w", err)
			}

			// Create task
			t := task.NewProtoTask(id, title)
			if description != "" {
				t.Description = &description
			}

			// If using template, get weight and phases from template
			var tpl *template.Template
			if templateName != "" {
				tpl, err = template.Load(templateName)
				if err != nil {
					return fmt.Errorf("template %q not found", templateName)
				}

				// Validate required variables
				if err := tpl.ValidateVariables(vars); err != nil {
					return err
				}

				// Use template weight unless overridden
				if weight == "" {
					weight = tpl.Weight
				}

				// Render title and description with variables
				vars["TASK_TITLE"] = title
				currentDesc := ""
				if t.Description != nil {
					currentDesc = *t.Description
				}
				renderedDesc := template.Render(currentDesc, vars)
				t.Description = &renderedDesc

				if !quiet {
					fmt.Printf("Using template: %s\n", tpl.Name)
				}
			}

			// Set weight (defaults to medium if not specified via --weight flag)
			if weight != "" {
				w, valid := task.ParseWeightProto(weight)
				if !valid {
					return fmt.Errorf("invalid weight: %s (valid: trivial, small, medium, large)", weight)
				}
				t.Weight = w
			} else {
				t.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
			}

			// Set category (defaults to feature if not specified)
			if category != "" {
				cat, valid := task.ParseCategoryProto(category)
				if !valid {
					return fmt.Errorf("invalid category: %s (valid: feature, bug, refactor, chore, docs, test)", category)
				}
				t.Category = cat
			}

			// Set priority (defaults to normal if not specified)
			if priority != "" {
				pri, valid := task.ParsePriorityProto(priority)
				if !valid {
					return fmt.Errorf("invalid priority: %s (valid: critical, high, normal, low)", priority)
				}
				t.Priority = pri
			}

			// Auto-assign workflow based on weight if not explicitly provided
			if workflowID == "" {
				workflowID = workflow.WeightToWorkflowID(t.Weight)
			}

			// Set workflow if we have one (either explicit or from weight)
			if workflowID != "" {
				t.WorkflowId = &workflowID
			}

			// Link to initiative if specified
			if initiativeID != "" {
				// Verify initiative exists
				exists, err := backend.InitiativeExists(initiativeID)
				if err != nil {
					return fmt.Errorf("check initiative: %w", err)
				}
				if !exists {
					return fmt.Errorf("initiative %s not found", initiativeID)
				}
				task.SetInitiativeProto(t, initiativeID)
			}

			// Set target branch if provided
			if targetBranch != "" {
				task.SetTargetBranchProto(t, targetBranch)
			}

			// Set branch control overrides
			if branchName != "" {
				task.SetBranchNameProto(t, branchName)
			}
			if prDraftSet {
				task.SetPRDraftProto(t, prDraft)
			}
			if prLabelsSet {
				task.SetPRLabelsProto(t, prLabels)
			}
			if prReviewersSet {
				task.SetPRReviewersProto(t, prReviewers)
			}

			// Set QA-specific task metadata
			if len(beforeImages) > 0 || qaMaxIterations > 0 {
				if t.Metadata == nil {
					t.Metadata = make(map[string]string)
				}
				if len(beforeImages) > 0 {
					// Join image paths with newlines for BEFORE_IMAGES variable
					t.Metadata["before_images"] = strings.Join(beforeImages, "\n")
				}
				if qaMaxIterations > 0 {
					t.Metadata["qa_max_iterations"] = fmt.Sprintf("%d", qaMaxIterations)
				}
			}

			// Detect project characteristics for testing requirements
			// This is a fast operation (<10ms) so we run it on every task creation
			detection, _ := detect.Detect(".")
			hasFrontend := detection != nil && detection.HasFrontend

			// Set testing requirements based on project and task content
			task.SetTestingRequirementsProto(t, hasFrontend)

			// Set dependencies if provided
			if len(blockedBy) > 0 || len(relatedTo) > 0 {
				// Load existing tasks for validation
				existingTasks, err := backend.LoadAllTasks()
				if err != nil {
					return fmt.Errorf("load existing tasks: %w", err)
				}
				existingIDs := make(map[string]bool)
				for _, existing := range existingTasks {
					existingIDs[existing.Id] = true
				}

				// Validate blocked_by references
				if errs := task.ValidateBlockedBy(id, blockedBy, existingIDs); len(errs) > 0 {
					return errs[0]
				}

				// Validate related_to references
				if errs := task.ValidateRelatedTo(id, relatedTo, existingIDs); len(errs) > 0 {
					return errs[0]
				}

				t.BlockedBy = blockedBy
				t.RelatedTo = relatedTo
			}

			// Save task with planned status
			// Plans are created dynamically at runtime based on task weight
			t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
			if err := backend.SaveTask(t); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			// Save pre-populated spec if provided (enables spec phase auto-skip)
			if specContent != "" {
				if err := backend.SaveSpecForTask(id, specContent, "brainstorm"); err != nil {
					return fmt.Errorf("save spec content: %w", err)
				}
			}

			// Save gate overrides if provided
			if len(gateOverrides) > 0 {
				// Need project DB if not already opened
				if pdb == nil {
					projectRoot, rootErr := ResolveProjectPath()
					if rootErr != nil {
						return fmt.Errorf("find project root for gate overrides: %w", rootErr)
					}
					pdb, err = db.OpenProject(projectRoot)
					if err != nil {
						return fmt.Errorf("open project database for gate overrides: %w", err)
					}
					defer func() { _ = pdb.Close() }()
				}

				for _, gateSpec := range gateOverrides {
					parts := strings.SplitN(gateSpec, ":", 2)
					if len(parts) != 2 {
						return fmt.Errorf("invalid gate format: %q (expected phase:type, e.g., spec:human)", gateSpec)
					}
					phaseID, gateType := parts[0], parts[1]

					// Validate gate type
					validGateTypes := map[string]bool{
						"auto": true, "human": true, "ai": true, "skip": true,
					}
					if !validGateTypes[gateType] {
						return fmt.Errorf("invalid gate type: %q (valid: auto, human, ai, skip)", gateType)
					}

					override := &db.TaskGateOverride{
						TaskID:   id,
						PhaseID:  phaseID,
						GateType: gateType,
					}
					if err := pdb.SaveTaskGateOverride(override); err != nil {
						return fmt.Errorf("save gate override %s: %w", gateSpec, err)
					}
				}
			}

			// Sync task to initiative if linked
			if task.HasInitiativeProto(t) {
				initID := task.GetInitiativeIDProto(t)
				init, err := backend.LoadInitiative(initID)
				if err != nil {
					// Log warning but don't fail task creation
					fmt.Printf("Warning: failed to load initiative %s for sync: %v\n", initID, err)
				} else {
					init.AddTask(t.Id, t.Title, nil)
					if err := backend.SaveInitiative(init); err != nil {
						fmt.Printf("Warning: failed to sync task to initiative: %v\n", err)
					}
				}
			}

			fmt.Printf("Task created: %s\n", id)
			fmt.Printf("   Title:    %s\n", title)
			fmt.Printf("   Weight:   %s\n", task.WeightFromProto(t.Weight))
			fmt.Printf("   Category: %s\n", task.CategoryFromProto(t.Category))
			fmt.Printf("   Priority: %s\n", task.PriorityFromProto(t.Priority))
			if t.WorkflowId != nil && *t.WorkflowId != "" {
				fmt.Printf("   Workflow: %s\n", *t.WorkflowId)
			}
			if tpl != nil {
				fmt.Printf("   Template: %s\n", tpl.Name)
			}
			if task.HasInitiativeProto(t) {
				fmt.Printf("   Initiative: %s\n", task.GetInitiativeIDProto(t))
			}
			if task.GetTargetBranchProto(t) != "" {
				fmt.Printf("   Target Branch: %s\n", task.GetTargetBranchProto(t))
			}
			if task.GetBranchNameProto(t) != "" {
				fmt.Printf("   Branch: %s\n", task.GetBranchNameProto(t))
			}
			if draft := task.GetPRDraftProto(t); draft != nil {
				fmt.Printf("   PR Draft: %v\n", *draft)
			}
			if labels := task.GetPRLabelsProto(t); len(labels) > 0 {
				fmt.Printf("   PR Labels: %s\n", strings.Join(labels, ", "))
			}
			if reviewers := task.GetPRReviewersProto(t); len(reviewers) > 0 {
				fmt.Printf("   PR Reviewers: %s\n", strings.Join(reviewers, ", "))
			}
			if t.RequiresUiTesting {
				fmt.Printf("   UI Testing: required (detected from task description)\n")
			}
			if t.TestingRequirements != nil {
				var reqs []string
				if t.TestingRequirements.Unit {
					reqs = append(reqs, "unit")
				}
				if t.TestingRequirements.E2E {
					reqs = append(reqs, "e2e")
				}
				if t.TestingRequirements.Visual {
					reqs = append(reqs, "visual")
				}
				if len(reqs) > 0 {
					fmt.Printf("   Testing: %s\n", strings.Join(reqs, ", "))
				}
			}
			if len(t.BlockedBy) > 0 {
				fmt.Printf("   Blocked by: %s\n", strings.Join(t.BlockedBy, ", "))
			}
			if len(t.RelatedTo) > 0 {
				fmt.Printf("   Related to: %s\n", strings.Join(t.RelatedTo, ", "))
			}
			if len(beforeImages) > 0 {
				fmt.Printf("   Before Images: %d file(s) for visual comparison\n", len(beforeImages))
			}
			if qaMaxIterations > 0 {
				fmt.Printf("   Max QA Iterations: %d\n", qaMaxIterations)
			}
			if len(gateOverrides) > 0 {
				fmt.Printf("   Gate Overrides: %s\n", strings.Join(gateOverrides, ", "))
			}

			// Upload attachments if provided
			if len(attachments) > 0 {
				var uploadedCount int
				for _, attachPath := range attachments {
					// Resolve relative paths
					if !filepath.IsAbs(attachPath) {
						cwd, err := os.Getwd()
						if err != nil {
							return fmt.Errorf("get working directory: %w", err)
						}
						attachPath = filepath.Join(cwd, attachPath)
					}

					// Read file
					data, err := os.ReadFile(attachPath)
					if err != nil {
						if os.IsNotExist(err) {
							return fmt.Errorf("attachment not found: %s", attachPath)
						}
						return fmt.Errorf("read attachment %s: %w", attachPath, err)
					}

					// Save attachment via backend
					filename := filepath.Base(attachPath)
					contentType := task.DetectContentType(filename)
					_, err = backend.SaveAttachment(id, filename, contentType, data)
					if err != nil {
						return fmt.Errorf("save attachment %s: %w", filename, err)
					}
					uploadedCount++
				}

				if uploadedCount > 0 {
					fmt.Printf("   Attachments: %d file(s) uploaded\n", uploadedCount)
				}
			}

			fmt.Println("\nNext steps:")
			fmt.Printf("  orc run %s    - Execute the task\n", id)
			fmt.Printf("  orc show %s   - View task details\n", id)

			return nil
		},
	}
	cmd.Flags().StringP("weight", "w", "", "task weight (trivial, small, medium, large, greenfield)")
	cmd.Flags().String("workflow", "", "workflow to use for execution (e.g., implement, qa-e2e)")
	cmd.Flags().StringP("category", "c", "", "task category (feature, bug, refactor, chore, docs, test)")
	cmd.Flags().StringP("priority", "p", "", "task priority (critical, high, normal, low)")
	cmd.Flags().StringP("description", "d", "", "task description")
	cmd.Flags().StringP("template", "t", "", "use template (bugfix, feature, refactor, migration, spike)")
	cmd.Flags().StringSlice("var", nil, "template variable (KEY=VALUE)")
	cmd.Flags().StringSliceP("attach", "a", nil, "file(s) to attach (screenshots, logs, etc.)")
	cmd.Flags().StringP("initiative", "i", "", "link task to initiative (e.g., INIT-001)")
	cmd.Flags().StringSlice("blocked-by", nil, "task IDs that must complete before this task")
	cmd.Flags().StringSlice("related-to", nil, "task IDs related to this task")
	cmd.Flags().String("target-branch", "", "override target branch for PR (instead of project default)")
	cmd.Flags().StringSlice("before-images", nil, "baseline images for visual comparison (QA E2E workflow)")
	cmd.Flags().Int("qa-max-iterations", 0, "max QA iterations before stopping (default: 3)")
	cmd.Flags().StringSlice("gate", nil, "gate overrides (phase:type, e.g., spec:human, review:ai)")
	cmd.Flags().String("spec-content", "", "pre-populate spec content (enables spec phase auto-skip)")
	// Branch control flags
	cmd.Flags().String("branch", "", "custom branch name (default: auto-generated from task ID)")
	cmd.Flags().Bool("pr-draft", false, "create PR as draft")
	cmd.Flags().StringSlice("pr-labels", nil, "PR labels to apply")
	cmd.Flags().StringSlice("pr-reviewers", nil, "PR reviewers to request")
	return cmd
}
