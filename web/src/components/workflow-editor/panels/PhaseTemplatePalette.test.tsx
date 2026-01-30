/**
 * TDD Tests for PhaseTemplatePalette component
 *
 * Tests for TASK-637: Left sidebar panel showing available phase templates
 * grouped by category with search filtering, drag support, and read-only mode.
 *
 * Success Criteria Coverage:
 * - Renders templates grouped by category headers
 * - Search filters templates by name/id/description
 * - Read-only mode shows clone banner and disables drag
 * - Editable mode enables drag on template cards
 * - Category headers toggle visibility (collapsible)
 * - Model override badge shown when modelOverride is set
 * - Gate type badge shown when gateType is not AUTO
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PhaseTemplatePalette } from './PhaseTemplatePalette';
import { usePhaseTemplates } from '@/stores/workflowStore';
import { createMockPhaseTemplate } from '@/test/factories';
import { GateType } from '@/gen/orc/v1/workflow_pb';

// Mock the workflowStore
vi.mock('@/stores/workflowStore', () => ({
	useWorkflowStore: vi.fn(),
	usePhaseTemplates: vi.fn(),
}));

/** Standard set of templates spanning all four categories */
function createStandardTemplates() {
	return [
		createMockPhaseTemplate({ id: 'spec', name: 'Full Spec', description: 'Generate specification' }),
		createMockPhaseTemplate({ id: 'implement', name: 'Implement', description: 'Implement the feature' }),
		createMockPhaseTemplate({ id: 'review', name: 'Review', description: 'Code review', gateType: GateType.HUMAN }),
		createMockPhaseTemplate({ id: 'docs', name: 'Documentation', description: 'Generate docs' }),
	];
}

describe('PhaseTemplatePalette', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('renders templates grouped by category', () => {
		it('shows category headers and template names', () => {
			vi.mocked(usePhaseTemplates).mockReturnValue(createStandardTemplates());

			render(<PhaseTemplatePalette readOnly={false} workflowId="test-wf" />);

			// Category headers
			expect(screen.getByText('Specification')).toBeTruthy();
			expect(screen.getByText('Implementation')).toBeTruthy();
			expect(screen.getByText('Quality')).toBeTruthy();
			// "Documentation" appears both as category header and template name for the "docs" template,
			// so getAllByText is needed to avoid the multiple-elements error from getByText
			expect(screen.getAllByText('Documentation').length).toBeGreaterThanOrEqual(1);

			// Template names
			expect(screen.getByText('Full Spec')).toBeTruthy();
			expect(screen.getByText('Implement')).toBeTruthy();
			expect(screen.getByText('Review')).toBeTruthy();
		});
	});

	describe('search filters templates', () => {
		it('only shows matching templates when search query is entered', async () => {
			const user = userEvent.setup();
			vi.mocked(usePhaseTemplates).mockReturnValue(createStandardTemplates());

			render(<PhaseTemplatePalette readOnly={false} workflowId="test-wf" />);

			// Type in search box
			const searchInput = screen.getByRole('searchbox') ?? screen.getByPlaceholderText(/search/i);
			await user.type(searchInput, 'spec');

			// Only "Full Spec" should be visible (matches id "spec" or name)
			expect(screen.getByText('Full Spec')).toBeTruthy();
			expect(screen.queryByText('Implement')).toBeNull();
			expect(screen.queryByText('Review')).toBeNull();
		});
	});

	describe('read-only mode', () => {
		it('shows clone banner when readOnly is true', () => {
			vi.mocked(usePhaseTemplates).mockReturnValue(createStandardTemplates());

			render(<PhaseTemplatePalette readOnly={true} workflowId="test-wf" />);

			expect(screen.getByText(/clone to customize/i)).toBeTruthy();
		});

		it('disables drag when readOnly is true', () => {
			vi.mocked(usePhaseTemplates).mockReturnValue(createStandardTemplates());

			render(<PhaseTemplatePalette readOnly={true} workflowId="test-wf" />);

			// Template cards should NOT have draggable attribute
			const cards = document.querySelectorAll('[data-testid="template-card"]');
			cards.forEach((card) => {
				expect(card.getAttribute('draggable')).not.toBe('true');
			});
		});
	});

	describe('editable mode', () => {
		it('enables drag when readOnly is false', () => {
			vi.mocked(usePhaseTemplates).mockReturnValue(createStandardTemplates());

			render(<PhaseTemplatePalette readOnly={false} workflowId="test-wf" />);

			// Template cards SHOULD have draggable="true"
			const cards = document.querySelectorAll('[data-testid="template-card"]');
			expect(cards.length).toBeGreaterThan(0);
			cards.forEach((card) => {
				expect(card.getAttribute('draggable')).toBe('true');
			});
		});
	});

	describe('collapsible categories', () => {
		it('toggles template visibility when category header is clicked', async () => {
			const user = userEvent.setup();
			vi.mocked(usePhaseTemplates).mockReturnValue(createStandardTemplates());

			render(<PhaseTemplatePalette readOnly={false} workflowId="test-wf" />);

			// Initially "Full Spec" is visible under Specification
			expect(screen.getByText('Full Spec')).toBeTruthy();

			// Click "Specification" header to collapse
			await user.click(screen.getByText('Specification'));

			// "Full Spec" should be hidden
			expect(screen.queryByText('Full Spec')).toBeNull();

			// Click again to expand
			await user.click(screen.getByText('Specification'));

			// "Full Spec" should be visible again
			expect(screen.getByText('Full Spec')).toBeTruthy();
		});
	});

	describe('badges', () => {
		it('shows agent badge when agentId is set', () => {
			const templates = [
				createMockPhaseTemplate({
					id: 'implement',
					name: 'Implement',
					description: 'Implement the feature',
					agentId: 'impl-executor',
				}),
			];
			vi.mocked(usePhaseTemplates).mockReturnValue(templates);

			render(<PhaseTemplatePalette readOnly={false} workflowId="test-wf" />);

			// Should show a badge with the agent id
			expect(screen.getByText('impl-executor')).toBeTruthy();
		});

		it('shows gate badge when gateType is HUMAN', () => {
			const templates = [
				createMockPhaseTemplate({
					id: 'review',
					name: 'Review',
					description: 'Code review',
					gateType: GateType.HUMAN,
				}),
			];
			vi.mocked(usePhaseTemplates).mockReturnValue(templates);

			render(<PhaseTemplatePalette readOnly={false} workflowId="test-wf" />);

			// Should show "Human" badge for HUMAN gate type
			expect(screen.getByText(/human/i)).toBeTruthy();
		});
	});
});
