/**
 * Dependency status filter store with URL + localStorage persistence.
 *
 * Priority order for dependency filter selection:
 * 1. URL param (?dependency_status=xxx) - enables shareable links
 * 2. localStorage - remembers user's last selection
 * 3. 'all' - no filter (show all tasks)
 *
 * Browser history is managed so back/forward navigates between filters.
 */
import { writable, get } from 'svelte/store';

const LOCAL_STORAGE_KEY = 'orc_dependency_status_filter';
const URL_PARAM_KEY = 'dependency_status';

// Filter options for the dropdown
export type DependencyStatusFilter = 'all' | 'blocked' | 'ready' | 'none';

export const DEPENDENCY_OPTIONS: { value: DependencyStatusFilter; label: string }[] = [
	{ value: 'all', label: 'All tasks' },
	{ value: 'ready', label: 'Ready' },
	{ value: 'blocked', label: 'Blocked' },
	{ value: 'none', label: 'No dependencies' }
];

// Helper to safely access localStorage
function getStoredDependencyStatus(): DependencyStatusFilter | null {
	if (typeof window === 'undefined') return null;
	try {
		const stored = localStorage.getItem(LOCAL_STORAGE_KEY);
		if (stored && ['all', 'blocked', 'ready', 'none'].includes(stored)) {
			return stored as DependencyStatusFilter;
		}
		return null;
	} catch {
		return null;
	}
}

function setStoredDependencyStatus(status: DependencyStatusFilter | null): void {
	if (typeof window === 'undefined') return;
	try {
		if (status && status !== 'all') {
			localStorage.setItem(LOCAL_STORAGE_KEY, status);
		} else {
			localStorage.removeItem(LOCAL_STORAGE_KEY);
		}
	} catch {
		// Ignore storage errors
	}
}

// Helper to get dependency status from URL query parameter
export function getUrlDependencyStatus(): DependencyStatusFilter | null {
	if (typeof window === 'undefined') return null;
	try {
		const params = new URLSearchParams(window.location.search);
		const value = params.get(URL_PARAM_KEY);
		if (value && ['blocked', 'ready', 'none'].includes(value)) {
			return value as DependencyStatusFilter;
		}
		return null;
	} catch {
		return null;
	}
}

// Helper to update URL with dependency status
export function setUrlDependencyStatus(status: DependencyStatusFilter | null, replace: boolean = false): void {
	if (typeof window === 'undefined') return;
	try {
		const url = new URL(window.location.href);
		if (status && status !== 'all') {
			url.searchParams.set(URL_PARAM_KEY, status);
		} else {
			url.searchParams.delete(URL_PARAM_KEY);
		}

		// Only update if URL actually changed
		if (url.href !== window.location.href) {
			if (replace) {
				window.history.replaceState({ ...history.state, dependencyStatus: status }, '', url.href);
			} else {
				window.history.pushState({ ...history.state, dependencyStatus: status }, '', url.href);
			}
		}
	} catch {
		// Ignore URL errors
	}
}

// Get initial dependency status: URL param takes precedence over localStorage
function getInitialDependencyStatus(): DependencyStatusFilter {
	const urlStatus = getUrlDependencyStatus();
	if (urlStatus) return urlStatus;
	const storedStatus = getStoredDependencyStatus();
	if (storedStatus) return storedStatus;
	return 'all';
}

// Store for currently selected dependency status filter ('all' = show all)
export const currentDependencyStatus = writable<DependencyStatusFilter>(getInitialDependencyStatus());

// Internal flag to prevent URL update during popstate handling
let isHandlingPopState = false;

// Sync to localStorage whenever currentDependencyStatus changes
currentDependencyStatus.subscribe(status => {
	setStoredDependencyStatus(status);
});

// Select a dependency status (persist to localStorage and URL)
export function selectDependencyStatus(status: DependencyStatusFilter | null): void {
	const newStatus = status ?? 'all';
	currentDependencyStatus.set(newStatus);
	if (!isHandlingPopState) {
		setUrlDependencyStatus(newStatus, false);
	}
}

// Handle browser back/forward navigation
export function handleDependencyPopState(event: PopStateEvent): void {
	const dependencyStatus = event.state?.dependencyStatus ?? getUrlDependencyStatus() ?? 'all';
	if (dependencyStatus !== get(currentDependencyStatus)) {
		isHandlingPopState = true;
		currentDependencyStatus.set(dependencyStatus);
		isHandlingPopState = false;
	}
}

// Initialize from URL on page load (call from layout or page component)
export function initDependencyStatusFromUrl(): void {
	const urlStatus = getUrlDependencyStatus();
	if (urlStatus) {
		isHandlingPopState = true;
		currentDependencyStatus.set(urlStatus);
		isHandlingPopState = false;
	}
}
