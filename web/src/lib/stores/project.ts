import { writable, derived, get, type Readable } from 'svelte/store';
import type { Project } from '$lib/types';
import { listProjects, getDefaultProject, setDefaultProject as apiSetDefaultProject } from '$lib/api';

const LOCAL_STORAGE_KEY = 'orc_current_project_id';

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

// Store for available projects
export const projects = writable<Project[]>([]);

// Store for currently selected project (initialized from localStorage)
export const currentProjectId = writable<string | null>(getStoredProjectId());

// Sync to localStorage whenever currentProjectId changes
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

		// Get stored project ID from localStorage
		const storedId = get(currentProjectId);

		// Check if stored ID is valid
		const storedIsValid = storedId && loaded.find(p => p.id === storedId);

		if (storedIsValid) {
			// localStorage selection is valid, use it
			return;
		}

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
		if (defaultId) {
			currentProjectId.set(defaultId);
		} else if (loaded.length > 0) {
			currentProjectId.set(loaded[0].id);
		} else {
			currentProjectId.set(null);
		}
	} catch (e) {
		const errorMsg = e instanceof Error ? e.message : 'Failed to load projects';
		projectsError.set(errorMsg);
		console.error('Failed to load projects:', e);
	} finally {
		projectsLoading.set(false);
	}
}

// Select a project (and persist to localStorage)
export function selectProject(id: string): void {
	currentProjectId.set(id);
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
