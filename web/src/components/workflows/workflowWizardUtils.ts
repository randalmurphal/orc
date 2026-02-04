/**
 * Utility functions for the WorkflowCreationWizard.
 *
 * These functions provide:
 * - Intent to phase recommendations mapping
 * - ID slugification for workflow IDs
 */

/**
 * The workflow intent types that users can select in the wizard.
 */
export type WorkflowIntent = 'build' | 'review' | 'test' | 'document' | 'custom';

/**
 * Phase recommendations based on workflow intent.
 *
 * Maps each intent to an ordered list of recommended phase template IDs.
 * The order matches the typical workflow execution order.
 */
const INTENT_PHASE_MAP: Record<WorkflowIntent, string[]> = {
	build: ['spec', 'implement', 'review'],
	review: ['review'],
	test: ['tdd_write', 'test'],
	document: ['docs'],
	custom: [],
};

/**
 * Returns the recommended phases for a given workflow intent.
 *
 * @param intent - The workflow intent selected by the user
 * @returns An ordered array of phase template IDs recommended for this intent
 */
export function getRecommendedPhases(intent: WorkflowIntent): string[] {
	return INTENT_PHASE_MAP[intent] ?? [];
}

/**
 * Slugifies a workflow name into a valid workflow ID.
 *
 * Converts to lowercase, replaces spaces and special characters with hyphens,
 * collapses multiple hyphens, removes leading/trailing hyphens, and truncates
 * to max 50 characters.
 *
 * @param name - The workflow name to slugify
 * @returns A valid workflow ID slug
 */
export function slugifyWorkflowId(name: string): string {
	if (!name) return '';

	return name
		// Convert to lowercase
		.toLowerCase()
		// Replace any non-alphanumeric characters (except hyphens) with hyphens
		.replace(/[^a-z0-9-]+/g, '-')
		// Collapse multiple consecutive hyphens
		.replace(/-+/g, '-')
		// Remove leading and trailing hyphens
		.replace(/^-+|-+$/g, '')
		// Truncate to max 50 characters
		.slice(0, 50);
}
