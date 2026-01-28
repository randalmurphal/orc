package config

import "time"

// Default returns the default configuration.
// Default is AUTOMATION-FIRST: all gates auto, retry enabled.
func Default() *Config {
	return &Config{
		Version: 1,
		Profile: ProfileAuto,
		Gates: GateConfig{
			DefaultType:          "auto",
			AutoApproveOnSuccess: true,
			RetryOnFailure:       true,
			MaxRetries:           5,
			// No phase or weight overrides by default - everything is auto
		},
		Retry: RetryConfig{
			Enabled:    true,
			MaxRetries: 5,
			// Default retry map: if phase fails, go back to earlier phase
			// Review uses three-tier approach: fix in-place, block for major issues,
			// or block with detailed context for wrong approach
			RetryMap: map[string]string{
				"test":      "implement",
				"test_unit": "implement",
				"test_e2e":  "implement",
				"review":    "implement", // Major issues; small ones fixed in-place
			},
		},
		Worktree: WorktreeConfig{
			Enabled:           true,
			Dir:               ".orc/worktrees",
			CleanupOnComplete: true,
			CleanupOnFail:     false, // Keep for debugging
		},
		Completion: CompletionConfig{
			Action:        "pr",
			TargetBranch:  "main",
			DeleteBranch:  true,
			WaitForCI:     true,             // Wait for CI before merge (replaces auto-merge)
			CITimeout:     10 * time.Minute, // 10 minute default timeout
			MergeOnCIPass: true,             // Merge when CI passes
			PR: PRConfig{
				Title:        "[orc] {{TASK_TITLE}}",
				BodyTemplate: "templates/pr-body.md",
				Labels:       []string{"automated"},
				AutoMerge:    true,
				AutoApprove:  true, // AI-assisted PR approval in auto mode
			},
			CI: CIConfig{
				WaitForCI:     true,             // Wait for CI checks before merge
				CITimeout:     10 * time.Minute, // Max 10 minutes to wait
				PollInterval:  30 * time.Second, // Check every 30 seconds
				MergeOnCIPass: true,             // Auto-merge when CI passes
				MergeMethod:   "squash",         // Use squash merge by default
			},
			Sync: SyncConfig{
				Strategy:         SyncStrategyCompletion, // Sync before PR creation by default
				SyncOnStart:      true,                   // Sync at task start to catch stale worktrees
				FailOnConflict:   true,                   // Fail on conflicts by default - let user decide resolution
				MaxConflictFiles: 0,                      // No limit by default
				SkipForWeights:   []string{"trivial"},    // Skip sync for trivial tasks
			},
			Finalize: FinalizeConfig{
				Enabled:               true, // Finalize phase enabled by default
				AutoTrigger:           true, // Auto-trigger after validate
				AutoTriggerOnApproval: true, // Auto-trigger when PR is approved (auto profile only)
				Sync: FinalizeSyncConfig{
					Strategy: FinalizeSyncMerge, // Merge target into branch (preserves history)
				},
				ConflictResolution: ConflictResolutionConfig{
					Enabled:      true, // AI-assisted conflict resolution enabled
					Instructions: "",   // No additional instructions by default
				},
				RiskAssessment: RiskAssessmentConfig{
					Enabled:           true,   // Risk assessment enabled
					ReReviewThreshold: "high", // Recommend re-review at high+ risk
				},
				Gates: FinalizeGatesConfig{
					PreMerge: "auto", // Auto gate before merge/PR by default
				},
			},
			// Safety defaults: use PR workflow for all weights
			// Direct merge is blocked for protected branches (main, master, develop, release)
			// Override per-weight via config if needed (e.g., "trivial": "merge")
			WeightActions: map[string]string{},
		},
		Execution: ExecutionConfig{
			UseSessionExecution: false, // Default to flowgraph for compatibility
			SessionPersistence:  true,
			CheckpointInterval:  0,  // Default to phase-complete only
			MaxRetries:          5,  // Default retry limit for phase failures
			ParallelTasks:       2,  // Default parallel tasks for UI
			CostLimit:           25, // Default cost limit ($25/day) for UI
		},
		Pool: PoolConfig{
			Enabled:    false, // Disabled by default
			ConfigPath: "~/.orc/token-pool/pool.yaml",
		},
		Server: ServerConfig{
			Host:            "127.0.0.1",
			Port:            8080,
			MaxPortAttempts: 10,
			Auth: AuthConfig{
				Enabled: false,
				Type:    "token",
			},
		},
		Team: TeamConfig{
			Name:            "",    // Auto-detected from username
			ActivityLogging: true,  // On by default - useful history even for solo
			TaskClaiming:    false, // Off by default - opt-in for multi-user
			Visibility:      "all",
			Mode:            "local", // Local by default, shared_db for teams
			ServerURL:       "",
		},
		Identity: IdentityConfig{
			Initials:    "",
			DisplayName: "",
		},
		TaskID: TaskIDConfig{
			Mode:         "solo",
			PrefixSource: "initials",
		},
		Testing: TestingConfig{
			Required:          true,
			CoverageThreshold: 85, // Default: 85% coverage required
			Types:             []string{"unit"},
			SkipForWeights:    []string{"trivial"},
			Commands: TestCommands{
				Unit:        "go test ./...",
				Integration: "go test -tags=integration ./...",
				E2E:         "make e2e",
				Coverage:    "go test -coverprofile=coverage.out ./...",
			},
			ParseOutput: true,
		},
		Validation: ValidationConfig{
			Enabled:          true,                // Validation enabled by default
			Model:            "haiku",             // Haiku for fast validation
			SkipForWeights:   []string{"trivial"}, // Only skip for trivial tasks
			ValidateSpecs:    true,                // Haiku validates spec quality
			ValidateCriteria: true,                // Haiku validates success criteria on completion
			FailOnAPIError:   true,                // Fail properly on API errors (resumable)
		},
		Documentation: DocumentationConfig{
			Enabled:            true,
			AutoUpdateClaudeMD: true,
			UpdateOn:           []string{"feature", "api_change"},
			SkipForWeights:     []string{"trivial"},
			Sections:           []string{"api-endpoints", "commands", "config-options"},
		},
		Timeouts: TimeoutsConfig{
			PhaseMax:          60 * time.Minute,
			TurnMax:           10 * time.Minute,
			IdleWarning:       5 * time.Minute,
			HeartbeatInterval: 30 * time.Second,
			IdleTimeout:       2 * time.Minute,
		},
		QA: QAConfig{
			Enabled:        true,
			SkipForWeights: []string{"trivial"},
			RequireE2E:     false,
			GenerateDocs:   true,
		},
		Review: ReviewConfig{
			Enabled:     true,
			Rounds:      2,
			RequirePass: true,
		},
		Plan: PlanConfig{
			MinimumSections: []string{"intent", "success_criteria", "testing"},
		},
		Weights: WeightsConfig{
			Trivial: "implement-trivial",
			Small:   "implement-small",
			Medium:  "implement-medium",
			Large:   "implement-large",
		},
		ArtifactSkip: ArtifactSkipConfig{
			Enabled:  true,                                 // Check for existing artifacts
			AutoSkip: false,                                // Prompt user by default
			Phases:   []string{"spec", "research", "docs"}, // Safe phases to skip
		},
		Subtasks: SubtasksConfig{
			AllowCreation: true,
			AutoApprove:   false,
			MaxPending:    10,
		},
		Tasks: TasksConfig{
			DisableAutoCommit: false, // Auto-commit enabled by default
		},
		Diagnostics: DiagnosticsConfig{
			ResourceTracking: ResourceTrackingConfig{
				Enabled:               true, // Enabled by default to detect orphaned processes
				MemoryThresholdMB:     500,  // Warn if memory grows by >500MB
				FilterSystemProcesses: true, // Filter system processes to avoid false positives
			},
		},
		MCP: MCPConfig{
			Playwright: PlaywrightConfig{
				Enabled:           true,       // Auto-configure for UI tasks
				Headless:          true,       // Headless for CI, override for debugging
				Browser:           "chromium", // Default browser
				TimeoutAction:     5000,       // 5s action timeout
				TimeoutNavigation: 60000,      // 60s navigation timeout
			},
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			SQLite: SQLiteConfig{
				Path:       ".orc/orc.db",
				GlobalPath: "~/.orc/orc.db",
			},
			Postgres: PostgresConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "orc",
				User:     "orc",
				SSLMode:  "disable",
				PoolMax:  10,
			},
		},
		Storage: StorageConfig{
			Mode: StorageModeHybrid, // Best of both worlds for solo devs
			Files: FileStorageConfig{
				CleanupOnComplete: true, // Keep .orc/tasks/ clean
			},
			Database: DatabaseStorageConfig{
				CacheTranscripts: true, // FTS search enabled by default
				RetentionDays:    90,   // Auto-cleanup old entries
			},
			Export: ExportConfig{
				Enabled:        false, // Nothing exported by default
				TaskDefinition: true,  // When enabled, export task.yaml + plan.yaml
				FinalState:     true,  // When enabled, export state.yaml
				Transcripts:    false, // Usually too large
				ContextSummary: true,  // When enabled, export context.md
			},
		},
		Automation: AutomationConfig{
			Enabled:        true,               // Automation enabled by default
			AutoApprove:    true,               // Auto-approve safe operations by default
			GlobalCooldown: 30 * time.Minute,   // 30 minute global cooldown
			MaxConcurrent:  1,                  // One automation task at a time
			DefaultMode:    AutomationModeAuto, // Auto mode by default
			Triggers:       nil,                // No triggers defined by default
			Templates:      nil,                // No templates defined by default
		},
		Model:                      "opus",
		MaxIterations:              30,
		Timeout:                    10 * time.Minute,
		BranchPrefix:               "orc/",
		CommitPrefix:               "[orc]",
		ClaudePath:                 "claude",
		DangerouslySkipPermissions: true,
		TemplatesDir:               "templates",
		EnableCheckpoints:          true,
	}
}

// ProfilePresets returns gate configuration for a given automation profile.
func ProfilePresets(profile AutomationProfile) GateConfig {
	switch profile {
	case ProfileFast:
		// Fast: everything auto, no AI review, fewer retries for speed
		return GateConfig{
			DefaultType:          "auto",
			AutoApproveOnSuccess: true,
			RetryOnFailure:       true,
			MaxRetries:           2,
		}
	case ProfileSafe:
		// Safe: AI reviews, human only for merge
		return GateConfig{
			DefaultType:          "auto",
			AutoApproveOnSuccess: true,
			RetryOnFailure:       true,
			MaxRetries:           5,
			PhaseOverrides: map[string]string{
				"review": "ai",
				"merge":  "human",
			},
		}
	case ProfileStrict:
		// Strict: human gates on key decisions
		return GateConfig{
			DefaultType:          "auto",
			AutoApproveOnSuccess: true,
			RetryOnFailure:       true,
			MaxRetries:           5,
			PhaseOverrides: map[string]string{
				"spec":   "human",
				"review": "ai",
				"merge":  "human",
			},
		}
	default: // ProfileAuto
		// Auto: fully automated, no human intervention
		return GateConfig{
			DefaultType:          "auto",
			AutoApproveOnSuccess: true,
			RetryOnFailure:       true,
			MaxRetries:           5,
		}
	}
}

// PRAutoApprovePreset returns the auto-approve setting for a given automation profile.
func PRAutoApprovePreset(profile AutomationProfile) bool {
	switch profile {
	case ProfileAuto, ProfileFast:
		// Auto and Fast profiles enable AI-assisted PR approval
		return true
	case ProfileSafe, ProfileStrict:
		// Safe and Strict profiles require human approval
		return false
	default:
		return true // Default to auto
	}
}

// FinalizePresets returns finalize configuration for a given automation profile.
func FinalizePresets(profile AutomationProfile) FinalizeConfig {
	switch profile {
	case ProfileFast:
		// Fast: minimal overhead, rebase for linear history, skip risk assessment
		return FinalizeConfig{
			Enabled:               true,
			AutoTrigger:           true,
			AutoTriggerOnApproval: true, // Auto-trigger on PR approval for speed
			Sync: FinalizeSyncConfig{
				Strategy: FinalizeSyncRebase, // Rebase for cleaner history, faster
			},
			ConflictResolution: ConflictResolutionConfig{
				Enabled: true, // Still resolve conflicts automatically
			},
			RiskAssessment: RiskAssessmentConfig{
				Enabled:           false, // Skip risk assessment for speed
				ReReviewThreshold: "high",
			},
			Gates: FinalizeGatesConfig{
				PreMerge: "none", // No pre-merge gate for speed
			},
		}
	case ProfileSafe:
		// Safe: auto gates, human approval for merge
		// No auto-trigger on approval - wait for human decision
		return FinalizeConfig{
			Enabled:               true,
			AutoTrigger:           true,
			AutoTriggerOnApproval: false, // Don't auto-trigger - humans should review before finalize
			Sync: FinalizeSyncConfig{
				Strategy: FinalizeSyncMerge, // Merge preserves history
			},
			ConflictResolution: ConflictResolutionConfig{
				Enabled: true,
			},
			RiskAssessment: RiskAssessmentConfig{
				Enabled:           true,
				ReReviewThreshold: "medium", // Lower threshold for safety
			},
			Gates: FinalizeGatesConfig{
				PreMerge: "human", // Human approval before merge
			},
		}
	case ProfileStrict:
		// Strict: human gates, merge strategy, strict risk assessment
		// No auto-trigger on approval - humans must explicitly trigger finalize
		return FinalizeConfig{
			Enabled:               true,
			AutoTrigger:           true,
			AutoTriggerOnApproval: false, // Don't auto-trigger - humans must decide
			Sync: FinalizeSyncConfig{
				Strategy: FinalizeSyncMerge, // Merge preserves history
			},
			ConflictResolution: ConflictResolutionConfig{
				Enabled: true,
			},
			RiskAssessment: RiskAssessmentConfig{
				Enabled:           true,
				ReReviewThreshold: "low", // Even low risk triggers re-review
			},
			Gates: FinalizeGatesConfig{
				PreMerge: "human", // Human gate before merge
			},
		}
	default: // ProfileAuto
		// Auto: fully automated, merge strategy, auto gates
		// Auto-trigger on approval for full automation
		return FinalizeConfig{
			Enabled:               true,
			AutoTrigger:           true,
			AutoTriggerOnApproval: true, // Auto-trigger when PR is approved
			Sync: FinalizeSyncConfig{
				Strategy: FinalizeSyncMerge,
			},
			ConflictResolution: ConflictResolutionConfig{
				Enabled: true,
			},
			RiskAssessment: RiskAssessmentConfig{
				Enabled:           true,
				ReReviewThreshold: "high",
			},
			Gates: FinalizeGatesConfig{
				PreMerge: "auto",
			},
		}
	}
}

// ValidationPresets returns validation configuration for a given automation profile.
func ValidationPresets(profile AutomationProfile) ValidationConfig {
	switch profile {
	case ProfileFast:
		// Fast: minimal validation for speed (only for quick iterations)
		return ValidationConfig{
			Enabled:          true,
			Model:            "haiku",
			SkipForWeights:   []string{"trivial", "small"},
			ValidateSpecs:    true,
			ValidateCriteria: false, // Fast: skip criteria validation for speed
			FailOnAPIError:   false, // Fast: fail open for speed
		}
	case ProfileSafe:
		// Safe: quality-focused validation
		return ValidationConfig{
			Enabled:          true,
			Model:            "haiku",
			SkipForWeights:   []string{"trivial"},
			ValidateSpecs:    true,
			ValidateCriteria: true, // Safe: validate criteria
			FailOnAPIError:   true, // Safe: fail properly on API errors
		}
	case ProfileStrict:
		// Strict: maximum validation
		return ValidationConfig{
			Enabled:          true,
			Model:            "haiku",
			SkipForWeights:   []string{}, // No skipping
			ValidateSpecs:    true,
			ValidateCriteria: true, // Strict: always validate criteria
			FailOnAPIError:   true, // Strict: always fail properly on API errors
		}
	default: // ProfileAuto
		// Auto: quality-first validation (default)
		return ValidationConfig{
			Enabled:          true,
			Model:            "haiku",
			SkipForWeights:   []string{"trivial"},
			ValidateSpecs:    true,
			ValidateCriteria: true, // Auto: validate criteria
			FailOnAPIError:   true, // Auto: fail properly on API errors (quality-first)
		}
	}
}
