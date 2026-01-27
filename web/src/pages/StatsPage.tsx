/**
 * StatsPage wrapper component for the /stats route.
 * Renders the StatsView container component within the app layout.
 */

import { StatsView } from '@/components/stats';
import { useDocumentTitle } from '@/hooks';

/**
 * StatsPage displays the statistics overview.
 * This is the page-level wrapper used in the router.
 */
export function StatsPage() {
	useDocumentTitle('Statistics');
	return <StatsView />;
}
