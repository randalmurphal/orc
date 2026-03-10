import { RecommendationInbox } from '@/components/recommendations/RecommendationInbox';
import { useDocumentTitle } from '@/hooks';

export function RecommendationsPage() {
	useDocumentTitle('Recommendations');
	return <RecommendationInbox />;
}
