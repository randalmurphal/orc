import { recommendationClient } from '@/lib/client';
import type {
	AcceptRecommendationResponse,
	DiscussRecommendationResponse,
	ListRecommendationsResponse,
	RejectRecommendationResponse,
	RecommendationStatus,
} from '@/gen/orc/v1/recommendation_pb';

export async function listRecommendations(projectId: string, status?: RecommendationStatus): Promise<ListRecommendationsResponse> {
	return recommendationClient.listRecommendations({
		projectId,
		...(status !== undefined ? { status } : {}),
	});
}

export async function acceptRecommendation(
	projectId: string,
	recommendationId: string,
	decidedBy: string,
	decisionReason: string,
): Promise<AcceptRecommendationResponse> {
	return recommendationClient.acceptRecommendation({
		projectId,
		recommendationId,
		decidedBy,
		decisionReason,
	});
}

export async function rejectRecommendation(
	projectId: string,
	recommendationId: string,
	decidedBy: string,
	decisionReason: string,
): Promise<RejectRecommendationResponse> {
	return recommendationClient.rejectRecommendation({
		projectId,
		recommendationId,
		decidedBy,
		decisionReason,
	});
}

export async function discussRecommendation(
	projectId: string,
	recommendationId: string,
	decidedBy: string,
	decisionReason: string,
): Promise<DiscussRecommendationResponse> {
	return recommendationClient.discussRecommendation({
		projectId,
		recommendationId,
		decidedBy,
		decisionReason,
	});
}
