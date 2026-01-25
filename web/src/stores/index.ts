// Task store
export {
	useTaskStore,
	useActiveTasks,
	useRecentTasks,
	useRunningTasks,
	useStatusCounts,
	useTask,
	useTaskState,
	useTaskActivity,
	type TaskActivity,
} from './taskStore';

// Project store
export {
	useProjectStore,
	useCurrentProject,
	useProjects,
	useCurrentProjectId,
	useProjectLoading,
} from './projectStore';

// Initiative store
export {
	useInitiativeStore,
	useInitiatives,
	useCurrentInitiative,
	useCurrentInitiativeId,
	UNASSIGNED_INITIATIVE,
	truncateInitiativeTitle,
	getInitiativeBadgeTitle,
	type InitiativeBadgeFormat,
} from './initiativeStore';

// UI store
export {
	useUIStore,
	useSidebarExpanded,
	useMobileMenuOpen,
	useWsStatus,
	useToasts,
	toast,
} from './uiStore';

// Dependency store
export {
	useDependencyStore,
	useCurrentDependencyStatus,
	DEPENDENCY_OPTIONS,
	type DependencyStatusFilter,
} from './dependencyStore';

// Preferences store
export {
	usePreferencesStore,
	useTheme,
	useSidebarDefault,
	useBoardViewMode,
	useDateFormat,
	STORAGE_KEYS,
	defaultPreferences,
	type Theme,
	type SidebarDefault,
	type BoardViewMode,
	type DateFormat,
	type Preferences,
} from './preferencesStore';

// Session store
export {
	useSessionStore,
	useSessionId,
	useStartTime,
	useTotalTokens,
	useTotalCost,
	useIsPaused,
	useActiveTaskCount,
	useFormattedDuration,
	useFormattedCost,
	useFormattedTokens,
	useSessionMetrics,
	formatDuration,
	STORAGE_KEYS as SESSION_STORAGE_KEYS,
	type SessionState,
	type SessionActions,
	type SessionStore,
	type SessionMetrics,
} from './sessionStore';

// Stats store
export {
	useStatsStore,
	useStatsPeriod,
	useStatsLoading,
	useStatsError,
	useActivityData,
	useOutcomes,
	useTasksPerDay,
	useTopInitiatives,
	useTopFiles,
	useSummaryStats,
	useWeeklyChanges,
	type StatsPeriod,
	type Outcomes,
	type TasksPerDay,
	type TopInitiative,
	type TopFile,
	type SummaryStats,
	type WeeklyChanges,
	type StatsStore,
} from './statsStore';

// Workflow store
export {
	useWorkflowStore,
	useWorkflows,
	usePhaseTemplates,
	useWorkflowRuns,
	useBuiltinWorkflows,
	useCustomWorkflows,
	useBuiltinPhases,
	useCustomPhases,
	useRunningRuns,
	useSelectedWorkflow,
	useSelectedRun,
} from './workflowStore';
