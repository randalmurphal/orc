/**
 * StatsPage wrapper component for the /stats route.
 * Renders the StatsView container component within the app layout.
 */

import { StatsView } from '@/components/stats';

/**
 * StatsPage displays the statistics overview.
 * This is the page-level wrapper used in the router.
 */
export function StatsPage() {
	return <StatsView />;
}
