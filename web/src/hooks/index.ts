// WebSocket hooks
export {
	WebSocketProvider,
	useWebSocket,
	useTaskSubscription,
	useConnectionStatus,
	GLOBAL_TASK_ID,
	type TranscriptLine,
} from './useWebSocket';

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

// Re-export commonly used store hooks for convenience
export { useCurrentProject } from '@/stores';
