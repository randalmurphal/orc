/**
 * DataProvider component
 *
 * Handles centralized data loading and synchronization:
 * - Loads projects, tasks, and initiatives on mount
 * - Reloads tasks and initiatives when project changes
 * - Handles browser back/forward navigation
 * - Syncs URL params with stores
 */

import { useEffect, useRef, useCallback, type ReactNode } from 'react';
import {
	useProjectStore,
	useTaskStore,
	useInitiativeStore,
	useDependencyStore,
} from '@/stores';
import { create } from '@bufbuild/protobuf';
import { projectClient, taskClient, initiativeClient } from '@/lib/client';
import { ListProjectsRequestSchema } from '@/gen/orc/v1/project_pb';
import { ListTasksRequestSchema, type Task } from '@/gen/orc/v1/task_pb';
import { ListInitiativesRequestSchema } from '@/gen/orc/v1/initiative_pb';
import { PageRequestSchema } from '@/gen/orc/v1/common_pb';

interface DataProviderProps {
	children: ReactNode;
}

/**
 * DataProvider
 *
 * Wraps the app to provide centralized data loading and synchronization.
 * Should be placed inside WebSocketProvider but outside router.
 */
export function DataProvider({ children }: DataProviderProps) {
	const currentProjectId = useProjectStore((state) => state.currentProjectId);
	const setProjects = useProjectStore((state) => state.setProjects);
	const selectProject = useProjectStore((state) => state.selectProject);
	const setProjectLoading = useProjectStore((state) => state.setLoading);
	const setProjectError = useProjectStore((state) => state.setError);
	const initializeProjectFromUrl = useProjectStore((state) => state.initializeFromUrl);
	const handleProjectPopState = useProjectStore((state) => state.handlePopState);

	const setTasks = useTaskStore((state) => state.setTasks);
	const setTaskLoading = useTaskStore((state) => state.setLoading);
	const setTaskError = useTaskStore((state) => state.setError);
	const resetTasks = useTaskStore((state) => state.reset);

	const setInitiatives = useInitiativeStore((state) => state.setInitiatives);
	const initializeInitiativeFromUrl = useInitiativeStore((state) => state.initializeFromUrl);
	const handleInitiativePopState = useInitiativeStore((state) => state.handlePopState);
	const initiativeReset = useInitiativeStore((state) => state.reset);

	const initializeDependencyFromUrl = useDependencyStore((state) => state.initializeFromUrl);
	const handleDependencyPopState = useDependencyStore((state) => state.handlePopState);

	// Track previous project ID to detect changes
	const prevProjectIdRef = useRef<string | null>(null);
	// Track if initial load has happened
	const initialLoadDone = useRef(false);

	// Load projects on mount
	const loadProjects = useCallback(async () => {
		setProjectLoading(true);
		setProjectError(null);
		try {
			const response = await projectClient.listProjects(create(ListProjectsRequestSchema, {}));
			setProjects(response.projects);

			// If no project selected yet and we have projects, select the first one
			const currentId = useProjectStore.getState().currentProjectId;
			if (!currentId && response.projects.length > 0) {
				selectProject(response.projects[0].id);
			}
		} catch (err) {
			setProjectError(err instanceof Error ? err.message : 'Failed to load projects');
		} finally {
			setProjectLoading(false);
		}
	}, [setProjects, selectProject, setProjectLoading, setProjectError]);

	// Load tasks for current project - paginates to get all tasks
	const loadTasks = useCallback(async (projectId: string | null) => {
		if (!projectId) {
			resetTasks();
			return;
		}

		setTaskLoading(true);
		setTaskError(null);
		try {
			// Fetch all tasks by paginating (backend caps at 100 per request)
			const allTasks: Task[] = [];
			let currentPage = 1;
			let hasMore = true;
			const pageLimit = 100; // Max allowed by backend

			while (hasMore) {
				const response = await taskClient.listTasks(create(ListTasksRequestSchema, {
					projectId,
					page: create(PageRequestSchema, { page: currentPage, limit: pageLimit }),
				}));
				allTasks.push(...response.tasks);
				hasMore = response.page?.hasMore ?? false;
				currentPage++;
				// Safety: prevent infinite loops
				if (currentPage > 1000) break;
			}

			setTasks(allTasks);
		} catch (err) {
			setTaskError(err instanceof Error ? err.message : 'Failed to load tasks');
		} finally {
			setTaskLoading(false);
		}
	}, [setTasks, setTaskLoading, setTaskError, resetTasks]);

	// Load initiatives for a project
	const loadInitiatives = useCallback(async (projectId: string | null) => {
		if (!projectId) {
			initiativeReset();
			return;
		}
		try {
			const response = await initiativeClient.listInitiatives(create(ListInitiativesRequestSchema, { projectId }));
			setInitiatives(response.initiatives);
		} catch (err) {
			console.error('Failed to load initiatives:', err);
			// Don't set error state - initiatives are not critical
		}
	}, [setInitiatives, initiativeReset]);

	// Initial load
	useEffect(() => {
		if (initialLoadDone.current) return;
		initialLoadDone.current = true;

		// Initialize from URL params first
		initializeProjectFromUrl();
		initializeInitiativeFromUrl();
		initializeDependencyFromUrl();

		// Load all data
		const init = async () => {
			await loadProjects();
			// After projects are loaded and selected, load tasks and initiatives for the selected project
			const projectId = useProjectStore.getState().currentProjectId;
			if (projectId) {
				await loadTasks(projectId);
				await loadInitiatives(projectId);
			}
		};
		init();
	}, [loadProjects, loadInitiatives, loadTasks, initializeProjectFromUrl, initializeInitiativeFromUrl, initializeDependencyFromUrl]);

	// Reload tasks and initiatives when project changes
	useEffect(() => {
		// Skip the first run (handled by initial load)
		if (prevProjectIdRef.current === undefined) {
			prevProjectIdRef.current = currentProjectId;
			return;
		}

		// Only reload if project actually changed
		if (prevProjectIdRef.current !== currentProjectId) {
			prevProjectIdRef.current = currentProjectId;

			// Clear existing data first
			resetTasks();
			initiativeReset();

			// Load new data
			loadTasks(currentProjectId);
			loadInitiatives(currentProjectId);
		}
	}, [currentProjectId, loadTasks, loadInitiatives, resetTasks, initiativeReset]);

	// Handle browser back/forward navigation
	useEffect(() => {
		const handlePopState = (e: PopStateEvent) => {
			handleProjectPopState(e);
			handleInitiativePopState(e);
			handleDependencyPopState(e);
		};

		window.addEventListener('popstate', handlePopState);
		return () => window.removeEventListener('popstate', handlePopState);
	}, [handleProjectPopState, handleInitiativePopState, handleDependencyPopState]);

	return <>{children}</>;
}
