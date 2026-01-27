/**
 * TimelinePage wrapper component for the /timeline route.
 * Renders the TimelineView container component within the app layout.
 */

import { TimelineView } from '@/components/timeline/TimelineView';
import { useDocumentTitle } from '@/hooks';

/**
 * TimelinePage displays the activity timeline feed.
 * This is the page-level wrapper used in the router.
 */
export function TimelinePage() {
	useDocumentTitle('Timeline');
	return (
		<div className="timeline-page">
			<TimelineView />
		</div>
	);
}
