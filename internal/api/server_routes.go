// Package api provides the REST API and SSE server for orc.
package api

import "net/http"

// registerRoutes sets up all API routes.
func (s *Server) registerRoutes() {
	// CORS middleware wrapper
	cors := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			h(w, r)
		}
	}

	// Health check
	s.mux.HandleFunc("GET /api/health", cors(s.handleHealth))

	// Session metrics
	s.mux.HandleFunc("GET /api/session", cors(s.handleGetSessionMetrics))

	// Tasks
	s.mux.HandleFunc("GET /api/tasks", cors(s.handleListTasks))
	s.mux.HandleFunc("POST /api/tasks", cors(s.handleCreateTask))
	s.mux.HandleFunc("GET /api/tasks/{id}", cors(s.handleGetTask))
	s.mux.HandleFunc("PATCH /api/tasks/{id}", cors(s.handleUpdateTask))
	s.mux.HandleFunc("DELETE /api/tasks/{id}", cors(s.handleDeleteTask))

	// Task state and plan
	s.mux.HandleFunc("GET /api/tasks/{id}/state", cors(s.handleGetState))
	s.mux.HandleFunc("GET /api/tasks/{id}/plan", cors(s.handleGetPlan))
	s.mux.HandleFunc("GET /api/tasks/{id}/transcripts", cors(s.handleGetTranscripts))
	s.mux.HandleFunc("GET /api/tasks/{id}/session", cors(s.handleGetSession))
	s.mux.HandleFunc("GET /api/tasks/{id}/tokens", cors(s.handleGetTokens))

	// Task attachments
	s.mux.HandleFunc("GET /api/tasks/{id}/attachments", cors(s.handleListAttachments))
	s.mux.HandleFunc("POST /api/tasks/{id}/attachments", cors(s.handleUploadAttachment))
	s.mux.HandleFunc("GET /api/tasks/{id}/attachments/{filename}", cors(s.handleGetAttachment))
	s.mux.HandleFunc("DELETE /api/tasks/{id}/attachments/{filename}", cors(s.handleDeleteAttachment))

	// Task test results (Playwright)
	s.mux.HandleFunc("GET /api/tasks/{id}/test-results", cors(s.handleGetTestResults))
	s.mux.HandleFunc("POST /api/tasks/{id}/test-results", cors(s.handleSaveTestReport))
	s.mux.HandleFunc("POST /api/tasks/{id}/test-results/init", cors(s.handleInitTestResults))
	s.mux.HandleFunc("GET /api/tasks/{id}/test-results/screenshots", cors(s.handleListScreenshots))
	s.mux.HandleFunc("POST /api/tasks/{id}/test-results/screenshots", cors(s.handleUploadScreenshot))
	s.mux.HandleFunc("GET /api/tasks/{id}/test-results/screenshots/{filename}", cors(s.handleGetScreenshot))
	s.mux.HandleFunc("GET /api/tasks/{id}/test-results/report", cors(s.handleGetHTMLReport))
	s.mux.HandleFunc("GET /api/tasks/{id}/test-results/traces/{filename}", cors(s.handleGetTrace))

	// Task dependencies
	s.mux.HandleFunc("GET /api/tasks/{id}/dependencies", cors(s.handleGetDependencies))

	// Task diff (git changes visualization)
	s.mux.HandleFunc("GET /api/tasks/{id}/diff", cors(s.handleGetDiff))
	s.mux.HandleFunc("GET /api/tasks/{id}/diff/stats", cors(s.handleGetDiffStats))
	s.mux.HandleFunc("GET /api/tasks/{id}/diff/file/{path...}", cors(s.handleGetDiffFile))

	// Task control
	s.mux.HandleFunc("POST /api/tasks/{id}/run", cors(s.handleRunTask))
	s.mux.HandleFunc("POST /api/tasks/{id}/pause", cors(s.handlePauseTask))
	s.mux.HandleFunc("POST /api/tasks/{id}/resume", cors(s.handleResumeTask))
	s.mux.HandleFunc("POST /api/tasks/{id}/skip-block", cors(s.handleSkipBlock))

	// Gate decisions (human approval in headless mode)
	s.mux.HandleFunc("GET /api/decisions", cors(s.handleListDecisions))
	s.mux.HandleFunc("POST /api/decisions/{id}", cors(s.handlePostDecision))

	// Task retry (fresh session with context injection)
	s.mux.HandleFunc("POST /api/tasks/{id}/retry", cors(s.handleRetryTask))
	s.mux.HandleFunc("GET /api/tasks/{id}/retry/preview", cors(s.handleGetRetryPreview))
	s.mux.HandleFunc("POST /api/tasks/{id}/retry/feedback", cors(s.handleRetryWithFeedback))

	// Task finalize (sync with target branch, resolve conflicts, run tests)
	s.mux.HandleFunc("POST /api/tasks/{id}/finalize", cors(s.handleFinalizeTask))
	s.mux.HandleFunc("GET /api/tasks/{id}/finalize", cors(s.handleGetFinalizeStatus))

	// Task export (export artifacts to branch or directory)
	s.mux.HandleFunc("POST /api/tasks/{id}/export", cors(s.handleExportTask))

	// Export configuration
	s.mux.HandleFunc("GET /api/config/export", cors(s.handleGetExportConfig))
	s.mux.HandleFunc("PUT /api/config/export", cors(s.handleUpdateExportConfig))

	// Initiatives
	s.mux.HandleFunc("GET /api/initiatives", cors(s.handleListInitiatives))
	s.mux.HandleFunc("POST /api/initiatives", cors(s.handleCreateInitiative))
	s.mux.HandleFunc("GET /api/initiatives/{id}", cors(s.handleGetInitiative))
	s.mux.HandleFunc("PUT /api/initiatives/{id}", cors(s.handleUpdateInitiative))
	s.mux.HandleFunc("DELETE /api/initiatives/{id}", cors(s.handleDeleteInitiative))
	s.mux.HandleFunc("GET /api/initiatives/{id}/tasks", cors(s.handleListInitiativeTasks))
	s.mux.HandleFunc("POST /api/initiatives/{id}/tasks", cors(s.handleAddInitiativeTask))
	s.mux.HandleFunc("DELETE /api/initiatives/{id}/tasks/{taskId}", cors(s.handleRemoveInitiativeTask))
	s.mux.HandleFunc("POST /api/initiatives/{id}/decisions", cors(s.handleAddInitiativeDecision))
	s.mux.HandleFunc("GET /api/initiatives/{id}/ready", cors(s.handleGetReadyTasks))
	s.mux.HandleFunc("GET /api/initiatives/{id}/dependency-graph", cors(s.handleGetInitiativeDependencyGraph))

	// Workflows
	s.mux.HandleFunc("GET /api/workflows", cors(s.handleListWorkflows))
	s.mux.HandleFunc("POST /api/workflows", cors(s.handleCreateWorkflow))
	s.mux.HandleFunc("GET /api/workflows/{id}", cors(s.handleGetWorkflow))
	s.mux.HandleFunc("PUT /api/workflows/{id}", cors(s.handleUpdateWorkflow))
	s.mux.HandleFunc("DELETE /api/workflows/{id}", cors(s.handleDeleteWorkflow))
	s.mux.HandleFunc("POST /api/workflows/{id}/clone", cors(s.handleCloneWorkflow))
	s.mux.HandleFunc("POST /api/workflows/{id}/phases", cors(s.handleAddWorkflowPhase))
	s.mux.HandleFunc("DELETE /api/workflows/{id}/phases/{phaseId}", cors(s.handleRemoveWorkflowPhase))
	s.mux.HandleFunc("PATCH /api/workflows/{id}/phases/{phaseId}", cors(s.handleUpdateWorkflowPhase))
	s.mux.HandleFunc("POST /api/workflows/{id}/variables", cors(s.handleAddWorkflowVariable))
	s.mux.HandleFunc("DELETE /api/workflows/{id}/variables/{name}", cors(s.handleRemoveWorkflowVariable))

	// Phase Templates
	s.mux.HandleFunc("GET /api/phase-templates", cors(s.handleListPhaseTemplates))
	s.mux.HandleFunc("POST /api/phase-templates", cors(s.handleCreatePhaseTemplate))
	s.mux.HandleFunc("GET /api/phase-templates/{id}", cors(s.handleGetPhaseTemplate))
	s.mux.HandleFunc("PUT /api/phase-templates/{id}", cors(s.handleUpdatePhaseTemplate))
	s.mux.HandleFunc("DELETE /api/phase-templates/{id}", cors(s.handleDeletePhaseTemplate))
	s.mux.HandleFunc("GET /api/phase-templates/{id}/prompt", cors(s.handleGetPhaseTemplatePrompt))

	// Workflow Runs
	s.mux.HandleFunc("GET /api/workflow-runs", cors(s.handleListWorkflowRuns))
	s.mux.HandleFunc("POST /api/workflow-runs", cors(s.handleTriggerWorkflowRun))
	s.mux.HandleFunc("GET /api/workflow-runs/{id}", cors(s.handleGetWorkflowRun))
	s.mux.HandleFunc("POST /api/workflow-runs/{id}/cancel", cors(s.handleCancelWorkflowRun))
	s.mux.HandleFunc("GET /api/workflow-runs/{id}/transcript", cors(s.handleGetWorkflowRunTranscript))

	// Task dependency graph (for arbitrary set of tasks)
	s.mux.HandleFunc("GET /api/tasks/dependency-graph", cors(s.handleGetTasksDependencyGraph))

	// Subtasks (proposed sub-tasks queue)
	s.mux.HandleFunc("GET /api/tasks/{taskId}/subtasks", cors(s.handleListSubtasks))
	s.mux.HandleFunc("GET /api/tasks/{taskId}/subtasks/pending", cors(s.handleListPendingSubtasks))
	s.mux.HandleFunc("POST /api/subtasks", cors(s.handleCreateSubtask))
	s.mux.HandleFunc("GET /api/subtasks/{id}", cors(s.handleGetSubtask))
	s.mux.HandleFunc("POST /api/subtasks/{id}/approve", cors(s.handleApproveSubtask))
	s.mux.HandleFunc("POST /api/subtasks/{id}/reject", cors(s.handleRejectSubtask))
	s.mux.HandleFunc("DELETE /api/subtasks/{id}", cors(s.handleDeleteSubtask))

	// Knowledge queue (patterns, gotchas, decisions)
	s.mux.HandleFunc("GET /api/knowledge", cors(s.handleListKnowledge))
	s.mux.HandleFunc("GET /api/knowledge/status", cors(s.handleGetKnowledgeStatus))
	s.mux.HandleFunc("GET /api/knowledge/stale", cors(s.handleListStaleKnowledge))
	s.mux.HandleFunc("POST /api/knowledge", cors(s.handleCreateKnowledge))
	s.mux.HandleFunc("POST /api/knowledge/approve-all", cors(s.handleApproveAllKnowledge))
	s.mux.HandleFunc("GET /api/knowledge/{id}", cors(s.handleGetKnowledge))
	s.mux.HandleFunc("POST /api/knowledge/{id}/approve", cors(s.handleApproveKnowledge))
	s.mux.HandleFunc("POST /api/knowledge/{id}/reject", cors(s.handleRejectKnowledge))
	s.mux.HandleFunc("POST /api/knowledge/{id}/validate", cors(s.handleValidateKnowledge))
	s.mux.HandleFunc("DELETE /api/knowledge/{id}", cors(s.handleDeleteKnowledge))

	// Review comments (code review UI)
	s.mux.HandleFunc("GET /api/tasks/{id}/review/comments", cors(s.handleListReviewComments))
	s.mux.HandleFunc("POST /api/tasks/{id}/review/comments", cors(s.handleCreateReviewComment))
	s.mux.HandleFunc("GET /api/tasks/{id}/review/comments/{commentId}", cors(s.handleGetReviewComment))
	s.mux.HandleFunc("PATCH /api/tasks/{id}/review/comments/{commentId}", cors(s.handleUpdateReviewComment))
	s.mux.HandleFunc("DELETE /api/tasks/{id}/review/comments/{commentId}", cors(s.handleDeleteReviewComment))
	s.mux.HandleFunc("POST /api/tasks/{id}/review/retry", cors(s.handleReviewRetry))
	s.mux.HandleFunc("GET /api/tasks/{id}/review/stats", cors(s.handleGetReviewStats))
	s.mux.HandleFunc("GET /api/tasks/{id}/review/findings", cors(s.handleGetReviewFindings))

	// Task comments (general notes/discussion)
	s.mux.HandleFunc("GET /api/tasks/{id}/comments", cors(s.handleListTaskComments))
	s.mux.HandleFunc("POST /api/tasks/{id}/comments", cors(s.handleCreateTaskComment))
	s.mux.HandleFunc("GET /api/tasks/{id}/comments/stats", cors(s.handleGetTaskCommentStats))
	s.mux.HandleFunc("GET /api/tasks/{id}/comments/{commentId}", cors(s.handleGetTaskComment))
	s.mux.HandleFunc("PATCH /api/tasks/{id}/comments/{commentId}", cors(s.handleUpdateTaskComment))
	s.mux.HandleFunc("DELETE /api/tasks/{id}/comments/{commentId}", cors(s.handleDeleteTaskComment))

	// GitHub PR integration
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr", cors(s.handleCreatePR))
	s.mux.HandleFunc("GET /api/tasks/{id}/github/pr", cors(s.handleGetPR))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/merge", cors(s.handleMergePR))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/comments/sync", cors(s.handleSyncPRComments))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/comments/import", cors(s.handleImportPRComments))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/comments/{commentId}/autofix", cors(s.handleAutoFixComment))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/comments/{commentId}/reply", cors(s.handleReplyToPRComment))
	s.mux.HandleFunc("GET /api/tasks/{id}/github/pr/checks", cors(s.handleListPRChecks))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/refresh", cors(s.handleRefreshPRStatus))

	// WebSocket for real-time updates
	s.mux.Handle("GET /api/ws", s.wsHandler)

	// Cost aggregation
	s.mux.HandleFunc("GET /api/cost/summary", cors(s.handleGetCostSummary))

	// Metrics (JSONL-based analytics)
	s.mux.HandleFunc("GET /api/metrics/summary", cors(s.handleGetMetricsSummary))
	s.mux.HandleFunc("GET /api/metrics/daily", cors(s.handleGetDailyMetrics))
	s.mux.HandleFunc("GET /api/metrics/by-model", cors(s.handleGetMetricsByModel))
	s.mux.HandleFunc("GET /api/tasks/{id}/metrics", cors(s.handleGetTaskMetrics))

	// Todos (progress tracking from JSONL)
	s.mux.HandleFunc("GET /api/tasks/{id}/todos", cors(s.handleGetTaskTodos))
	s.mux.HandleFunc("GET /api/tasks/{id}/todos/history", cors(s.handleGetTaskTodoHistory))

	// Prompts
	s.mux.HandleFunc("GET /api/prompts", cors(s.handleListPrompts))
	s.mux.HandleFunc("GET /api/prompts/variables", cors(s.handleGetPromptVariables))
	s.mux.HandleFunc("GET /api/prompts/{phase}", cors(s.handleGetPrompt))
	s.mux.HandleFunc("GET /api/prompts/{phase}/default", cors(s.handleGetPromptDefault))
	s.mux.HandleFunc("PUT /api/prompts/{phase}", cors(s.handleSavePrompt))
	s.mux.HandleFunc("DELETE /api/prompts/{phase}", cors(s.handleDeletePrompt))

	// Hooks
	s.mux.HandleFunc("GET /api/hooks", cors(s.handleListHooks))
	s.mux.HandleFunc("GET /api/hooks/types", cors(s.handleGetHookTypes))
	s.mux.HandleFunc("POST /api/hooks", cors(s.handleCreateHook))
	s.mux.HandleFunc("GET /api/hooks/{name}", cors(s.handleGetHook))
	s.mux.HandleFunc("PUT /api/hooks/{name}", cors(s.handleUpdateHook))
	s.mux.HandleFunc("DELETE /api/hooks/{name}", cors(s.handleDeleteHook))

	// Skills (SKILL.md format)
	s.mux.HandleFunc("GET /api/skills", cors(s.handleListSkills))
	s.mux.HandleFunc("POST /api/skills", cors(s.handleCreateSkill))
	s.mux.HandleFunc("GET /api/skills/{name}", cors(s.handleGetSkill))
	s.mux.HandleFunc("PUT /api/skills/{name}", cors(s.handleUpdateSkill))
	s.mux.HandleFunc("DELETE /api/skills/{name}", cors(s.handleDeleteSkill))

	// Settings (Claude Code settings.json with inheritance)
	s.mux.HandleFunc("GET /api/settings", cors(s.handleGetSettings))
	s.mux.HandleFunc("GET /api/settings/global", cors(s.handleGetGlobalSettings))
	s.mux.HandleFunc("PUT /api/settings/global", cors(s.handleUpdateGlobalSettings))
	s.mux.HandleFunc("GET /api/settings/project", cors(s.handleGetProjectSettings))
	s.mux.HandleFunc("GET /api/settings/hierarchy", cors(s.handleGetSettingsHierarchy))
	s.mux.HandleFunc("PUT /api/settings", cors(s.handleUpdateSettings))

	// Tools (available Claude Code tools with permissions)
	s.mux.HandleFunc("GET /api/tools", cors(s.handleListTools))
	s.mux.HandleFunc("GET /api/tools/permissions", cors(s.handleGetToolPermissions))
	s.mux.HandleFunc("PUT /api/tools/permissions", cors(s.handleUpdateToolPermissions))

	// Agents (sub-agent definitions)
	s.mux.HandleFunc("GET /api/agents", cors(s.handleListAgents))
	s.mux.HandleFunc("GET /api/agents/stats", cors(s.handleGetAgentStats))
	s.mux.HandleFunc("POST /api/agents", cors(s.handleCreateAgent))
	s.mux.HandleFunc("GET /api/agents/{name}", cors(s.handleGetAgent))
	s.mux.HandleFunc("PUT /api/agents/{name}", cors(s.handleUpdateAgent))
	s.mux.HandleFunc("DELETE /api/agents/{name}", cors(s.handleDeleteAgent))

	// Scripts (project script registry)
	s.mux.HandleFunc("GET /api/scripts", cors(s.handleListScripts))
	s.mux.HandleFunc("POST /api/scripts", cors(s.handleCreateScript))
	s.mux.HandleFunc("POST /api/scripts/discover", cors(s.handleDiscoverScripts))
	s.mux.HandleFunc("GET /api/scripts/{name}", cors(s.handleGetScript))
	s.mux.HandleFunc("PUT /api/scripts/{name}", cors(s.handleUpdateScript))
	s.mux.HandleFunc("DELETE /api/scripts/{name}", cors(s.handleDeleteScript))

	// CLAUDE.md
	s.mux.HandleFunc("GET /api/claudemd", cors(s.handleGetClaudeMD))
	s.mux.HandleFunc("PUT /api/claudemd", cors(s.handleUpdateClaudeMD))
	s.mux.HandleFunc("GET /api/claudemd/hierarchy", cors(s.handleGetClaudeMDHierarchy))

	// Constitution (project principles/invariants)
	s.mux.HandleFunc("GET /api/constitution", cors(s.handleGetConstitution))
	s.mux.HandleFunc("PUT /api/constitution", cors(s.handleUpdateConstitution))
	s.mux.HandleFunc("DELETE /api/constitution", cors(s.handleDeleteConstitution))

	// MCP Servers (.mcp.json)
	s.mux.HandleFunc("GET /api/mcp", cors(s.handleListMCPServers))
	s.mux.HandleFunc("POST /api/mcp", cors(s.handleCreateMCPServer))
	s.mux.HandleFunc("GET /api/mcp/{name}", cors(s.handleGetMCPServer))
	s.mux.HandleFunc("PUT /api/mcp/{name}", cors(s.handleUpdateMCPServer))
	s.mux.HandleFunc("DELETE /api/mcp/{name}", cors(s.handleDeleteMCPServer))

	// Plugins - Local (.claude/plugins/)
	s.mux.HandleFunc("GET /api/plugins", cors(s.handleListPlugins))
	s.mux.HandleFunc("GET /api/plugins/resources", cors(s.handleListPluginResources))
	s.mux.HandleFunc("GET /api/plugins/updates", cors(s.handleCheckPluginUpdates))
	s.mux.HandleFunc("GET /api/plugins/{name}", cors(s.handleGetPlugin))
	s.mux.HandleFunc("GET /api/plugins/{name}/commands", cors(s.handleListPluginCommands))
	s.mux.HandleFunc("POST /api/plugins/{name}/enable", cors(s.handleEnablePlugin))
	s.mux.HandleFunc("POST /api/plugins/{name}/disable", cors(s.handleDisablePlugin))
	s.mux.HandleFunc("POST /api/plugins/{name}/update", cors(s.handleUpdatePlugin))
	s.mux.HandleFunc("DELETE /api/plugins/{name}", cors(s.handleUninstallPlugin))

	// Plugins - Marketplace (separate prefix to avoid route conflicts)
	s.mux.HandleFunc("GET /api/marketplace/plugins", cors(s.handleBrowseMarketplace))
	s.mux.HandleFunc("GET /api/marketplace/plugins/search", cors(s.handleSearchMarketplace))
	s.mux.HandleFunc("GET /api/marketplace/plugins/{name}", cors(s.handleGetMarketplacePlugin))
	s.mux.HandleFunc("POST /api/marketplace/plugins/{name}/install", cors(s.handleInstallPlugin))

	// Config (orc configuration)
	s.mux.HandleFunc("GET /api/config", cors(s.handleGetConfig))
	s.mux.HandleFunc("PUT /api/config", cors(s.handleUpdateConfig))
	s.mux.HandleFunc("GET /api/config/stats", cors(s.handleGetConfigStats))

	// Projects
	s.mux.HandleFunc("GET /api/projects", cors(s.handleListProjects))
	s.mux.HandleFunc("GET /api/projects/default", cors(s.handleGetDefaultProject))
	s.mux.HandleFunc("PUT /api/projects/default", cors(s.handleSetDefaultProject))
	s.mux.HandleFunc("GET /api/projects/{id}", cors(s.handleGetProject))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks", cors(s.handleListProjectTasks))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks", cors(s.handleCreateProjectTask))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks/{taskId}", cors(s.handleGetProjectTask))
	s.mux.HandleFunc("DELETE /api/projects/{id}/tasks/{taskId}", cors(s.handleDeleteProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/run", cors(s.handleRunProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/pause", cors(s.handlePauseProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/resume", cors(s.handleResumeProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/rewind", cors(s.handleRewindProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/escalate", cors(s.handleEscalateProjectTask))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks/{taskId}/state", cors(s.handleGetProjectTaskState))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks/{taskId}/plan", cors(s.handleGetProjectTaskPlan))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks/{taskId}/transcripts", cors(s.handleGetProjectTaskTranscripts))

	// Templates
	s.mux.HandleFunc("GET /api/templates", cors(s.handleListTemplates))
	s.mux.HandleFunc("POST /api/templates", cors(s.handleCreateTemplate))
	s.mux.HandleFunc("GET /api/templates/{name}", cors(s.handleGetTemplate))
	s.mux.HandleFunc("DELETE /api/templates/{name}", cors(s.handleDeleteTemplate))

	// Dashboard
	s.mux.HandleFunc("GET /api/dashboard/stats", cors(s.handleGetDashboardStats))

	// Stats (activity heatmap, outcomes donut, metrics)
	s.mux.HandleFunc("GET /api/stats/activity", cors(s.handleGetActivityStats))
	s.mux.HandleFunc("GET /api/stats/per-day", cors(s.handleGetPerDayStats))
	s.mux.HandleFunc("GET /api/stats/outcomes", cors(s.handleGetOutcomesStats))
	s.mux.HandleFunc("GET /api/stats/top-initiatives", cors(s.handleGetTopInitiatives))
	s.mux.HandleFunc("GET /api/stats/top-files", cors(s.handleGetTopFiles))
	s.mux.HandleFunc("GET /api/stats/comparison", cors(s.handleGetComparisonStats))

	// Events (timeline queries)
	s.mux.HandleFunc("GET /api/events", cors(s.handleGetEvents))

	// Automation (triggers and automation tasks)
	s.mux.HandleFunc("GET /api/automation/triggers", cors(s.handleListTriggers))
	s.mux.HandleFunc("GET /api/automation/triggers/{id}", cors(s.handleGetTrigger))
	s.mux.HandleFunc("PUT /api/automation/triggers/{id}", cors(s.handleUpdateTrigger))
	s.mux.HandleFunc("POST /api/automation/triggers/{id}/run", cors(s.handleRunTrigger))
	s.mux.HandleFunc("GET /api/automation/triggers/{id}/history", cors(s.handleGetTriggerHistory))
	s.mux.HandleFunc("POST /api/automation/triggers/{id}/reset", cors(s.handleResetTrigger))
	s.mux.HandleFunc("GET /api/automation/tasks", cors(s.handleListAutomationTasks))
	s.mux.HandleFunc("GET /api/automation/stats", cors(s.handleGetAutomationStats))

	// Notifications
	s.mux.HandleFunc("GET /api/notifications", cors(s.handleListNotifications))
	s.mux.HandleFunc("PUT /api/notifications/{id}/dismiss", cors(s.handleDismissNotification))
	s.mux.HandleFunc("PUT /api/notifications/dismiss-all", cors(s.handleDismissAllNotifications))

	// Team (team mode infrastructure)
	s.mux.HandleFunc("GET /api/team/members", cors(s.handleListTeamMembers))
	s.mux.HandleFunc("POST /api/team/members", cors(s.handleCreateTeamMember))
	s.mux.HandleFunc("GET /api/team/members/{id}", cors(s.handleGetTeamMember))
	s.mux.HandleFunc("PUT /api/team/members/{id}", cors(s.handleUpdateTeamMember))
	s.mux.HandleFunc("DELETE /api/team/members/{id}", cors(s.handleDeleteTeamMember))
	s.mux.HandleFunc("GET /api/team/members/{id}/claims", cors(s.handleGetMemberClaims))
	s.mux.HandleFunc("POST /api/tasks/{id}/claim", cors(s.handleClaimTask))
	s.mux.HandleFunc("POST /api/tasks/{id}/release", cors(s.handleReleaseTask))
	s.mux.HandleFunc("GET /api/tasks/{id}/claim", cors(s.handleGetTaskClaim))
	s.mux.HandleFunc("GET /api/team/activity", cors(s.handleListActivity))

	// Branches (branch registry and lifecycle)
	s.mux.HandleFunc("GET /api/branches", cors(s.handleListBranches))
	s.mux.HandleFunc("GET /api/branches/{name}", cors(s.handleGetBranch))
	s.mux.HandleFunc("PATCH /api/branches/{name}/status", cors(s.handleUpdateBranchStatus))
	s.mux.HandleFunc("DELETE /api/branches/{name}", cors(s.handleDeleteBranch))

	// Static files (embedded frontend) - catch-all for non-API routes
	s.mux.Handle("/", staticHandler())
}
