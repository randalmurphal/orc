import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import type { ConnectionStatus } from '@/lib/events';
import type { PendingDecision } from '@/gen/orc/v1/decision_pb';

// UI-specific types (not proto)
export type ToastType = 'success' | 'error' | 'warning' | 'info';

export interface Toast {
	id: string;
	type: ToastType;
	message: string;
	title?: string;
	duration?: number;
	dismissible?: boolean;
}

const SIDEBAR_STORAGE_KEY = 'orc-sidebar-expanded';
const SIDEBAR_DEFAULT_KEY = 'orc-sidebar-default';

// localStorage helpers for sidebar
function getStoredSidebarExpanded(): boolean {
	if (typeof window === 'undefined') return true;
	try {
		const stored = localStorage.getItem(SIDEBAR_STORAGE_KEY);
		// If no stored state, check the user's default preference
		if (stored === null) {
			const defaultPref = localStorage.getItem(SIDEBAR_DEFAULT_KEY);
			return defaultPref === null ? true : defaultPref === 'expanded';
		}
		return stored === 'true';
	} catch {
		return true;
	}
}

function setStoredSidebarExpanded(expanded: boolean): void {
	if (typeof window === 'undefined') return;
	try {
		localStorage.setItem(SIDEBAR_STORAGE_KEY, String(expanded));
	} catch {
		// Ignore localStorage errors
	}
}

// Generate unique toast IDs
let toastIdCounter = 0;
function generateToastId(): string {
	return `toast-${Date.now()}-${++toastIdCounter}`;
}

// Default durations by toast type
const DEFAULT_DURATIONS: Record<ToastType, number> = {
	success: 5000,
	error: 8000,
	warning: 5000,
	info: 5000,
};

interface UIStore {
	// Sidebar state
	sidebarExpanded: boolean;

	// Mobile sidebar state (separate from desktop expanded state)
	mobileMenuOpen: boolean;

	// WebSocket connection status
	wsStatus: ConnectionStatus;

	// Toast queue
	toasts: Toast[];

	// Pending decisions (from decision_required events)
	pendingDecisions: PendingDecision[];

	// Sidebar actions
	toggleSidebar: () => void;
	setSidebarExpanded: (expanded: boolean) => void;

	// Mobile menu actions
	openMobileMenu: () => void;
	closeMobileMenu: () => void;
	toggleMobileMenu: () => void;

	// WebSocket actions
	setWsStatus: (status: ConnectionStatus) => void;

	// Toast actions
	addToast: (toast: Omit<Toast, 'id'> & { id?: string }) => string;
	dismissToast: (id: string) => void;
	clearToasts: () => void;

	// Decision actions
	addPendingDecision: (decision: PendingDecision) => void;
	removePendingDecision: (decisionId: string) => void;
	clearPendingDecisions: () => void;

	// Convenience toast methods
	toast: {
		success: (message: string, options?: Partial<Omit<Toast, 'id' | 'type' | 'message'>>) => string;
		error: (message: string, options?: Partial<Omit<Toast, 'id' | 'type' | 'message'>>) => string;
		warning: (message: string, options?: Partial<Omit<Toast, 'id' | 'type' | 'message'>>) => string;
		info: (message: string, options?: Partial<Omit<Toast, 'id' | 'type' | 'message'>>) => string;
	};

	// Reset
	reset: () => void;
}

const initialState = {
	sidebarExpanded: true, // Will be overridden by getStoredSidebarExpanded() on init
	mobileMenuOpen: false,
	wsStatus: 'disconnected' as ConnectionStatus,
	toasts: [] as Toast[],
	pendingDecisions: [] as PendingDecision[],
};

export const useUIStore = create<UIStore>()(
	subscribeWithSelector((set, get) => {
		// Auto-dismiss toast helper
		const scheduleAutoDismiss = (id: string, duration: number) => {
			if (duration > 0) {
				setTimeout(() => {
					get().dismissToast(id);
				}, duration);
			}
		};

		// Core addToast implementation
		const addToastImpl = (toast: Omit<Toast, 'id'> & { id?: string }): string => {
			const id = toast.id ?? generateToastId();
			const duration = toast.duration ?? DEFAULT_DURATIONS[toast.type];
			const dismissible = toast.dismissible ?? true;

			const newToast: Toast = {
				...toast,
				id,
				duration,
				dismissible,
			};

			set((state: UIStore) => ({
				toasts: [...state.toasts, newToast],
			}));

			// Schedule auto-dismiss if duration is set
			if (duration && duration > 0) {
				scheduleAutoDismiss(id, duration);
			}

			return id;
		};

		return {
			...initialState,
			sidebarExpanded: getStoredSidebarExpanded(),

			// Sidebar actions
			toggleSidebar: () =>
				set((state: UIStore) => {
					const newExpanded = !state.sidebarExpanded;
					setStoredSidebarExpanded(newExpanded);
					return { sidebarExpanded: newExpanded };
				}),

			setSidebarExpanded: (expanded: boolean) => {
				setStoredSidebarExpanded(expanded);
				set({ sidebarExpanded: expanded });
			},

			// Mobile menu actions
			openMobileMenu: () => set({ mobileMenuOpen: true }),
			closeMobileMenu: () => set({ mobileMenuOpen: false }),
			toggleMobileMenu: () =>
				set((state: UIStore) => ({ mobileMenuOpen: !state.mobileMenuOpen })),

			// WebSocket actions
			setWsStatus: (status: ConnectionStatus) => set({ wsStatus: status }),

			// Toast actions
			addToast: addToastImpl,

			dismissToast: (id: string) =>
				set((state: UIStore) => ({
					toasts: state.toasts.filter((t: Toast) => t.id !== id),
				})),

			clearToasts: () => set({ toasts: [] }),

			// Decision actions
			addPendingDecision: (decision: PendingDecision) =>
				set((state: UIStore) => {
					// Avoid duplicates
					if (state.pendingDecisions.some((d) => d.id === decision.id)) {
						return state;
					}
					return { pendingDecisions: [...state.pendingDecisions, decision] };
				}),

			removePendingDecision: (decisionId: string) =>
				set((state: UIStore) => ({
					pendingDecisions: state.pendingDecisions.filter((d) => d.id !== decisionId),
				})),

			clearPendingDecisions: () => set({ pendingDecisions: [] }),

			// Convenience toast methods
			toast: {
				success: (message: string, options?: Partial<Omit<Toast, 'id' | 'type' | 'message'>>) =>
					addToastImpl({ type: 'success', message, ...options }),
				error: (message: string, options?: Partial<Omit<Toast, 'id' | 'type' | 'message'>>) =>
					addToastImpl({ type: 'error', message, ...options }),
				warning: (message: string, options?: Partial<Omit<Toast, 'id' | 'type' | 'message'>>) =>
					addToastImpl({ type: 'warning', message, ...options }),
				info: (message: string, options?: Partial<Omit<Toast, 'id' | 'type' | 'message'>>) =>
					addToastImpl({ type: 'info', message, ...options }),
			},

			reset: () => set(initialState),
		};
	})
);

// Subscribe to sidebar changes to persist
useUIStore.subscribe(
	(state: UIStore) => state.sidebarExpanded,
	(expanded: boolean) => {
		setStoredSidebarExpanded(expanded);
	}
);

// Selector hooks
export const useSidebarExpanded = () => useUIStore((state: UIStore) => state.sidebarExpanded);
export const useMobileMenuOpen = () => useUIStore((state: UIStore) => state.mobileMenuOpen);
export const useWsStatus = () => useUIStore((state: UIStore) => state.wsStatus);
export const useToasts = () => useUIStore((state: UIStore) => state.toasts);
export const usePendingDecisions = () => useUIStore((state: UIStore) => state.pendingDecisions);

// Direct access to toast methods (for use outside React components)
export const toast = {
	success: (message: string, options?: Partial<Omit<Toast, 'id' | 'type' | 'message'>>) =>
		useUIStore.getState().toast.success(message, options),
	error: (message: string, options?: Partial<Omit<Toast, 'id' | 'type' | 'message'>>) =>
		useUIStore.getState().toast.error(message, options),
	warning: (message: string, options?: Partial<Omit<Toast, 'id' | 'type' | 'message'>>) =>
		useUIStore.getState().toast.warning(message, options),
	info: (message: string, options?: Partial<Omit<Toast, 'id' | 'type' | 'message'>>) =>
		useUIStore.getState().toast.info(message, options),
	dismiss: (id: string) => useUIStore.getState().dismissToast(id),
	clear: () => useUIStore.getState().clearToasts(),
};
