// Event streaming hooks (Connect RPC)
export {
	EventProvider,
	useEvents,
	useTaskSubscription,
	useConnectionStatus,
	GLOBAL_TASK_ID,
	type TranscriptLine,
} from './useEvents';

// Shortcut hooks
export {
	ShortcutProvider,
	useShortcuts,
	useShortcutContext,
	useGlobalShortcuts,
	useTaskListShortcuts,
} from './useShortcuts';

// Document title hook
export { useDocumentTitle } from './useDocumentTitle';

// Accessibility hooks
export { useClickKeyboard } from './useClickKeyboard';

// Re-export commonly used store hooks for convenience
export { useCurrentProject } from '@/stores';
