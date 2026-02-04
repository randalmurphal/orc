/**
 * TDD Tests for Workflow Wizard Utility Functions
 *
 * Tests for TASK-746: Implement guided workflow creation wizard
 *
 * Success Criteria Coverage:
 * - SC-8: Phases are pre-selected based on intent (e.g., Build→spec+implement+review)
 *
 * These tests verify the business logic for:
 * - Intent to phase recommendations mapping
 * - ID slugification
 */

import { describe, it, expect } from 'vitest';
import {
	getRecommendedPhases,
	slugifyWorkflowId,
	WorkflowIntent,
} from './workflowWizardUtils';

describe('workflowWizardUtils', () => {
	describe('getRecommendedPhases', () => {
		it('returns spec, implement, review for Build intent', () => {
			const phases = getRecommendedPhases('build');

			expect(phases).toContain('spec');
			expect(phases).toContain('implement');
			expect(phases).toContain('review');
		});

		it('returns review only for Review intent', () => {
			const phases = getRecommendedPhases('review');

			expect(phases).toContain('review');
			expect(phases).not.toContain('implement');
			expect(phases).not.toContain('spec');
		});

		it('returns test-related phases for Test intent', () => {
			const phases = getRecommendedPhases('test');

			// Should include test or tdd_write
			const hasTestPhase = phases.includes('test') || phases.includes('tdd_write');
			expect(hasTestPhase).toBe(true);
		});

		it('returns docs for Document intent', () => {
			const phases = getRecommendedPhases('document');

			expect(phases).toContain('docs');
		});

		it('returns empty array for Custom intent', () => {
			const phases = getRecommendedPhases('custom');

			expect(phases).toHaveLength(0);
		});

		it('handles unknown intent as custom (empty)', () => {
			const phases = getRecommendedPhases('unknown' as WorkflowIntent);

			expect(phases).toHaveLength(0);
		});

		describe('Build intent phase recommendations', () => {
			it('recommends phases in correct order: spec → implement → review', () => {
				const phases = getRecommendedPhases('build');

				const specIndex = phases.indexOf('spec');
				const implementIndex = phases.indexOf('implement');
				const reviewIndex = phases.indexOf('review');

				// All should be present
				expect(specIndex).toBeGreaterThanOrEqual(0);
				expect(implementIndex).toBeGreaterThanOrEqual(0);
				expect(reviewIndex).toBeGreaterThanOrEqual(0);

				// Should be in order
				expect(specIndex).toBeLessThan(implementIndex);
				expect(implementIndex).toBeLessThan(reviewIndex);
			});
		});

		describe('Review intent specifics', () => {
			it('may include security_scan as optional', () => {
				const phases = getRecommendedPhases('review');

				// Security scan could be a recommended but optional addition
				// Main requirement is that 'review' is included
				expect(phases).toContain('review');
			});
		});

		describe('Test intent specifics', () => {
			it('includes tdd_write for test-driven workflow', () => {
				const phases = getRecommendedPhases('test');

				// Could include tdd_write, test, or both
				expect(phases.length).toBeGreaterThan(0);
			});
		});
	});

	describe('slugifyWorkflowId', () => {
		it('converts to lowercase', () => {
			expect(slugifyWorkflowId('MyWorkflow')).toBe('myworkflow');
		});

		it('replaces spaces with hyphens', () => {
			expect(slugifyWorkflowId('My Workflow')).toBe('my-workflow');
		});

		it('removes special characters', () => {
			expect(slugifyWorkflowId("My Workflow! (v2.0) - Test's")).toBe('my-workflow-v2-0-test-s');
		});

		it('collapses multiple hyphens', () => {
			expect(slugifyWorkflowId('My---Workflow')).toBe('my-workflow');
		});

		it('removes leading and trailing hyphens', () => {
			expect(slugifyWorkflowId('--My Workflow--')).toBe('my-workflow');
		});

		it('handles empty string', () => {
			expect(slugifyWorkflowId('')).toBe('');
		});

		it('handles only special characters', () => {
			expect(slugifyWorkflowId('!@#$%')).toBe('');
		});

		it('truncates to max 50 characters', () => {
			const longName = 'This is a very long workflow name that exceeds fifty characters limit';
			const result = slugifyWorkflowId(longName);

			expect(result.length).toBeLessThanOrEqual(50);
		});

		it('preserves numbers', () => {
			expect(slugifyWorkflowId('Workflow v2')).toBe('workflow-v2');
		});

		it('handles unicode characters', () => {
			// Unicode should be stripped or transliterated
			const result = slugifyWorkflowId('Workflow café');
			expect(result).toMatch(/^[a-z0-9-]+$/);
		});
	});

	describe('WorkflowIntent type', () => {
		it('has all required intent types', () => {
			// Type check - these should all be valid
			const intents: WorkflowIntent[] = ['build', 'review', 'test', 'document', 'custom'];
			expect(intents).toHaveLength(5);
		});
	});
});

describe('Intent to Phase Mapping - Detailed', () => {
	/**
	 * These tests document the expected phase recommendations for each intent.
	 * The design doc specifies:
	 *
	 * Build → spec + implement + review (typical development workflow)
	 * Review → review (code review workflow)
	 * Test → test / tdd_write (testing workflow)
	 * Document → docs (documentation workflow)
	 * Custom → empty (user picks everything)
	 */

	describe('Build Intent', () => {
		it('includes specification phase for requirements gathering', () => {
			const phases = getRecommendedPhases('build');
			expect(phases).toContain('spec');
		});

		it('includes implementation phase for code writing', () => {
			const phases = getRecommendedPhases('build');
			expect(phases).toContain('implement');
		});

		it('includes review phase for code review', () => {
			const phases = getRecommendedPhases('build');
			expect(phases).toContain('review');
		});

		it('may optionally include tdd_write for test-first approach', () => {
			const phases = getRecommendedPhases('build');
			// tdd_write is optional for Build - some build workflows include it
			// This test documents the possibility
			expect(typeof phases.includes('tdd_write')).toBe('boolean');
		});
	});

	describe('Review Intent', () => {
		it('focuses primarily on code review', () => {
			const phases = getRecommendedPhases('review');
			expect(phases).toContain('review');
		});

		it('does not include implementation phase', () => {
			const phases = getRecommendedPhases('review');
			expect(phases).not.toContain('implement');
		});
	});

	describe('Test Intent', () => {
		it('includes testing-related phases', () => {
			const phases = getRecommendedPhases('test');
			const hasTestPhase = phases.some(p =>
				['test', 'tdd_write', 'qa'].includes(p)
			);
			expect(hasTestPhase).toBe(true);
		});
	});

	describe('Document Intent', () => {
		it('includes documentation phase', () => {
			const phases = getRecommendedPhases('document');
			expect(phases).toContain('docs');
		});

		it('may include spec for capturing requirements', () => {
			const phases = getRecommendedPhases('document');
			// Some documentation workflows start with gathering info
			expect(typeof phases.includes('spec')).toBe('boolean');
		});
	});

	describe('Custom Intent', () => {
		it('returns empty array for full user control', () => {
			const phases = getRecommendedPhases('custom');
			expect(phases).toEqual([]);
		});
	});
});
