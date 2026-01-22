/**
 * WorkflowsPage wrapper component for the /workflows route.
 * Renders the WorkflowsView container component within the app layout.
 */

import { WorkflowsView } from '@/components/workflows';

/**
 * WorkflowsPage displays the workflows and phase templates configuration.
 * This is the page-level wrapper used in the router.
 */
export function WorkflowsPage() {
	return <WorkflowsView />;
}
