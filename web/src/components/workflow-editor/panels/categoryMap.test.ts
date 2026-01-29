/**
 * TDD Tests for categoryMap utility functions
 *
 * Tests for TASK-637: PhaseTemplatePalette category mapping and filtering
 *
 * These tests validate:
 * - getCategoryForTemplate maps template IDs to the correct category
 * - Unknown template IDs fall back to "Other"
 * - filterTemplates filters by name, id, or description (case-insensitive)
 */

import { describe, it, expect } from 'vitest';
import { getCategoryForTemplate, filterTemplates } from './categoryMap';
import { createMockPhaseTemplate } from '@/test/factories';

describe('getCategoryForTemplate', () => {
	it.each([
		// Specification category
		['spec', 'Specification'],
		['tiny_spec', 'Specification'],
		['research', 'Specification'],
		['design', 'Specification'],
		// Implementation category
		['implement', 'Implementation'],
		['tdd_write', 'Implementation'],
		['breakdown', 'Implementation'],
		// Quality category
		['review', 'Quality'],
		['validate', 'Quality'],
		['qa', 'Quality'],
		['qa_e2e_test', 'Quality'],
		['qa_e2e_fix', 'Quality'],
		// Documentation category
		['docs', 'Documentation'],
	])('maps "%s" to "%s"', (templateId, expectedCategory) => {
		expect(getCategoryForTemplate(templateId)).toBe(expectedCategory);
	});

	it('returns "Other" for unknown template IDs', () => {
		expect(getCategoryForTemplate('unknown_phase')).toBe('Other');
		expect(getCategoryForTemplate('')).toBe('Other');
		expect(getCategoryForTemplate('custom_phase_xyz')).toBe('Other');
	});
});

describe('filterTemplates', () => {
	const templates = [
		createMockPhaseTemplate({ id: 'spec', name: 'Full Spec', description: 'Generate specification' }),
		createMockPhaseTemplate({ id: 'implement', name: 'Implement', description: 'Implement the feature' }),
		createMockPhaseTemplate({ id: 'review', name: 'Review', description: 'Code review' }),
		createMockPhaseTemplate({ id: 'docs', name: 'Documentation', description: 'Generate docs' }),
	];

	it('returns all templates when query is empty', () => {
		const result = filterTemplates(templates, '');
		expect(result).toHaveLength(4);
	});

	it('matches by name (case insensitive)', () => {
		const result = filterTemplates(templates, 'full spec');
		expect(result).toHaveLength(1);
		expect(result[0].id).toBe('spec');
	});

	it('matches by id', () => {
		const result = filterTemplates(templates, 'implement');
		expect(result).toHaveLength(1);
		expect(result[0].id).toBe('implement');
	});

	it('matches by description', () => {
		const result = filterTemplates(templates, 'code review');
		expect(result).toHaveLength(1);
		expect(result[0].id).toBe('review');
	});

	it('returns empty array when no templates match', () => {
		const result = filterTemplates(templates, 'nonexistent_query_xyz');
		expect(result).toHaveLength(0);
	});
});
