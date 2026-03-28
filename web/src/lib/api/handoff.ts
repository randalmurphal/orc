import { handoffClient } from '@/lib/client';
import type {
	GenerateHandoffResponse,
	HandoffSourceType,
	HandoffTarget,
} from '@/gen/orc/v1/handoff_pb';

export async function generateHandoff(
	projectId: string,
	sourceType: HandoffSourceType,
	sourceId: string,
	target: HandoffTarget,
): Promise<GenerateHandoffResponse> {
	return handoffClient.generateHandoff({
		projectId,
		sourceType,
		sourceId,
		target,
	});
}
