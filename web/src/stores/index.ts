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
