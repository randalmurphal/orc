import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import type { Project } from '@/lib/types';

const STORAGE_KEY = 'orc_current_project_id';
const URL_PARAM = 'project';

// URL helpers
function getUrlProjectId(): string | null {
	if (typeof window === 'undefined') return null;
	const params = new URLSearchParams(window.location.search);
	return params.get(URL_PARAM);
}

function setUrlProjectId(id: string | null, replace = false): void {
	if (typeof window === 'undefined') return;

	const url = new URL(window.location.href);
	if (id) {
		url.searchParams.set(URL_PARAM, id);
	} else {
		url.searchParams.delete(URL_PARAM);
	}

	if (replace) {
		window.history.replaceState({ projectId: id }, '', url.toString());
	} else {
		window.history.pushState({ projectId: id }, '', url.toString());
	}
}

// localStorage helpers
function getStoredProjectId(): string | null {
	if (typeof window === 'undefined') return null;
	try {
		return localStorage.getItem(STORAGE_KEY);
	} catch {
		return null;
	}
}

function setStoredProjectId(id: string | null): void {
	if (typeof window === 'undefined') return;
	try {
		if (id) {
			localStorage.setItem(STORAGE_KEY, id);
		} else {
			localStorage.removeItem(STORAGE_KEY);
		}
	} catch {
		// Ignore localStorage errors
	}
}

interface ProjectStore {
	// State
	projects: Project[];
	currentProjectId: string | null;
	loading: boolean;
	error: string | null;

	// Flag to prevent recursive URL updates during popstate
	_isHandlingPopState: boolean;

	// Derived
	getCurrentProject: () => Project | undefined;

	// Actions
	setProjects: (projects: Project[]) => void;
	selectProject: (id: string | null) => void;
	handlePopState: (event: PopStateEvent) => void;
	initializeFromUrl: () => void;
	setLoading: (loading: boolean) => void;
	setError: (error: string | null) => void;
	reset: () => void;
}

// Get initial project ID: URL param > localStorage
function getInitialProjectId(): string | null {
	const urlId = getUrlProjectId();
	if (urlId) return urlId;
	return getStoredProjectId();
}

const initialState = {
	projects: [] as Project[],
	currentProjectId: null as string | null,
	loading: false,
	error: null as string | null,
	_isHandlingPopState: false,
};

export const useProjectStore = create<ProjectStore>()(
	subscribeWithSelector((set, get) => ({
		...initialState,

		// Derived
		getCurrentProject: () => {
			const { projects, currentProjectId } = get();
			return projects.find((p) => p.id === currentProjectId);
		},

		// Actions
		setProjects: (projects) => {
			const { currentProjectId } = get();

			// Validate current selection exists in new projects
			if (currentProjectId && !projects.some((p) => p.id === currentProjectId)) {
				// Current project not in list, fall back to first project
				const firstProject = projects[0];
				if (firstProject) {
					set({ projects, currentProjectId: firstProject.id });
					setStoredProjectId(firstProject.id);
					setUrlProjectId(firstProject.id, true);
				} else {
					set({ projects, currentProjectId: null });
				}
			} else {
				set({ projects });
			}
		},

		selectProject: (id) => {
			const { _isHandlingPopState } = get();
			set({ currentProjectId: id });
			setStoredProjectId(id);

			// Only update URL if not handling popstate (avoid double push)
			if (!_isHandlingPopState) {
				setUrlProjectId(id, false);
			}
		},

		handlePopState: (event: PopStateEvent) => {
			const projectId = (event.state?.projectId as string | undefined) ?? getUrlProjectId();
			const { currentProjectId } = get();

			if (projectId !== currentProjectId) {
				set({ _isHandlingPopState: true });
				set({ currentProjectId: projectId });
				setStoredProjectId(projectId);
				set({ _isHandlingPopState: false });
			}
		},

		initializeFromUrl: () => {
			const initialId = getInitialProjectId();
			if (initialId) {
				set({ currentProjectId: initialId });
				setStoredProjectId(initialId);
				// Replace URL to ensure it's in sync
				setUrlProjectId(initialId, true);
			}
		},

		setLoading: (loading) => set({ loading }),
		setError: (error) => set({ error }),
		reset: () => set(initialState),
	}))
);

// Subscribe to store changes to sync localStorage
// (This runs after any state change)
useProjectStore.subscribe(
	(state) => state.currentProjectId,
	(currentProjectId) => {
		setStoredProjectId(currentProjectId);
	}
);

// Selector hooks
export const useCurrentProject = () => useProjectStore((state) => state.getCurrentProject());
export const useProjects = () => useProjectStore((state) => state.projects);
export const useCurrentProjectId = () => useProjectStore((state) => state.currentProjectId);
