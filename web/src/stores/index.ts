// Task store
export {
	useTaskStore,
	useActiveTasks,
	useRecentTasks,
	useRunningTasks,
	useStatusCounts,
	useTask,
	useTaskState,
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
} from './initiativeStore';

// UI store
export {
	useUIStore,
	useSidebarExpanded,
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
