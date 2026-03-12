import { RecommendationKind } from '@/gen/orc/v1/recommendation_pb';

export function recommendationKindLabel(kind: RecommendationKind): string {
	switch (kind) {
		case RecommendationKind.CLEANUP:
			return 'Cleanup';
		case RecommendationKind.RISK:
			return 'Risk';
		case RecommendationKind.DECISION_REQUEST:
			return 'Decision request';
		default:
			return 'Follow-up';
	}
}
