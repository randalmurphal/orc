/**
 * Initiative store with URL + localStorage persistence.
 *
 * Priority order for initiative selection:
 * 1. URL param (?initiative=xxx) - enables shareable links
 * 2. localStorage - remembers user's last selection
 * 3. null - no filter (show all tasks)
 *
 * Browser history is managed so back/forward navigates between filters.
 */
import { writable, derived, get, type Readable } from 'svelte/store';
import type { Initiative } from '$lib/types';
import { listInitiatives, createInitiative as apiCreateInitiative, type CreateInitiativeRequest } from '$lib/api';

const LOCAL_STORAGE_KEY = 'orc_current_initiative_id';
const URL_PARAM_KEY = 'initiative';

// Special value for showing only unassigned tasks (no initiative)
export const UNASSIGNED_INITIATIVE = '__unassigned__';

// Helper to safely access localStorage
function getStoredInitiativeId(): string | null {
	if (typeof window === 'undefined') return null;
	try {
		return localStorage.getItem(LOCAL_STORAGE_KEY);
	} catch {
		return null;
	}
}

function setStoredInitiativeId(id: string | null): void {
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

// Helper to get initiative ID from URL query parameter
export function getUrlInitiativeId(): string | null {
	if (typeof window === 'undefined') return null;
	try {
		const params = new URLSearchParams(window.location.search);
		return params.get(URL_PARAM_KEY);
	} catch {
		return null;
	}
}

// Helper to update URL with initiative ID
export function setUrlInitiativeId(id: string | null, replace: boolean = false): void {
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
				window.history.replaceState({ ...history.state, initiativeId: id }, '', url.href);
			} else {
				window.history.pushState({ ...history.state, initiativeId: id }, '', url.href);
			}
		}
	} catch {
		// Ignore URL errors
	}
}

// Get initial initiative ID: URL param takes precedence over localStorage
function getInitialInitiativeId(): string | null {
	const urlId = getUrlInitiativeId();
	if (urlId) return urlId;
	return getStoredInitiativeId();
}

// Store for available initiatives
export const initiatives = writable<Initiative[]>([]);

// Store for currently selected initiative filter (null = show all)
export const currentInitiativeId = writable<string | null>(getInitialInitiativeId());

// Internal flag to prevent URL update during popstate handling
let isHandlingPopState = false;

// Sync to localStorage whenever currentInitiativeId changes
currentInitiativeId.subscribe(id => {
	setStoredInitiativeId(id);
});

// Loading and error states
export const initiativesLoading = writable<boolean>(false);
export const initiativesError = writable<string | null>(null);

// Derived store for current initiative
export const currentInitiative: Readable<Initiative | null> = derived(
	[initiatives, currentInitiativeId],
	([$initiatives, $id]) => {
		if (!$id) return null;
		return $initiatives.find(i => i.id === $id) ?? null;
	}
);

// Derived store for initiative progress (completed/total tasks)
export interface InitiativeProgress {
	id: string;
	completed: number;
	total: number;
}

export const initiativeProgress: Readable<Map<string, InitiativeProgress>> = derived(
	initiatives,
	($initiatives) => {
		const progressMap = new Map<string, InitiativeProgress>();
		for (const init of $initiatives) {
			const tasks = init.tasks || [];
			const completed = tasks.filter(t => t.status === 'completed').length;
			progressMap.set(init.id, {
				id: init.id,
				completed,
				total: tasks.length
			});
		}
		return progressMap;
	}
);

// Load initiatives from API
export async function loadInitiatives(): Promise<void> {
	initiativesLoading.set(true);
	initiativesError.set(null);

	try {
		const loaded = await listInitiatives();
		initiatives.set(loaded);

		// Validate current selection
		const currentId = get(currentInitiativeId);
		if (currentId) {
			const currentIsValid = loaded.find(i => i.id === currentId);
			if (!currentIsValid) {
				// Clear invalid selection
				currentInitiativeId.set(null);
				setUrlInitiativeId(null, true);
			}
		}
	} catch (e) {
		const errorMsg = e instanceof Error ? e.message : 'Failed to load initiatives';
		initiativesError.set(errorMsg);
		console.error('Failed to load initiatives:', e);
	} finally {
		initiativesLoading.set(false);
	}
}

// Select an initiative (persist to localStorage and URL)
export function selectInitiative(id: string | null): void {
	currentInitiativeId.set(id);
	if (!isHandlingPopState) {
		setUrlInitiativeId(id, false);
	}
}

// Handle browser back/forward navigation
export function handleInitiativePopState(event: PopStateEvent): void {
	const initiativeId = event.state?.initiativeId ?? getUrlInitiativeId();
	if (initiativeId !== get(currentInitiativeId)) {
		isHandlingPopState = true;
		currentInitiativeId.set(initiativeId);
		isHandlingPopState = false;
	}
}

// Create a new initiative
export async function createNewInitiative(req: CreateInitiativeRequest): Promise<Initiative> {
	const initiative = await apiCreateInitiative(req);
	// Add to store
	initiatives.update(current => [initiative, ...current]);
	return initiative;
}

// Update an initiative in the store (from WebSocket event)
export function updateInitiativeInStore(id: string, updates: Partial<Initiative>): void {
	initiatives.update(current => {
		const idx = current.findIndex(i => i.id === id);
		if (idx >= 0) {
			return current.map((i, index) => index === idx ? { ...i, ...updates } : i);
		}
		return current;
	});
}

// Remove an initiative from the store
export function removeInitiativeFromStore(id: string): void {
	initiatives.update(current => current.filter(i => i.id !== id));
	// Clear selection if it was the removed initiative
	if (get(currentInitiativeId) === id) {
		selectInitiative(null);
	}
}

// Add a new initiative to the store
export function addInitiativeToStore(initiative: Initiative): void {
	initiatives.update(current => {
		if (current.some(i => i.id === initiative.id)) {
			return current;
		}
		return [initiative, ...current];
	});
}
