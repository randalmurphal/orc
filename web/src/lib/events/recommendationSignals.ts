export type RecommendationSignalType = 'created' | 'decided';

export interface RecommendationSignal {
	projectId: string;
	recommendationId: string;
	type: RecommendationSignalType;
}

const recommendationSignalTarget = new EventTarget();
const recommendationSignalEventType = 'orc:recommendation';

export function emitRecommendationSignal(signal: RecommendationSignal): void {
	recommendationSignalTarget.dispatchEvent(
		new CustomEvent<RecommendationSignal>(recommendationSignalEventType, {
			detail: signal,
		}),
	);
}

export function onRecommendationSignal(handler: (signal: RecommendationSignal) => void): () => void {
	const listener = (event: Event) => {
		const customEvent = event as CustomEvent<RecommendationSignal>;
		handler(customEvent.detail);
	};

	recommendationSignalTarget.addEventListener(recommendationSignalEventType, listener);
	return () => recommendationSignalTarget.removeEventListener(recommendationSignalEventType, listener);
}
