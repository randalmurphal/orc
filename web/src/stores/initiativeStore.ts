import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import { useShallow } from 'zustand/react/shallow';
import type { Initiative, InitiativeProgress, Task } from '@/lib/types';

// Special value for filtering tasks without an initiative
export const UNASSIGNED_INITIATIVE = '__unassigned__';

const STORAGE_KEY = 'orc_current_initiative_id';
const URL_PARAM = 'initiative';

// URL helpers
function getUrlInitiativeId(): string | null {
	if (typeof window === 'undefined') return null;
	const params = new URLSearchParams(window.location.search);
	return params.get(URL_PARAM);
}

function setUrlInitiativeId(id: string | null, replace = false): void {
	if (typeof window === 'undefined') return;

	const url = new URL(window.location.href);
	if (id) {
		url.searchParams.set(URL_PARAM, id);
	} else {
		url.searchParams.delete(URL_PARAM);
	}

	if (replace) {
		window.history.replaceState({ initiativeId: id }, '', url.toString());
	} else {
		window.history.pushState({ initiativeId: id }, '', url.toString());
	}
}

// localStorage helpers
function getStoredInitiativeId(): string | null {
	if (typeof window === 'undefined') return null;
	try {
		return localStorage.getItem(STORAGE_KEY);
	} catch {
		return null;
	}
}

function setStoredInitiativeId(id: string | null): void {
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

interface InitiativeStore {
	// State
	initiatives: Map<string, Initiative>;
	currentInitiativeId: string | null; // Filter selection (null = all, UNASSIGNED_INITIATIVE = no initiative)
	loading: boolean;
	error: string | null;
	hasLoaded: boolean; // Caches loaded state (load once per session)

	// Flag to prevent recursive URL updates during popstate
	_isHandlingPopState: boolean;

	// Derived
	getInitiativesList: () => Initiative[];
	getCurrentInitiative: () => Initiative | undefined;
	getInitiativeProgress: (tasks: Task[]) => Map<string, InitiativeProgress>;
	getInitiative: (id: string) => Initiative | undefined;
	getInitiativeTitle: (id: string) => string;

	// Actions
	setInitiatives: (initiatives: Initiative[]) => void;
	addInitiative: (initiative: Initiative) => void;
	updateInitiative: (id: string, updates: Partial<Initiative>) => void;
	removeInitiative: (id: string) => void;
	selectInitiative: (id: string | null) => void;
	handlePopState: (event: PopStateEvent) => void;
	initializeFromUrl: () => void;
	setLoading: (loading: boolean) => void;
	setError: (error: string | null) => void;
	setHasLoaded: (loaded: boolean) => void;
	reset: () => void;
}

// Get initial initiative filter: URL param > localStorage
function getInitialInitiativeId(): string | null {
	const urlId = getUrlInitiativeId();
	if (urlId) return urlId;
	return getStoredInitiativeId();
}

const initialState = {
	initiatives: new Map<string, Initiative>(),
	currentInitiativeId: null as string | null,
	loading: false,
	error: null as string | null,
	hasLoaded: false,
	_isHandlingPopState: false,
};

export const useInitiativeStore = create<InitiativeStore>()(
	subscribeWithSelector((set, get) => ({
		...initialState,

		// Derived
		getInitiativesList: () => {
			const { initiatives } = get();
			return Array.from(initiatives.values());
		},

		getCurrentInitiative: () => {
			const { initiatives, currentInitiativeId } = get();
			if (!currentInitiativeId || currentInitiativeId === UNASSIGNED_INITIATIVE) {
				return undefined;
			}
			return initiatives.get(currentInitiativeId);
		},

		getInitiativeProgress: (tasks: Task[]) => {
			const progress = new Map<string, InitiativeProgress>();

			// Count tasks per initiative
			for (const task of tasks) {
				if (!task.initiative_id) continue;

				const existing = progress.get(task.initiative_id);
				const isCompleted = task.status === 'completed';

				if (existing) {
					existing.total++;
					if (isCompleted) existing.completed++;
				} else {
					progress.set(task.initiative_id, {
						id: task.initiative_id,
						completed: isCompleted ? 1 : 0,
						total: 1,
					});
				}
			}

			return progress;
		},

		getInitiative: (id) => get().initiatives.get(id),

		getInitiativeTitle: (id) => {
			const initiative = get().initiatives.get(id);
			return initiative?.title ?? id;
		},

		// Actions
		setInitiatives: (initiativesList) => {
			const initiatives = new Map<string, Initiative>();
			for (const initiative of initiativesList) {
				initiatives.set(initiative.id, initiative);
			}

			// Validate current selection (but UNASSIGNED_INITIATIVE is always valid)
			const { currentInitiativeId } = get();
			if (
				currentInitiativeId &&
				currentInitiativeId !== UNASSIGNED_INITIATIVE &&
				!initiatives.has(currentInitiativeId)
			) {
				// Current filter not in list, clear it
				set({ initiatives, currentInitiativeId: null, hasLoaded: true });
				setStoredInitiativeId(null);
				setUrlInitiativeId(null, true);
			} else {
				set({ initiatives, hasLoaded: true });
			}
		},

		addInitiative: (initiative) =>
			set((state) => {
				const newInitiatives = new Map(state.initiatives);
				newInitiatives.set(initiative.id, initiative);
				return { initiatives: newInitiatives };
			}),

		updateInitiative: (id, updates) =>
			set((state) => {
				const existing = state.initiatives.get(id);
				if (!existing) return state;

				const newInitiatives = new Map(state.initiatives);
				newInitiatives.set(id, { ...existing, ...updates });
				return { initiatives: newInitiatives };
			}),

		removeInitiative: (id) =>
			set((state) => {
				const newInitiatives = new Map(state.initiatives);
				newInitiatives.delete(id);

				// Clear selection if removed initiative was selected
				if (state.currentInitiativeId === id) {
					setStoredInitiativeId(null);
					setUrlInitiativeId(null, true);
					return { initiatives: newInitiatives, currentInitiativeId: null };
				}

				return { initiatives: newInitiatives };
			}),

		selectInitiative: (id) => {
			const { _isHandlingPopState } = get();
			set({ currentInitiativeId: id });
			setStoredInitiativeId(id);

			// Only update URL if not handling popstate (avoid double push)
			if (!_isHandlingPopState) {
				setUrlInitiativeId(id, false);
			}
		},

		handlePopState: (event: PopStateEvent) => {
			const initiativeId =
				(event.state?.initiativeId as string | undefined) ?? getUrlInitiativeId();
			const { currentInitiativeId } = get();

			if (initiativeId !== currentInitiativeId) {
				set({ _isHandlingPopState: true });
				set({ currentInitiativeId: initiativeId });
				setStoredInitiativeId(initiativeId);
				set({ _isHandlingPopState: false });
			}
		},

		initializeFromUrl: () => {
			const initialId = getInitialInitiativeId();
			if (initialId) {
				set({ currentInitiativeId: initialId });
				setStoredInitiativeId(initialId);
				// Replace URL to ensure it's in sync
				setUrlInitiativeId(initialId, true);
			}
		},

		setLoading: (loading) => set({ loading }),
		setError: (error) => set({ error }),
		setHasLoaded: (loaded) => set({ hasLoaded: loaded }),
		reset: () => set(initialState),
	}))
);

// Subscribe to store changes to sync localStorage
useInitiativeStore.subscribe(
	(state) => state.currentInitiativeId,
	(currentInitiativeId) => {
		setStoredInitiativeId(currentInitiativeId);
	}
);

// Selector hooks
// Using useShallow to prevent infinite loops with derived array values
export const useInitiatives = () =>
	useInitiativeStore(useShallow((state) => Array.from(state.initiatives.values())));
export const useCurrentInitiative = () =>
	useInitiativeStore((state) => {
		const { initiatives, currentInitiativeId } = state;
		if (!currentInitiativeId || currentInitiativeId === UNASSIGNED_INITIATIVE) {
			return undefined;
		}
		return initiatives.get(currentInitiativeId);
	});
export const useCurrentInitiativeId = () =>
	useInitiativeStore((state) => state.currentInitiativeId);

// Helper to truncate initiative title for badges
export function truncateInitiativeTitle(title: string, maxLength: number = 20): string {
	if (title.length <= maxLength) return title;
	return title.slice(0, maxLength - 1) + '…';
}

// Badge display format options
export type InitiativeBadgeFormat = 'id-only' | 'id-with-title' | 'title-only';

/**
 * Helper to get badge title with full title for tooltip.
 *
 * Display format:
 * - 'id-only': Shows just "INIT-012" (default, most compact)
 * - 'id-with-title': Shows "INIT-012: Systems..." (ID + truncated title)
 * - 'title-only': Shows "Systems Reliability..." (legacy behavior)
 *
 * Full title is always available for tooltip display.
 */
export function getInitiativeBadgeTitle(
	id: string,
	format: InitiativeBadgeFormat = 'id-only',
	maxLength: number = 20
): { display: string; full: string; id: string } {
	const title = useInitiativeStore.getState().getInitiativeTitle(id);

	let display: string;
	switch (format) {
		case 'id-only':
			display = id;
			break;
		case 'id-with-title': {
			// Show "INIT-012: Title..." with truncation
			const prefixLength = id.length + 2; // id + ": "
			const titleMaxLength = Math.max(maxLength - prefixLength, 8);
			const truncatedTitle = truncateInitiativeTitle(title, titleMaxLength);
			display = `${id}: ${truncatedTitle}`;
			break;
		}
		case 'title-only':
			display = truncateInitiativeTitle(title, maxLength);
			break;
	}

	return {
		display,
		full: `${id}: ${title}`,
		id,
	};
}
