// WebSocket hooks
export {
	WebSocketProvider,
	useWebSocket,
	useTaskSubscription,
	useConnectionStatus,
	GLOBAL_TASK_ID,
	type TranscriptLine,
} from './useWebSocket';

// Re-export commonly used store hooks for convenience
export { useCurrentProject } from '@/stores';
