export type AttentionDashboardSignalType =
	| 'task-updated'
	| 'decision-required'
	| 'decision-resolved';

export interface AttentionDashboardSignal {
	projectId: string;
	taskId?: string;
	type: AttentionDashboardSignalType;
}

const attentionDashboardSignalTarget = new EventTarget();
const attentionDashboardSignalEventType = 'orc:attention-dashboard';

export function emitAttentionDashboardSignal(signal: AttentionDashboardSignal): void {
	attentionDashboardSignalTarget.dispatchEvent(
		new CustomEvent<AttentionDashboardSignal>(attentionDashboardSignalEventType, {
			detail: signal,
		}),
	);
}

export function onAttentionDashboardSignal(
	handler: (signal: AttentionDashboardSignal) => void,
): () => void {
	const listener = (event: Event) => {
		const customEvent = event as CustomEvent<AttentionDashboardSignal>;
		handler(customEvent.detail);
	};

	attentionDashboardSignalTarget.addEventListener(attentionDashboardSignalEventType, listener);
	return () =>
		attentionDashboardSignalTarget.removeEventListener(attentionDashboardSignalEventType, listener);
}
