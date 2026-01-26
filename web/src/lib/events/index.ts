/**
 * Events module - Real-time event subscription via Connect RPC
 *
 * Replaces the WebSocket-based system with Connect server streaming.
 */

export {
	EventSubscription,
	type ConnectionStatus,
	type EventHandler,
	type StatusHandler,
} from './subscription';

export { handleEvent } from './handlers';
