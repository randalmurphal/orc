/**
 * React hooks and context for keyboard shortcuts
 *
 * Provides:
 * - ShortcutProvider: Context provider for shortcuts (wraps app root)
 * - useShortcuts: Hook to register/unregister shortcuts
 * - useShortcutContext: Hook to manage shortcut context
 * - useGlobalShortcuts: Hook for common global shortcuts
 * - useTaskListShortcuts: Hook for task list shortcuts
 */

import { createContext, useContext, useEffect, useCallback, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import {
	getShortcutManager,
	setupGlobalShortcuts,
	setupTaskListShortcuts,
	type Shortcut,
	type ShortcutSequence,
	type ShortcutContext as ShortcutCtx,
	type ShortcutInfo,
} from '@/lib/shortcuts';
import { useUIStore } from '@/stores';

// Context value type
interface ShortcutContextValue {
	registerShortcut: (shortcut: Shortcut) => () => void;
	registerSequence: (sequence: ShortcutSequence) => () => void;
	setContext: (context: ShortcutCtx) => void;
	getContext: () => ShortcutCtx;
	setEnabled: (enabled: boolean) => void;
	isEnabled: () => boolean;
	getShortcuts: () => ShortcutInfo[];
}

const ShortcutContext = createContext<ShortcutContextValue | null>(null);

interface ShortcutProviderProps {
	children: ReactNode;
}

/**
 * Provider component that initializes the shortcut manager
 * Should wrap the app at the root level
 */
export function ShortcutProvider({ children }: ShortcutProviderProps) {
	const manager = getShortcutManager();

	const contextValue: ShortcutContextValue = {
		registerShortcut: useCallback((shortcut: Shortcut) => manager.register(shortcut), [manager]),
		registerSequence: useCallback(
			(sequence: ShortcutSequence) => manager.registerSequence(sequence),
			[manager]
		),
		setContext: useCallback((context: ShortcutCtx) => manager.setContext(context), [manager]),
		getContext: useCallback(() => manager.getContext(), [manager]),
		setEnabled: useCallback((enabled: boolean) => manager.setEnabled(enabled), [manager]),
		isEnabled: useCallback(() => manager.isEnabled(), [manager]),
		getShortcuts: useCallback(() => manager.getShortcuts(), [manager]),
	};

	return <ShortcutContext.Provider value={contextValue}>{children}</ShortcutContext.Provider>;
}

/**
 * Hook to access shortcut functionality
 * Must be used within a ShortcutProvider
 */
export function useShortcuts(): ShortcutContextValue {
	const context = useContext(ShortcutContext);
	if (!context) {
		throw new Error('useShortcuts must be used within a ShortcutProvider');
	}
	return context;
}

/**
 * Hook to manage shortcut context (e.g., switching to 'tasks' context)
 */
export function useShortcutContext(context: ShortcutCtx) {
	const { setContext } = useShortcuts();

	useEffect(() => {
		setContext(context);
		return () => {
			setContext('global');
		};
	}, [context, setContext]);
}

/**
 * Hook options for global shortcuts
 */
interface UseGlobalShortcutsOptions {
	onCommandPalette?: () => void;
	onNewTask?: () => void;
	onProjectSwitcher?: () => void;
	onSearch?: () => void;
	onHelp?: () => void;
	onEscape?: () => void;
}

/**
 * Hook to set up global shortcuts with navigation and common actions
 *
 * @param options Custom handlers for specific shortcuts
 */
export function useGlobalShortcuts(options: UseGlobalShortcutsOptions = {}) {
	const navigate = useNavigate();
	const toggleSidebar = useUIStore((state) => state.toggleSidebar);

	useEffect(() => {
		const cleanup = setupGlobalShortcuts({
			onCommandPalette: options.onCommandPalette,
			onNewTask: options.onNewTask,
			onToggleSidebar: toggleSidebar,
			onProjectSwitcher: options.onProjectSwitcher,
			onSearch: options.onSearch,
			onHelp: options.onHelp,
			onEscape: options.onEscape,
			onGoDashboard: () => navigate('/dashboard'),
			onGoTasks: () => navigate('/'),
			onGoEnvironment: () => navigate('/environment/settings'),
			onGoPreferences: () => navigate('/preferences'),
			onGoPrompts: () => navigate('/environment/prompts'),
			onGoHooks: () => navigate('/environment/hooks'),
			onGoSkills: () => navigate('/environment/skills'),
		});

		return cleanup;
	}, [
		navigate,
		toggleSidebar,
		options.onCommandPalette,
		options.onNewTask,
		options.onProjectSwitcher,
		options.onSearch,
		options.onHelp,
		options.onEscape,
	]);
}

/**
 * Hook options for task list shortcuts
 */
interface UseTaskListShortcutsOptions {
	onNavDown?: () => void;
	onNavUp?: () => void;
	onOpen?: () => void;
	onRun?: () => void;
	onPause?: () => void;
	onDelete?: () => void;
}

/**
 * Hook to set up task list shortcuts (j/k navigation, enter, r, p, d)
 *
 * @param options Handlers for task list actions
 */
export function useTaskListShortcuts(options: UseTaskListShortcutsOptions) {
	useEffect(() => {
		const cleanup = setupTaskListShortcuts({
			onNavDown: options.onNavDown,
			onNavUp: options.onNavUp,
			onOpen: options.onOpen,
			onRun: options.onRun,
			onPause: options.onPause,
			onDelete: options.onDelete,
		});

		return cleanup;
	}, [options.onNavDown, options.onNavUp, options.onOpen, options.onRun, options.onPause, options.onDelete]);
}
