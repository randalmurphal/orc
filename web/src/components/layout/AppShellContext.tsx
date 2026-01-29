/* eslint-disable react-refresh/only-export-components */
/**
 * AppShellContext - Context provider for AppShell state management.
 *
 * Manages:
 * - Right panel open/collapsed state
 * - localStorage persistence for collapsed state
 * - Keyboard shortcut for panel toggle (Shift+Alt+R)
 * - Responsive behavior (auto-collapse at breakpoints)
 *
 * NOTE: Right panel CONTENT is NOT managed here. Components like
 * BoardCommandPanel read directly from stores — no JSX-through-context.
 */

import {
	createContext,
	useContext,
	useState,
	useCallback,
	useEffect,
	useMemo,
	useRef,
	type ReactNode,
} from 'react';

// =============================================================================
// CONSTANTS
// =============================================================================

const STORAGE_KEY = 'orc-right-panel-collapsed';
const TABLET_BREAKPOINT = 1024;
const MOBILE_BREAKPOINT = 768;

// =============================================================================
// TYPES
// =============================================================================

export interface AppShellContextValue {
	/** Whether the right panel is open */
	isRightPanelOpen: boolean;
	/** Toggle the right panel open/closed */
	toggleRightPanel: () => void;
	/** Whether mobile nav is in hamburger mode */
	isMobileNavMode: boolean;
	/** Ref to attach to the panel toggle button for focus management */
	panelToggleRef: React.RefObject<HTMLButtonElement | null>;
}

interface AppShellProviderProps {
	children: ReactNode;
}

// =============================================================================
// STORAGE HELPERS
// =============================================================================

function loadCollapsedState(): boolean {
	if (typeof window === 'undefined') return false;
	try {
		const stored = localStorage.getItem(STORAGE_KEY);
		return stored === 'true';
	} catch {
		return false;
	}
}

function saveCollapsedState(collapsed: boolean): void {
	if (typeof window === 'undefined') return;
	try {
		localStorage.setItem(STORAGE_KEY, String(collapsed));
	} catch {
		// Ignore localStorage errors (e.g., private browsing)
	}
}

// =============================================================================
// INITIALIZATION HELPERS
// =============================================================================

/**
 * Determines the initial right panel state based on environment and preferences.
 *
 * Priority order:
 * 1. SSR environment → default to open (will recompute on hydration)
 * 2. Below tablet breakpoint → always start collapsed (responsive UX)
 * 3. Otherwise → use persisted localStorage preference
 */
function getInitialPanelState(): boolean {
	// SSR: default to open, will recompute on client hydration
	if (typeof window === 'undefined') return true;

	// Mobile/tablet: always start collapsed regardless of preference
	if (window.innerWidth < TABLET_BREAKPOINT) return false;

	// Desktop: use stored preference (stored value is "collapsed", so invert)
	return !loadCollapsedState();
}

/**
 * Determines if mobile navigation mode should be active.
 */
function getInitialMobileNavMode(): boolean {
	if (typeof window === 'undefined') return false;
	return window.innerWidth < MOBILE_BREAKPOINT;
}

// =============================================================================
// CONTEXT
// =============================================================================

const AppShellContext = createContext<AppShellContextValue | null>(null);

// =============================================================================
// PROVIDER
// =============================================================================

export function AppShellProvider({ children }: AppShellProviderProps) {
	// State initialization uses extracted helpers for clarity and testability
	const [isRightPanelOpen, setIsRightPanelOpen] = useState(getInitialPanelState);
	const [isMobileNavMode, setIsMobileNavMode] = useState(getInitialMobileNavMode);

	// Ref to track if initial render is done (for focus management)
	const initialRenderRef = useRef(true);
	const panelToggleRef = useRef<HTMLButtonElement | null>(null);

	// Ref to allow resize handler to read current panel state without
	// re-registering the event listener on every toggle
	const isRightPanelOpenRef = useRef(isRightPanelOpen);
	useEffect(() => {
		isRightPanelOpenRef.current = isRightPanelOpen;
	}, [isRightPanelOpen]);

	// Toggle panel and persist state
	const toggleRightPanel = useCallback(() => {
		setIsRightPanelOpen((prev) => {
			const next = !prev;
			saveCollapsedState(!next); // Save collapsed (inverted)
			return next;
		});
	}, []);

	// Handle keyboard shortcut (Shift+Alt+R)
	useEffect(() => {
		const handleKeyDown = (e: KeyboardEvent) => {
			if (e.shiftKey && e.altKey && e.key.toLowerCase() === 'r') {
				e.preventDefault();
				toggleRightPanel();
			}
		};

		document.addEventListener('keydown', handleKeyDown);
		return () => document.removeEventListener('keydown', handleKeyDown);
	}, [toggleRightPanel]);

	// Handle responsive breakpoints with RAF-throttled resize handler
	useEffect(() => {
		if (typeof window === 'undefined') return;

		let rafId: number | null = null;

		const handleResize = () => {
			// Skip if a frame is already pending (RAF throttling)
			if (rafId !== null) return;

			rafId = requestAnimationFrame(() => {
				const width = window.innerWidth;

				// Update mobile nav mode
				setIsMobileNavMode(width < MOBILE_BREAKPOINT);

				// Auto-collapse right panel below tablet breakpoint
				if (width < TABLET_BREAKPOINT && isRightPanelOpenRef.current) {
					setIsRightPanelOpen(false);
				}

				rafId = null;
			});
		};

		window.addEventListener('resize', handleResize);
		return () => {
			window.removeEventListener('resize', handleResize);
			if (rafId !== null) cancelAnimationFrame(rafId);
		};
	}, []);

	// Focus management when panel opens/closes
	// Skip initial render: we only want to manage focus when the user actively toggles the panel,
	// not when the component first mounts (which would steal focus from wherever it was)
	useEffect(() => {
		if (initialRenderRef.current) {
			initialRenderRef.current = false;
			return;
		}

		// When panel closes, focus should return to toggle button
		if (!isRightPanelOpen && panelToggleRef.current) {
			panelToggleRef.current.focus();
		}
	}, [isRightPanelOpen]);

	// Memoize context value to prevent unnecessary consumer re-renders.
	const value = useMemo<AppShellContextValue>(
		() => ({
			isRightPanelOpen,
			toggleRightPanel,
			isMobileNavMode,
			panelToggleRef,
		}),
		[isRightPanelOpen, toggleRightPanel, isMobileNavMode],
	);

	return (
		<AppShellContext.Provider value={value}>
			{children}
		</AppShellContext.Provider>
	);
}

// =============================================================================
// HOOK
// =============================================================================

export function useAppShell(): AppShellContextValue {
	const context = useContext(AppShellContext);
	if (!context) {
		throw new Error('useAppShell must be used within an AppShellProvider');
	}
	return context;
}

// Export context for testing purposes
export { AppShellContext };
