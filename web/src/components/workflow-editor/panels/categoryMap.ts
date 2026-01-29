import type { PhaseTemplate } from '@/gen/orc/v1/workflow_pb';

export type CategoryName = 'Specification' | 'Implementation' | 'Quality' | 'Documentation' | 'Other';

const CATEGORY_MAP: Record<string, CategoryName> = {
	spec: 'Specification',
	tiny_spec: 'Specification',
	research: 'Specification',
	design: 'Specification',
	implement: 'Implementation',
	tdd_write: 'Implementation',
	breakdown: 'Implementation',
	review: 'Quality',
	validate: 'Quality',
	qa: 'Quality',
	qa_e2e_test: 'Quality',
	qa_e2e_fix: 'Quality',
	docs: 'Documentation',
};

export const CATEGORY_ORDER: CategoryName[] = [
	'Specification',
	'Implementation',
	'Quality',
	'Documentation',
	'Other',
];

export function getCategoryForTemplate(templateId: string): CategoryName {
	return CATEGORY_MAP[templateId] ?? 'Other';
}

export function filterTemplates(templates: PhaseTemplate[], query: string): PhaseTemplate[] {
	if (!query) return templates;
	const q = query.toLowerCase();
	return templates.filter(
		(t) =>
			t.name.toLowerCase().includes(q) ||
			t.id.toLowerCase().includes(q) ||
			(t.description?.toLowerCase().includes(q) ?? false),
	);
}
