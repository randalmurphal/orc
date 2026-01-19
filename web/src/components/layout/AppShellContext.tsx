/**
 * AppShellContext - Context provider for AppShell state management.
 *
 * Manages:
 * - Right panel open/collapsed state
 * - Right panel content
 * - localStorage persistence for collapsed state
 * - Keyboard shortcut for panel toggle (Shift+Alt+R)
 * - Responsive behavior (auto-collapse at breakpoints)
 */

import {
	createContext,
	useContext,
	useState,
	useCallback,
	useEffect,
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
	/** Set custom content for the right panel */
	setRightPanelContent: (content: ReactNode) => void;
	/** Current right panel content (null for default) */
	rightPanelContent: ReactNode;
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
// CONTEXT
// =============================================================================

const AppShellContext = createContext<AppShellContextValue | null>(null);

// =============================================================================
// PROVIDER
// =============================================================================

export function AppShellProvider({ children }: AppShellProviderProps) {
	// Initialize state from localStorage, but check viewport first
	const [isRightPanelOpen, setIsRightPanelOpen] = useState(() => {
		if (typeof window === 'undefined') return true;
		// Below tablet breakpoint, start collapsed
		if (window.innerWidth < TABLET_BREAKPOINT) return false;
		// Otherwise, use stored preference (inverted: stored is "collapsed")
		return !loadCollapsedState();
	});

	const [rightPanelContent, setRightPanelContent] = useState<ReactNode>(null);
	const [isMobileNavMode, setIsMobileNavMode] = useState(() => {
		if (typeof window === 'undefined') return false;
		return window.innerWidth < MOBILE_BREAKPOINT;
	});

	// Ref to track if initial render is done (for focus management)
	const initialRenderRef = useRef(true);
	const panelToggleRef = useRef<HTMLButtonElement | null>(null);

	// REF SYNCHRONIZATION PATTERN:
	// We use a ref alongside state to avoid re-registering the resize listener on every state change.
	// The resize handler (line 139-149) needs to read the current panel state to decide whether
	// to auto-collapse it, but including isRightPanelOpen in the effect's dependency array would
	// cause the listener to be re-registered on every toggle.
	//
	// Instead, the resize handler reads from isRightPanelOpenRef to get the latest panel state.
	// IMPORTANT: The effect below MUST keep this ref in sync with state. If this sync effect
	// is removed, the ref becomes stale and the resize handler will read outdated values.
	const isRightPanelOpenRef = useRef(isRightPanelOpen);

	// Keep ref in sync with state - REQUIRED for resize handler to work correctly
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

	// Handle responsive breakpoints
	useEffect(() => {
		if (typeof window === 'undefined') return;

		const handleResize = () => {
			const width = window.innerWidth;

			// Update mobile nav mode
			setIsMobileNavMode(width < MOBILE_BREAKPOINT);

			// Auto-collapse right panel below tablet breakpoint
			// (reads from isRightPanelOpenRef - see REF SYNCHRONIZATION PATTERN comment above)
			if (width < TABLET_BREAKPOINT && isRightPanelOpenRef.current) {
				setIsRightPanelOpen(false);
			}
		};

		window.addEventListener('resize', handleResize);
		return () => window.removeEventListener('resize', handleResize);
	}, []); // Empty deps - only run once

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

	const value: AppShellContextValue = {
		isRightPanelOpen,
		toggleRightPanel,
		setRightPanelContent,
		rightPanelContent,
		isMobileNavMode,
		panelToggleRef,
	};

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
