import { writable, derived, get, type Readable } from 'svelte/store';
import type { Project } from '$lib/types';
import { listProjects, getDefaultProject, setDefaultProject as apiSetDefaultProject } from '$lib/api';

const LOCAL_STORAGE_KEY = 'orc_current_project_id';
const URL_PARAM_KEY = 'project';

// Helper to safely access localStorage (handles SSR and disabled storage)
function getStoredProjectId(): string | null {
	if (typeof window === 'undefined') return null;
	try {
		return localStorage.getItem(LOCAL_STORAGE_KEY);
	} catch {
		return null;
	}
}

function setStoredProjectId(id: string | null): void {
	if (typeof window === 'undefined') return;
	try {
		if (id) {
			localStorage.setItem(LOCAL_STORAGE_KEY, id);
		} else {
			localStorage.removeItem(LOCAL_STORAGE_KEY);
		}
	} catch {
		// Ignore storage errors
	}
}

// Helper to get project ID from URL query parameter
export function getUrlProjectId(): string | null {
	if (typeof window === 'undefined') return null;
	try {
		const params = new URLSearchParams(window.location.search);
		return params.get(URL_PARAM_KEY);
	} catch {
		return null;
	}
}

// Helper to update URL with project ID (pushes to browser history)
export function setUrlProjectId(id: string | null, replace: boolean = false): void {
	if (typeof window === 'undefined') return;
	try {
		const url = new URL(window.location.href);
		if (id) {
			url.searchParams.set(URL_PARAM_KEY, id);
		} else {
			url.searchParams.delete(URL_PARAM_KEY);
		}

		// Only update if URL actually changed
		if (url.href !== window.location.href) {
			if (replace) {
				window.history.replaceState({ projectId: id }, '', url.href);
			} else {
				window.history.pushState({ projectId: id }, '', url.href);
			}
		}
	} catch {
		// Ignore URL errors
	}
}

// Get initial project ID: URL param takes precedence over localStorage
function getInitialProjectId(): string | null {
	const urlId = getUrlProjectId();
	if (urlId) return urlId;
	return getStoredProjectId();
}

// Store for available projects
export const projects = writable<Project[]>([]);

// Store for currently selected project (initialized from URL param or localStorage)
export const currentProjectId = writable<string | null>(getInitialProjectId());

// Internal flag to prevent URL update during popstate handling
let isHandlingPopState = false;

// Sync to localStorage whenever currentProjectId changes
// URL is NOT synced here - it's managed by selectProject() to control history
currentProjectId.subscribe(id => {
	setStoredProjectId(id);
});

// Loading and error states
export const projectsLoading = writable<boolean>(false);
export const projectsError = writable<string | null>(null);

// Derived store for current project
export const currentProject: Readable<Project | null> = derived(
	[projects, currentProjectId],
	([$projects, $id]) => {
		if (!$id) return null;
		return $projects.find(p => p.id === $id) ?? null;
	}
);

// Load projects from API
export async function loadProjects(): Promise<void> {
	projectsLoading.set(true);
	projectsError.set(null);

	try {
		const loaded = await listProjects();
		projects.set(loaded);

		// Priority: URL param > localStorage > server default > first project
		// Get current ID (already initialized from URL or localStorage)
		const currentId = get(currentProjectId);

		// Check if current selection is valid in the loaded projects
		const currentIsValid = currentId && loaded.find(p => p.id === currentId);

		if (currentIsValid) {
			// Current selection is valid - ensure URL reflects it (replace, not push)
			setUrlProjectId(currentId, true);
			return;
		}

		// Current selection is invalid - need to find a valid one
		// Try to get default project from server
		let defaultId: string | null = null;
		try {
			const defaultProject = await getDefaultProject();
			if (defaultProject && loaded.find(p => p.id === defaultProject)) {
				defaultId = defaultProject;
			}
		} catch {
			// Server doesn't have a default set, ignore
		}

		// Fall back to first project if no valid selection
		let newId: string | null = null;
		if (defaultId) {
			newId = defaultId;
		} else if (loaded.length > 0) {
			newId = loaded[0].id;
		}

		if (newId) {
			currentProjectId.set(newId);
			// Use replace for initial load to avoid polluting history
			setUrlProjectId(newId, true);
		} else {
			currentProjectId.set(null);
			setUrlProjectId(null, true);
		}
	} catch (e) {
		const errorMsg = e instanceof Error ? e.message : 'Failed to load projects';
		projectsError.set(errorMsg);
		console.error('Failed to load projects:', e);
	} finally {
		projectsLoading.set(false);
	}
}

// Select a project (persist to localStorage and URL, push to browser history)
export function selectProject(id: string): void {
	currentProjectId.set(id);
	// Push to browser history so back button works (unless we're handling popstate)
	if (!isHandlingPopState) {
		setUrlProjectId(id, false);
	}
}

// Handle browser back/forward navigation
export function handlePopState(event: PopStateEvent): void {
	// Check if state has a projectId, or fall back to URL param
	const projectId = event.state?.projectId ?? getUrlProjectId();
	if (projectId && projectId !== get(currentProjectId)) {
		isHandlingPopState = true;
		currentProjectId.set(projectId);
		isHandlingPopState = false;
	}
}

// Set a project as the default (persisted to server)
export async function setDefaultProject(id: string): Promise<void> {
	try {
		await apiSetDefaultProject(id);
	} catch (e) {
		console.error('Failed to set default project:', e);
		throw e;
	}
}
