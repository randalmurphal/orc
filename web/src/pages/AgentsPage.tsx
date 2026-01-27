/**
 * AgentsPage wrapper component for the /agents route.
 * Renders the AgentsView container component within the app layout.
 */

import { AgentsView } from '@/components/agents';
import { useDocumentTitle } from '@/hooks';

/**
 * AgentsPage displays the agents configuration overview.
 * This is the page-level wrapper used in the router.
 */
export function AgentsPage() {
	useDocumentTitle('Agents');
	return <AgentsView />;
}
