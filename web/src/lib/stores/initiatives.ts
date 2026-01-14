import { writable, derived, get } from 'svelte/store';
import type { Initiative } from '$lib/types';
import { listInitiatives } from '$lib/api';

// Store for all initiatives (keyed by ID for fast lookup)
export const initiatives = writable<Map<string, Initiative>>(new Map());

// Loading state
export const initiativesLoading = writable<boolean>(false);
export const initiativesError = writable<string | null>(null);

// Track if we've loaded initiatives at least once
let hasLoaded = false;

// Derived store for initiatives as array (for listing)
export const initiativesList = derived(
	initiatives,
	($initiatives) => Array.from($initiatives.values())
);

// Load initiatives from API (with caching - only loads once per session)
export async function loadInitiatives(force = false): Promise<void> {
	// Skip if already loaded unless forced
	if (hasLoaded && !force) {
		return;
	}

	initiativesLoading.set(true);
	initiativesError.set(null);

	try {
		const loaded = await listInitiatives();
		const initiativeMap = new Map<string, Initiative>();
		for (const init of loaded) {
			initiativeMap.set(init.id, init);
		}
		initiatives.set(initiativeMap);
		hasLoaded = true;
	} catch (e) {
		const errorMsg = e instanceof Error ? e.message : 'Failed to load initiatives';
		initiativesError.set(errorMsg);
		console.error('Failed to load initiatives:', e);
	} finally {
		initiativesLoading.set(false);
	}
}

// Get initiative by ID from store
export function getInitiativeFromStore(id: string): Initiative | undefined {
	return get(initiatives).get(id);
}

// Get initiative title by ID (returns ID if not found, for loading states)
export function getInitiativeTitle(id: string): string {
	const init = getInitiativeFromStore(id);
	return init?.title ?? id;
}

// Truncate initiative title for badge display
export function truncateInitiativeTitle(title: string, maxLength = 12): string {
	if (title.length <= maxLength) {
		return title;
	}
	return title.slice(0, maxLength - 1) + '\u2026'; // Unicode ellipsis
}

// Get display title for badge (truncated with full title available)
export function getInitiativeBadgeTitle(id: string, maxLength = 12): { display: string; full: string } {
	const init = getInitiativeFromStore(id);
	const full = init?.title ?? id;
	const display = truncateInitiativeTitle(full, maxLength);
	return { display, full };
}

// Update a single initiative in the store
export function updateInitiative(id: string, updates: Partial<Initiative>): void {
	initiatives.update(current => {
		const existing = current.get(id);
		if (existing) {
			const updated = { ...existing, ...updates };
			const newMap = new Map(current);
			newMap.set(id, updated);
			return newMap;
		}
		return current;
	});
}

// Add a new initiative to the store
export function addInitiative(initiative: Initiative): void {
	initiatives.update(current => {
		const newMap = new Map(current);
		newMap.set(initiative.id, initiative);
		return newMap;
	});
}

// Remove an initiative from the store
export function removeInitiative(id: string): void {
	initiatives.update(current => {
		const newMap = new Map(current);
		newMap.delete(id);
		return newMap;
	});
}

// Reset the store (useful for testing or project switch)
export function resetInitiatives(): void {
	initiatives.set(new Map());
	hasLoaded = false;
	initiativesError.set(null);
}
