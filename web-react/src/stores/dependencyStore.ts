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
import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';

const LOCAL_STORAGE_KEY = 'orc_dependency_status_filter';
const URL_PARAM_KEY = 'dependency_status';

// Filter options for the dropdown
export type DependencyStatusFilter = 'all' | 'blocked' | 'ready' | 'none';

export const DEPENDENCY_OPTIONS: { value: DependencyStatusFilter; label: string }[] = [
	{ value: 'all', label: 'All tasks' },
	{ value: 'ready', label: 'Ready' },
	{ value: 'blocked', label: 'Blocked' },
	{ value: 'none', label: 'No dependencies' },
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
function getUrlDependencyStatus(): DependencyStatusFilter | null {
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
function setUrlDependencyStatus(
	status: DependencyStatusFilter | null,
	replace: boolean = false
): void {
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
				window.history.replaceState(
					{ ...history.state, dependencyStatus: status },
					'',
					url.href
				);
			} else {
				window.history.pushState(
					{ ...history.state, dependencyStatus: status },
					'',
					url.href
				);
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

interface DependencyStore {
	// State
	currentDependencyStatus: DependencyStatusFilter;

	// Flag to prevent recursive URL updates during popstate
	_isHandlingPopState: boolean;

	// Actions
	selectDependencyStatus: (status: DependencyStatusFilter | null) => void;
	handlePopState: (event: PopStateEvent) => void;
	initializeFromUrl: () => void;
}

const initialState = {
	currentDependencyStatus: 'all' as DependencyStatusFilter,
	_isHandlingPopState: false,
};

export const useDependencyStore = create<DependencyStore>()(
	subscribeWithSelector((set, get) => ({
		...initialState,

		selectDependencyStatus: (status) => {
			const newStatus = status ?? 'all';
			const { _isHandlingPopState } = get();
			set({ currentDependencyStatus: newStatus });
			setStoredDependencyStatus(newStatus);

			// Only update URL if not handling popstate (avoid double push)
			if (!_isHandlingPopState) {
				setUrlDependencyStatus(newStatus, false);
			}
		},

		handlePopState: (event: PopStateEvent) => {
			const dependencyStatus =
				(event.state?.dependencyStatus as DependencyStatusFilter | undefined) ??
				getUrlDependencyStatus() ??
				'all';
			const { currentDependencyStatus } = get();

			if (dependencyStatus !== currentDependencyStatus) {
				set({ _isHandlingPopState: true });
				set({ currentDependencyStatus: dependencyStatus });
				setStoredDependencyStatus(dependencyStatus);
				set({ _isHandlingPopState: false });
			}
		},

		initializeFromUrl: () => {
			const initialStatus = getInitialDependencyStatus();
			set({ currentDependencyStatus: initialStatus });
			setStoredDependencyStatus(initialStatus);
			// Replace URL to ensure it's in sync
			if (initialStatus !== 'all') {
				setUrlDependencyStatus(initialStatus, true);
			}
		},
	}))
);

// Sync to localStorage on changes
useDependencyStore.subscribe(
	(state) => state.currentDependencyStatus,
	(status) => {
		setStoredDependencyStatus(status);
	}
);

// Selector hooks
export const useCurrentDependencyStatus = () =>
	useDependencyStore((state) => state.currentDependencyStatus);
