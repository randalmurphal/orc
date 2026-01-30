# Multi-Project Support Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform orc from single-project-per-server to true multi-tenant architecture where one server serves all registered projects.

**Architecture:** Single API server with LRU-cached project databases. Project context flows through URL paths (`/api/projects/:id/tasks`). Workflows, phases, and agents are global (shared across projects). Frontend routes restructured to `/projects/:id/*`.

**Tech Stack:** Go (server), Connect RPC (API), React 19 + React Router 7 (frontend), SQLite (databases), Zustand (state)

---

## Task 1: Project Database Cache

Create an LRU cache for project databases so the server can serve multiple projects without opening all databases at startup.

**Files:**
- Create: `internal/api/project_cache.go`
- Create: `internal/api/project_cache_test.go`

**Step 1: Write the failing test**

```go
// internal/api/project_cache_test.go
package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/project"
)

func TestProjectCache_GetOpensDatabase(t *testing.T) {
	// Setup: create a temp project with initialized .orc directory
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := config.InitAt(projectPath, false); err != nil {
		t.Fatal(err)
	}

	// Register the project
	proj, err := project.RegisterProject(projectPath)
	if err != nil {
		t.Fatal(err)
	}

	// Create cache
	cache := NewProjectCache(10)

	// Get should open the database
	pdb, err := cache.Get(proj.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if pdb == nil {
		t.Fatal("expected non-nil database")
	}

	// Second get should return cached instance
	pdb2, err := cache.Get(proj.ID)
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if pdb != pdb2 {
		t.Error("expected same database instance from cache")
	}
}

func TestProjectCache_LRUEviction(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 3 projects
	var projectIDs []string
	for i := 0; i < 3; i++ {
		projectPath := filepath.Join(tmpDir, fmt.Sprintf("project-%d", i))
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatal(err)
		}
		if err := config.InitAt(projectPath, false); err != nil {
			t.Fatal(err)
		}
		proj, err := project.RegisterProject(projectPath)
		if err != nil {
			t.Fatal(err)
		}
		projectIDs = append(projectIDs, proj.ID)
	}

	// Cache with max size 2
	cache := NewProjectCache(2)

	// Access projects 0 and 1
	_, _ = cache.Get(projectIDs[0])
	_, _ = cache.Get(projectIDs[1])

	// Access project 2 - should evict project 0 (LRU)
	_, _ = cache.Get(projectIDs[2])

	// Project 0 should be evicted
	if cache.Contains(projectIDs[0]) {
		t.Error("project 0 should have been evicted")
	}
	if !cache.Contains(projectIDs[1]) {
		t.Error("project 1 should still be cached")
	}
	if !cache.Contains(projectIDs[2]) {
		t.Error("project 2 should be cached")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/api/... -run TestProjectCache -v`
Expected: FAIL with "NewProjectCache not defined"

**Step 3: Write minimal implementation**

```go
// internal/api/project_cache.go
package api

import (
	"fmt"
	"sync"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/project"
)

// ProjectCache provides LRU-cached access to project databases.
// Thread-safe for concurrent access.
type ProjectCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	order   []string // LRU order: oldest at front
	maxSize int
}

type cacheEntry struct {
	db   *db.ProjectDB
	path string
}

// NewProjectCache creates a cache with the given maximum size.
func NewProjectCache(maxSize int) *ProjectCache {
	if maxSize < 1 {
		maxSize = 10
	}
	return &ProjectCache{
		entries: make(map[string]*cacheEntry),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
	}
}

// Get returns the ProjectDB for the given project ID.
// Opens the database if not cached, evicting LRU entry if at capacity.
func (c *ProjectCache) Get(projectID string) (*db.ProjectDB, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check cache
	if entry, ok := c.entries[projectID]; ok {
		c.touch(projectID)
		return entry.db, nil
	}

	// Load project from registry
	reg, err := project.LoadRegistry()
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}
	proj, err := reg.Get(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	// Open database
	pdb, err := db.OpenProject(proj.Path)
	if err != nil {
		return nil, fmt.Errorf("open project db: %w", err)
	}

	// Evict if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	// Add to cache
	c.entries[projectID] = &cacheEntry{db: pdb, path: proj.Path}
	c.order = append(c.order, projectID)

	return pdb, nil
}

// Contains checks if a project is in the cache (for testing).
func (c *ProjectCache) Contains(projectID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.entries[projectID]
	return ok
}

// touch moves projectID to end of order (most recently used).
func (c *ProjectCache) touch(projectID string) {
	for i, id := range c.order {
		if id == projectID {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, projectID)
			return
		}
	}
}

// evictOldest removes the least recently used entry.
func (c *ProjectCache) evictOldest() {
	if len(c.order) == 0 {
		return
	}
	oldest := c.order[0]
	c.order = c.order[1:]
	if entry, ok := c.entries[oldest]; ok {
		_ = entry.db.Close()
		delete(c.entries, oldest)
	}
}

// Close closes all cached databases.
func (c *ProjectCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, entry := range c.entries {
		_ = entry.db.Close()
	}
	c.entries = make(map[string]*cacheEntry)
	c.order = nil
	return nil
}

// GetProjectPath returns the filesystem path for a cached project.
func (c *ProjectCache) GetProjectPath(projectID string) (string, error) {
	c.mu.RLock()
	if entry, ok := c.entries[projectID]; ok {
		c.mu.RUnlock()
		return entry.path, nil
	}
	c.mu.RUnlock()

	// Not cached, look up from registry
	reg, err := project.LoadRegistry()
	if err != nil {
		return "", fmt.Errorf("load registry: %w", err)
	}
	proj, err := reg.Get(projectID)
	if err != nil {
		return "", fmt.Errorf("project not found: %w", err)
	}
	return proj.Path, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/api/... -run TestProjectCache -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/project_cache.go internal/api/project_cache_test.go
git commit -m "feat(api): add LRU project database cache

Enables multi-tenant server to manage multiple project databases
with automatic eviction when cache reaches capacity."
```

---

## Task 2: Update Server to Use Project Cache

Modify the API server to use the project cache instead of a single hardcoded project database.

**Files:**
- Modify: `internal/api/server.go`

**Step 1: Read current server.go structure**

Review lines 34-81 (Server struct) and 107-150 (New function) to understand current single-project binding.

**Step 2: Add project cache to Server struct**

In `internal/api/server.go`, modify the Server struct to add the cache:

```go
// Server is the orc API server.
type Server struct {
	addr            string
	maxPortAttempts int
	mux             *http.ServeMux
	logger          *slog.Logger

	// Orc configuration (global, not project-specific)
	orcConfig *config.Config

	// Event publisher for real-time updates
	publisher events.Publisher
	wsHandler *WSHandler

	// Storage backend for global database
	globalDB *db.GlobalDB

	// Project database cache for multi-tenant access
	projectCache *ProjectCache

	// Running tasks for cancellation
	runningTasks   map[string]context.CancelFunc
	runningTasksMu sync.RWMutex

	// Diff cache for computed diffs
	diffCache *diff.Cache

	// PR status poller for periodic updates
	prPoller *PRPoller

	// Automation service for trigger-based automation
	automationSvc *automation.Service

	// Pending gate decisions (for human approval gates in API mode)
	pendingDecisions *gate.PendingDecisionStore

	// Server context for graceful shutdown of background goroutines
	serverCtx       context.Context
	serverCtxCancel context.CancelFunc

	// Session tracking
	sessionID    string
	sessionStart time.Time

	// Session broadcaster for real-time metrics
	sessionBroadcaster *executor.SessionBroadcaster
}
```

**Step 3: Update New() to initialize cache instead of single project**

Replace the single-project database initialization with cache initialization:

```go
func New(cfg *Config) *Server {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Open global database
	globalDB, err := db.OpenGlobal()
	if err != nil {
		panic(fmt.Sprintf("failed to open global database: %v", err))
	}

	// Seed built-in workflows and phase templates to global DB
	if seeded, err := workflow.SeedBuiltinsToGlobal(globalDB); err != nil {
		logger.Warn("failed to seed builtins", "error", err)
	} else if seeded {
		logger.Info("seeded built-in workflows and phases to global database")
	}

	// Create project cache (max 10 projects open simultaneously)
	projectCache := NewProjectCache(10)

	// Load global orc configuration
	orcCfg, err := config.LoadGlobal()
	if err != nil {
		logger.Warn("failed to load global orc config, using defaults", "error", err)
		orcCfg = config.Default()
	}

	// ... rest of initialization using globalDB and projectCache
}
```

**Step 4: Run existing tests to verify no regression**

Run: `go test ./internal/api/... -v -count=1`
Expected: Tests pass (some may need updates for multi-tenant changes)

**Step 5: Commit**

```bash
git add internal/api/server.go
git commit -m "refactor(api): replace single project DB with cache

Server now uses ProjectCache for multi-tenant database access.
Global database opened separately for shared resources."
```

---

## Task 3: Add Project ID to Proto Requests

Update proto definitions to include project_id in all project-scoped requests.

**Files:**
- Modify: `proto/orc/v1/task.proto`
- Modify: `proto/orc/v1/initiative.proto`
- Modify: `proto/orc/v1/transcript.proto`
- Modify: `proto/orc/v1/events.proto`

**Step 1: Update task.proto requests**

Add `project_id` field to all project-scoped task requests:

```protobuf
// In proto/orc/v1/task.proto

message ListTasksRequest {
  string project_id = 1;  // Required: project to list tasks from
  // ... existing fields renumbered
}

message GetTaskRequest {
  string project_id = 1;
  string task_id = 2;
}

message CreateTaskRequest {
  string project_id = 1;
  // ... existing fields
}

message UpdateTaskRequest {
  string project_id = 1;
  string task_id = 2;
  // ... existing fields
}

message DeleteTaskRequest {
  string project_id = 1;
  string task_id = 2;
}

message RunTaskRequest {
  string project_id = 1;
  string task_id = 2;
  // ... existing fields
}
```

**Step 2: Update initiative.proto requests**

```protobuf
message ListInitiativesRequest {
  string project_id = 1;
}

message GetInitiativeRequest {
  string project_id = 1;
  string initiative_id = 2;
}

message CreateInitiativeRequest {
  string project_id = 1;
  // ... existing fields
}
```

**Step 3: Update transcript.proto requests**

```protobuf
message GetTranscriptRequest {
  string project_id = 1;
  string task_id = 2;
}

message StreamTranscriptRequest {
  string project_id = 1;
  string task_id = 2;
}
```

**Step 4: Update events.proto - add project_id to events**

```protobuf
message Event {
  string project_id = 1;  // Which project this event belongs to
  string type = 2;
  // ... existing fields renumbered
}

message SubscribeRequest {
  repeated string project_ids = 1;  // Empty = all projects, ["*"] = all
  // ... existing filters
}
```

**Step 5: Regenerate proto code**

Run: `make proto`
Expected: Generated Go and TypeScript files updated

**Step 6: Commit**

```bash
git add proto/ gen/
git commit -m "proto: add project_id to all project-scoped requests

Breaking change: All task, initiative, transcript, and event
requests now require project_id for multi-tenant routing."
```

---

## Task 4: Update Task Service for Multi-Tenant

Update task_server.go to extract project from request and use project cache.

**Files:**
- Modify: `internal/api/task_server.go`

**Step 1: Update taskServer struct**

Remove single projectDB, add projectCache reference:

```go
type taskServer struct {
	orcv1connect.UnimplementedTaskServiceHandler
	backend      storage.Backend
	config       *config.Config
	logger       *slog.Logger
	publisher    events.Publisher
	projectCache *ProjectCache
	diffCache    *diff.Cache
	startTask    func(ctx context.Context, task *orcv1.Task, opts executor.RunOptions) error
}
```

**Step 2: Add helper to get project DB from request**

```go
func (s *taskServer) getProjectDB(projectID string) (*db.ProjectDB, error) {
	if projectID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("project_id is required"))
	}
	pdb, err := s.projectCache.Get(projectID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("project not found: %w", err))
	}
	return pdb, nil
}
```

**Step 3: Update GetTask to use project from request**

```go
func (s *taskServer) GetTask(
	ctx context.Context,
	req *connect.Request[orcv1.GetTaskRequest],
) (*connect.Response[orcv1.GetTaskResponse], error) {
	pdb, err := s.getProjectDB(req.Msg.ProjectId)
	if err != nil {
		return nil, err
	}

	task, err := db.LoadTask(pdb, req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&orcv1.GetTaskResponse{Task: task}), nil
}
```

**Step 4: Update ListTasks similarly**

```go
func (s *taskServer) ListTasks(
	ctx context.Context,
	req *connect.Request[orcv1.ListTasksRequest],
) (*connect.Response[orcv1.ListTasksResponse], error) {
	pdb, err := s.getProjectDB(req.Msg.ProjectId)
	if err != nil {
		return nil, err
	}

	tasks, err := db.ListTasks(pdb, db.ListOpts{
		// ... existing options
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&orcv1.ListTasksResponse{Tasks: tasks}), nil
}
```

**Step 5: Update remaining methods (CreateTask, UpdateTask, DeleteTask, RunTask)**

Apply same pattern: extract project_id, get DB from cache, operate.

**Step 6: Run tests**

Run: `go test ./internal/api/... -run TestTask -v`
Expected: PASS (may need test updates for new project_id parameter)

**Step 7: Commit**

```bash
git add internal/api/task_server.go
git commit -m "feat(api): update TaskService for multi-tenant

All task operations now route through project cache based on
project_id in request. Removes hardcoded single-project binding."
```

---

## Task 5: Update Initiative Service for Multi-Tenant

Same pattern as Task 4, but for initiatives.

**Files:**
- Modify: `internal/api/initiative_server.go`

**Step 1: Update initiativeServer struct**

```go
type initiativeServer struct {
	orcv1connect.UnimplementedInitiativeServiceHandler
	backend      storage.Backend
	logger       *slog.Logger
	publisher    events.Publisher
	projectCache *ProjectCache
}
```

**Step 2: Add getProjectDB helper and update methods**

Apply same pattern as task_server.go.

**Step 3: Run tests**

Run: `go test ./internal/api/... -run TestInitiative -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/api/initiative_server.go
git commit -m "feat(api): update InitiativeService for multi-tenant"
```

---

## Task 6: Update Transcript and Event Services

**Files:**
- Modify: `internal/api/transcript_server.go`
- Modify: `internal/api/event_server.go`

**Step 1: Update transcript_server.go**

Add projectCache, update GetTranscript and StreamTranscript to use project_id.

**Step 2: Update event_server.go**

- Add project_id to all published events
- Update Subscribe to filter by project_ids
- Forward project context through WebSocket

**Step 3: Run tests**

Run: `go test ./internal/api/... -run "TestTranscript|TestEvent" -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/api/transcript_server.go internal/api/event_server.go
git commit -m "feat(api): update Transcript and Event services for multi-tenant"
```

---

## Task 7: Move Workflows/Phases/Agents to Global DB

Currently workflows are per-project. Move them to global database.

**Files:**
- Modify: `internal/db/workflow.go` (add Global variants)
- Modify: `internal/api/workflow_server.go`
- Modify: `internal/workflow/seed.go`

**Step 1: Add workflow operations to GlobalDB**

```go
// internal/db/global.go (add these methods)

func (g *GlobalDB) SaveWorkflow(w *Workflow) error {
	// Same implementation as ProjectDB.SaveWorkflow
}

func (g *GlobalDB) ListWorkflows() ([]Workflow, error) {
	// Same implementation
}

// ... similar for PhaseTemplate, Agent
```

**Step 2: Update workflow_server.go to use globalDB**

```go
type workflowServer struct {
	globalDB *db.GlobalDB  // Changed from projectDB
	resolver *workflow.Resolver
	cloner   *workflow.Cloner
	cache    *workflow.CacheService
	logger   *slog.Logger
}
```

**Step 3: Update seeding to use global DB**

```go
func SeedBuiltinsToGlobal(gdb *db.GlobalDB) (bool, error) {
	// Seed built-in workflows, phases, agents to global database
}
```

**Step 4: Migrate existing workflow data**

Create one-time migration to copy workflows from project DBs to global DB.

**Step 5: Run tests**

Run: `go test ./internal/api/... -run TestWorkflow -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/db/global.go internal/api/workflow_server.go internal/workflow/seed.go
git commit -m "feat: move workflows/phases/agents to global database

Shared resources now stored in ~/.orc/orc.db for cross-project access."
```

---

## Task 8: CLI --project Flag Infrastructure

Add --project flag to all project-scoped CLI commands.

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/commands.go`
- Create: `internal/cli/project_context.go`

**Step 1: Create project context resolver**

```go
// internal/cli/project_context.go
package cli

import (
	"fmt"
	"os"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/project"
)

var projectFlag string

// ResolveProjectID returns the project ID based on:
// 1. --project flag
// 2. ORC_PROJECT env var
// 3. Current directory detection
// 4. Error if none found
func ResolveProjectID() (string, error) {
	// 1. Flag
	if projectFlag != "" {
		return resolveProjectRef(projectFlag)
	}

	// 2. Env var
	if envProject := os.Getenv("ORC_PROJECT"); envProject != "" {
		return resolveProjectRef(envProject)
	}

	// 3. Cwd detection
	projectRoot, err := config.FindProjectRoot()
	if err == nil {
		reg, err := project.LoadRegistry()
		if err != nil {
			return "", fmt.Errorf("load registry: %w", err)
		}
		proj, err := reg.Get(projectRoot)
		if err == nil {
			return proj.ID, nil
		}
		// Try by path
		for _, p := range reg.Projects {
			if p.Path == projectRoot {
				return p.ID, nil
			}
		}
	}

	// 4. Error
	return "", fmt.Errorf("not in an orc project; use --project or cd to a project directory")
}

// resolveProjectRef resolves a project reference (ID, name, or path) to an ID.
func resolveProjectRef(ref string) (string, error) {
	reg, err := project.LoadRegistry()
	if err != nil {
		return "", fmt.Errorf("load registry: %w", err)
	}

	// Try as ID
	if proj, err := reg.Get(ref); err == nil {
		return proj.ID, nil
	}

	// Try as name (must be unique)
	var matches []project.Project
	for _, p := range reg.Projects {
		if p.Name == ref {
			matches = append(matches, p)
		}
	}
	if len(matches) == 1 {
		return matches[0].ID, nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous project name %q matches %d projects", ref, len(matches))
	}

	// Try as path
	for _, p := range reg.Projects {
		if p.Path == ref {
			return p.ID, nil
		}
	}

	return "", fmt.Errorf("project not found: %s", ref)
}
```

**Step 2: Add persistent flag to root command**

```go
// internal/cli/root.go
func init() {
	rootCmd.PersistentFlags().StringVarP(&projectFlag, "project", "P", "", "Project ID, name, or path")
}
```

**Step 3: Update project-scoped commands to use resolver**

Example for `orc new`:

```go
// internal/cli/cmd_new.go
RunE: func(cmd *cobra.Command, args []string) error {
	projectID, err := ResolveProjectID()
	if err != nil {
		return err
	}

	// Use projectID in API call or local operation
	// ...
}
```

**Step 4: Run CLI tests**

Run: `go test ./internal/cli/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/root.go internal/cli/project_context.go
git commit -m "feat(cli): add --project flag for multi-project support

Resolves project from: flag > env var > cwd detection > error"
```

---

## Task 9: Update CLI Commands to Use Project Context

Apply project context to all project-scoped commands.

**Files:**
- Modify: `internal/cli/cmd_new.go`
- Modify: `internal/cli/cmd_run.go`
- Modify: `internal/cli/cmd_status.go`
- Modify: `internal/cli/cmd_show.go`
- Modify: `internal/cli/cmd_list.go`
- Modify: `internal/cli/cmd_initiative.go`
- (and all other project-scoped commands)

**Step 1: Update each command**

For each project-scoped command, add `ResolveProjectID()` call at the start and pass to operations.

**Step 2: Run full CLI test suite**

Run: `go test ./internal/cli/... -v`
Expected: PASS

**Step 3: Manual verification**

```bash
# From orc directory
./bin/orc status  # Should work (cwd detection)

# From different directory
cd /tmp
./path/to/orc status --project orc  # Should work
./path/to/orc status  # Should error: "not in an orc project"
```

**Step 4: Commit**

```bash
git add internal/cli/cmd_*.go
git commit -m "feat(cli): update all commands to use project context"
```

---

## Task 10: Add CLI Projects Subcommands

Add `orc projects add` and `orc projects remove` commands.

**Files:**
- Modify: `internal/cli/cmd_projects.go`

**Step 1: Add add subcommand**

```go
func newProjectsAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <path>",
		Short: "Register a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			// Resolve to absolute path
			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}

			// Verify .orc directory exists
			orcDir := filepath.Join(absPath, ".orc")
			if _, err := os.Stat(orcDir); os.IsNotExist(err) {
				return fmt.Errorf("not an orc project: %s (missing .orc directory)", absPath)
			}

			proj, err := project.RegisterProject(absPath)
			if err != nil {
				return fmt.Errorf("register project: %w", err)
			}

			fmt.Printf("Registered project: %s (%s)\n", proj.Name, proj.ID)
			return nil
		},
	}
}

func newProjectsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Unregister a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			idOrPath := args[0]

			reg, err := project.LoadRegistry()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			if err := reg.Unregister(idOrPath); err != nil {
				return err
			}

			if err := reg.Save(); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			fmt.Printf("Unregistered project: %s\n", idOrPath)
			return nil
		},
	}
}
```

**Step 2: Register subcommands**

```go
func newProjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Manage registered orc projects",
		// ... existing list behavior as default
	}
	cmd.AddCommand(newProjectsAddCmd())
	cmd.AddCommand(newProjectsRemoveCmd())
	return cmd
}
```

**Step 3: Test manually**

```bash
./bin/orc projects add /path/to/project
./bin/orc projects
./bin/orc projects remove abc123
```

**Step 4: Commit**

```bash
git add internal/cli/cmd_projects.go
git commit -m "feat(cli): add projects add/remove subcommands"
```

---

## Task 11: Frontend - Create Project Picker Page

New landing page that shows all projects with option to add new ones.

**Files:**
- Create: `web/src/pages/ProjectPickerPage.tsx`
- Create: `web/src/pages/ProjectPickerPage.css`

**Step 1: Create the page component**

```tsx
// web/src/pages/ProjectPickerPage.tsx
import { useState, useCallback, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useProjects, useProjectStore } from '@/stores/projectStore';
import { Button, Input } from '@/components/ui';
import { Icon } from '@/components/ui/Icon';
import { projectService } from '@/lib/client';
import './ProjectPickerPage.css';

export function ProjectPickerPage() {
  const projects = useProjects();
  const navigate = useNavigate();
  const [showAddForm, setShowAddForm] = useState(false);
  const [newPath, setNewPath] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSelectProject = useCallback((projectId: string) => {
    navigate(`/projects/${projectId}/board`);
  }, [navigate]);

  const handleAddProject = useCallback(async () => {
    if (!newPath.trim()) return;

    setLoading(true);
    setError(null);

    try {
      const response = await projectService.addProject({ path: newPath.trim() });
      useProjectStore.getState().setProjects([
        ...useProjectStore.getState().projects,
        response.project!,
      ]);
      setNewPath('');
      setShowAddForm(false);
      navigate(`/projects/${response.project!.id}/board`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add project');
    } finally {
      setLoading(false);
    }
  }, [newPath, navigate]);

  return (
    <div className="project-picker">
      <div className="project-picker__header">
        <h1>Select a Project</h1>
        <Button onClick={() => setShowAddForm(true)} variant="primary">
          <Icon name="plus" size={16} />
          Add Project
        </Button>
      </div>

      {showAddForm && (
        <div className="project-picker__add-form">
          <Input
            value={newPath}
            onChange={(e) => setNewPath(e.target.value)}
            placeholder="/path/to/project"
            autoFocus
          />
          <Button onClick={handleAddProject} disabled={loading}>
            {loading ? 'Adding...' : 'Add'}
          </Button>
          <Button variant="ghost" onClick={() => setShowAddForm(false)}>
            Cancel
          </Button>
          {error && <div className="project-picker__error">{error}</div>}
        </div>
      )}

      <div className="project-picker__grid">
        {projects.map((project) => (
          <button
            key={project.id}
            className="project-picker__card"
            onClick={() => handleSelectProject(project.id)}
          >
            <div className="project-picker__card-name">{project.name}</div>
            <div className="project-picker__card-path">{project.path}</div>
          </button>
        ))}

        {projects.length === 0 && (
          <div className="project-picker__empty">
            No projects registered. Add a project to get started.
          </div>
        )}
      </div>
    </div>
  );
}
```

**Step 2: Create CSS**

```css
/* web/src/pages/ProjectPickerPage.css */
.project-picker {
  max-width: 800px;
  margin: 0 auto;
  padding: var(--spacing-8);
}

.project-picker__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-6);
}

.project-picker__header h1 {
  font-size: var(--font-size-2xl);
  font-weight: var(--font-weight-semibold);
}

.project-picker__add-form {
  display: flex;
  gap: var(--spacing-2);
  margin-bottom: var(--spacing-6);
  padding: var(--spacing-4);
  background: var(--surface-secondary);
  border-radius: var(--radius-md);
}

.project-picker__error {
  color: var(--color-error);
  font-size: var(--font-size-sm);
}

.project-picker__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: var(--spacing-4);
}

.project-picker__card {
  padding: var(--spacing-4);
  background: var(--surface-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s;
}

.project-picker__card:hover {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 1px var(--color-primary);
}

.project-picker__card-name {
  font-weight: var(--font-weight-medium);
  font-size: var(--font-size-lg);
  margin-bottom: var(--spacing-1);
}

.project-picker__card-path {
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  font-family: var(--font-mono);
}

.project-picker__empty {
  grid-column: 1 / -1;
  text-align: center;
  padding: var(--spacing-8);
  color: var(--text-secondary);
}
```

**Step 3: Run dev server and verify**

Run: `cd web && bun run dev`
Navigate to page, verify rendering.

**Step 4: Commit**

```bash
git add web/src/pages/ProjectPickerPage.tsx web/src/pages/ProjectPickerPage.css
git commit -m "feat(web): add project picker landing page"
```

---

## Task 12: Frontend - Restructure Router

Update router to use `/projects/:projectId/*` structure.

**Files:**
- Modify: `web/src/router/routes.tsx`

**Step 1: Restructure routes**

```tsx
// web/src/router/routes.tsx
export const routes: RouteObject[] = [
  // Landing page - project picker
  {
    path: '/',
    element: <ProjectPickerPage />,
  },
  // Project-scoped routes
  {
    path: '/projects/:projectId',
    element: <AppShellLayout />,
    errorElement: <ErrorBoundary />,
    children: [
      { index: true, element: <Navigate to="board" replace /> },
      { path: 'board', element: <LazyRoute><Board /></LazyRoute> },
      { path: 'tasks/:id', element: <LazyRoute><TaskDetail /></LazyRoute> },
      { path: 'initiatives', element: <LazyRoute><InitiativesPage /></LazyRoute> },
      { path: 'initiatives/:id', element: <LazyRoute><InitiativeDetailPage /></LazyRoute> },
      { path: 'timeline', element: <LazyRoute><TimelinePage /></LazyRoute> },
      { path: 'stats', element: <LazyRoute><StatsPage /></LazyRoute> },
      { path: 'settings/*', element: <LazyRoute><SettingsPage /></LazyRoute> },
      // 404 within project
      { path: '*', element: <LazyRoute><NotFoundPage /></LazyRoute> },
    ],
  },
  // Global routes (no project context)
  {
    path: '/workflows',
    element: <AppShellLayout />,
    children: [
      { index: true, element: <LazyRoute><WorkflowsPage /></LazyRoute> },
      { path: ':id', element: <LazyRoute><WorkflowEditorPage /></LazyRoute> },
    ],
  },
  {
    path: '/agents',
    element: <AppShellLayout />,
    children: [
      { index: true, element: <LazyRoute><AgentsView /></LazyRoute> },
    ],
  },
  // Global settings
  {
    path: '/settings',
    element: <AppShellLayout />,
    children: [
      { index: true, element: <Navigate to="commands" replace /> },
      // Global settings routes...
    ],
  },
  // 404
  {
    path: '*',
    element: <NotFoundPage />,
  },
];
```

**Step 2: Update AppShellLayout to read projectId from route**

```tsx
function AppShellLayout() {
  const { projectId } = useParams();
  // Pass projectId to context or children as needed
  // ...
}
```

**Step 3: Run and verify navigation**

Run: `cd web && bun run dev`
Test: Navigate through routes, verify URLs are correct.

**Step 4: Commit**

```bash
git add web/src/router/routes.tsx
git commit -m "feat(web): restructure router for multi-project URLs"
```

---

## Task 13: Frontend - Update API Calls with Project ID

Update all API calls to include project ID from route.

**Files:**
- Create: `web/src/hooks/useProjectId.ts`
- Modify: `web/src/stores/taskStore.ts`
- Modify: `web/src/stores/initiativeStore.ts`
- Modify: `web/src/components/layout/DataProvider.tsx`

**Step 1: Create useProjectId hook**

```tsx
// web/src/hooks/useProjectId.ts
import { useParams } from 'react-router-dom';

export function useProjectId(): string | undefined {
  const { projectId } = useParams<{ projectId: string }>();
  return projectId;
}

export function useRequiredProjectId(): string {
  const projectId = useProjectId();
  if (!projectId) {
    throw new Error('useRequiredProjectId must be used within a project route');
  }
  return projectId;
}
```

**Step 2: Update taskStore to accept projectId**

```tsx
// web/src/stores/taskStore.ts
interface TaskStore {
  // ...
  fetchTasks: (projectId: string) => Promise<void>;
}

// Update fetchTasks to include projectId in request
fetchTasks: async (projectId: string) => {
  const response = await taskService.listTasks({ projectId });
  set({ tasks: response.tasks });
}
```

**Step 3: Update DataProvider to use projectId from route**

```tsx
// web/src/components/layout/DataProvider.tsx
export function DataProvider({ children }: { children: React.ReactNode }) {
  const projectId = useProjectId();

  useEffect(() => {
    if (projectId) {
      useTaskStore.getState().fetchTasks(projectId);
      useInitiativeStore.getState().fetchInitiatives(projectId);
    }
  }, [projectId]);

  // ...
}
```

**Step 4: Update remaining stores and components**

Apply same pattern to all project-scoped data fetching.

**Step 5: Run tests**

Run: `cd web && bun run test`
Expected: PASS (with test updates)

**Step 6: Commit**

```bash
git add web/src/hooks/useProjectId.ts web/src/stores/*.ts web/src/components/layout/DataProvider.tsx
git commit -m "feat(web): update API calls to include project ID"
```

---

## Task 14: Frontend - Update WebSocket Subscription

Update WebSocket to subscribe to current project.

**Files:**
- Modify: `web/src/hooks/useEvents.tsx`

**Step 1: Update subscription to filter by project**

```tsx
// In EventProvider
const projectId = useProjectId();

useEffect(() => {
  if (!socket) return;

  // Subscribe to current project
  socket.send(JSON.stringify({
    type: 'subscribe',
    project_ids: projectId ? [projectId] : ['*'],
  }));
}, [socket, projectId]);
```

**Step 2: Update event handlers to check project_id**

```tsx
// In message handler
const handleMessage = (event: MessageEvent) => {
  const data = JSON.parse(event.data);

  // Skip events for other projects
  if (projectId && data.project_id && data.project_id !== projectId) {
    return;
  }

  // Handle event...
};
```

**Step 3: Commit**

```bash
git add web/src/hooks/useEvents.tsx
git commit -m "feat(web): update WebSocket subscription for project filtering"
```

---

## Task 15: Frontend - Update Navigation Links

Update all internal links to include project ID.

**Files:**
- Modify: `web/src/components/layout/IconNav.tsx`
- Modify: `web/src/components/board/TaskCard.tsx`
- Modify: Various components with Link/navigate calls

**Step 1: Update IconNav to use project-scoped paths**

```tsx
// web/src/components/layout/IconNav.tsx
const projectId = useProjectId();

const navItems = projectId ? [
  { path: `/projects/${projectId}/board`, icon: 'layout', label: 'Board' },
  { path: `/projects/${projectId}/initiatives`, icon: 'flag', label: 'Initiatives' },
  { path: `/projects/${projectId}/timeline`, icon: 'clock', label: 'Timeline' },
  { path: `/projects/${projectId}/stats`, icon: 'bar-chart', label: 'Stats' },
] : [];

const globalItems = [
  { path: '/workflows', icon: 'git-branch', label: 'Workflows' },
  { path: '/agents', icon: 'bot', label: 'Agents' },
  { path: '/settings', icon: 'settings', label: 'Settings' },
];
```

**Step 2: Update TaskCard links**

```tsx
// web/src/components/board/TaskCard.tsx
const projectId = useProjectId();

const handleClick = () => {
  navigate(`/projects/${projectId}/tasks/${task.id}`);
};
```

**Step 3: Search for all navigate() and Link calls, update as needed**

Run: `grep -r "navigate\|<Link" web/src --include="*.tsx"`
Update each to use project-scoped paths where appropriate.

**Step 4: Commit**

```bash
git add web/src/components/
git commit -m "feat(web): update navigation links for multi-project routes"
```

---

## Task 16: Cleanup and Remove Single-Project Code

Remove all deprecated single-project code paths.

**Files:**
- Modify: `internal/api/server.go` (remove workDir, backend fields)
- Modify: `internal/config/config.go` (remove FindProjectRoot from server usage)
- Delete or update tests that assume single-project

**Step 1: Remove deprecated fields from Server struct**

Remove: `workDir`, `backend`, `projectDB` fields that were for single-project mode.

**Step 2: Update all server initialization code**

Ensure nothing references the old single-project pattern.

**Step 3: Run full test suite**

Run: `make test`
Expected: PASS

**Step 4: Run full web test suite**

Run: `cd web && bun run test`
Expected: PASS

**Step 5: Manual end-to-end test**

```bash
# Start server
./bin/orc serve

# In browser
# 1. Open http://localhost:8080
# 2. Should see project picker
# 3. Click a project
# 4. Should navigate to /projects/abc123/board
# 5. Tasks should load for that project
# 6. Switch to different project
# 7. Tasks should update
```

**Step 6: Commit**

```bash
git add -A
git commit -m "chore: remove single-project code paths

Breaking change: Server no longer binds to single project.
All operations require explicit project context."
```

---

## Task 17: Update Documentation

Update CLAUDE.md and other docs to reflect multi-project architecture.

**Files:**
- Modify: `CLAUDE.md`
- Modify: `internal/api/CLAUDE.md`
- Modify: `web/CLAUDE.md`

**Step 1: Update main CLAUDE.md**

Add section on multi-project support, API route structure, CLI usage.

**Step 2: Update API CLAUDE.md**

Document new project-scoped vs global routes.

**Step 3: Update web CLAUDE.md**

Document new route structure, useProjectId hook.

**Step 4: Commit**

```bash
git add CLAUDE.md internal/api/CLAUDE.md web/CLAUDE.md
git commit -m "docs: update documentation for multi-project support"
```

---

## Final Verification

**Run all tests:**
```bash
make test
cd web && bun run test
```

**Manual smoke test:**
1. Start server: `./bin/orc serve`
2. Open browser to `http://localhost:8080`
3. Verify project picker shows
4. Select a project
5. Verify board loads with tasks
6. Create a task
7. Switch projects
8. Verify different tasks load
9. CLI: `orc status --project orc` works
10. CLI: `orc new "test" --project llmkit` creates in correct project

**Create final commit:**
```bash
git add -A
git commit -m "feat: complete multi-project support implementation

- Single server serves all registered projects
- LRU cache for project databases
- Project context via URL paths (/projects/:id/*)
- CLI --project flag with cwd fallback
- Global workflows/phases/agents
- Project picker landing page"
```
