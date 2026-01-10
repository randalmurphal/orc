import { writable, derived, type Readable } from 'svelte/store';
import type { Project } from '$lib/types';
import { listProjects } from '$lib/api';

// Store for available projects
export const projects = writable<Project[]>([]);

// Store for currently selected project
export const currentProjectId = writable<string | null>(null);

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
	try {
		const loaded = await listProjects();
		projects.set(loaded);

		// Auto-select first project if none selected
		currentProjectId.update(current => {
			if (!current && loaded.length > 0) {
				return loaded[0].id;
			}
			// Ensure current selection is still valid
			if (current && !loaded.find(p => p.id === current)) {
				return loaded.length > 0 ? loaded[0].id : null;
			}
			return current;
		});
	} catch (e) {
		console.error('Failed to load projects:', e);
	}
}

// Select a project
export function selectProject(id: string): void {
	currentProjectId.set(id);
}
