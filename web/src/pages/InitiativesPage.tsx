/**
 * InitiativesPage wrapper component for the /initiatives route.
 * Renders the InitiativesView container component within the app layout.
 */

import { InitiativesView } from '@/components/initiatives';

/**
 * InitiativesPage displays the initiatives overview.
 * This is the page-level wrapper used in the router.
 */
export function InitiativesPage() {
	return <InitiativesView />;
}
