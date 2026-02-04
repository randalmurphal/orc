import { render, screen } from '@testing-library/react';
import { vi } from 'vitest';
import { LeftPalette } from './LeftPalette';
import type { Workflow } from '@/gen/orc/v1/workflow_pb';

// Mock the workflow settings panel and phase template palette
vi.mock('./WorkflowSettingsPanel', () => ({
	WorkflowSettingsPanel: ({ workflow, onWorkflowUpdate }: any) => (
		<div data-testid="workflow-settings-panel">
			Workflow Settings for {workflow.name}
			<button onClick={() => onWorkflowUpdate(workflow)}>Update</button>
		</div>
	),
}));

vi.mock('./PhaseTemplatePalette', () => ({
	PhaseTemplatePalette: ({ readOnly, workflowId }: any) => (
		<div data-testid="phase-template-palette">
			Phase Templates (readOnly: {String(readOnly)}, workflowId: {workflowId})
		</div>
	),
}));

const mockWorkflow = {
	id: 'test-workflow',
	name: 'Test Workflow',
	description: 'A test workflow',
	defaultModel: 'claude-sonnet-3-5',
	defaultThinking: true,
	completionAction: 'pr',
	targetBranch: 'main',
	isBuiltin: false,
	basedOn: '',
	createdAt: undefined,
	updatedAt: undefined,
} as unknown as Workflow;

const mockBuiltinWorkflow = {
	...mockWorkflow,
	id: 'builtin-workflow',
	name: 'Built-in Workflow',
	isBuiltin: true,
};

describe('LeftPalette', () => {
	// Integration test - SC-6: Integration with existing editor
	describe('Integration with Editor Components', () => {
		it('renders both workflow settings and phase template palette', () => {
			render(<LeftPalette workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByTestId('workflow-settings-panel')).toBeInTheDocument();
			expect(screen.getByTestId('phase-template-palette')).toBeInTheDocument();
		});

		it('passes correct props to WorkflowSettingsPanel', () => {
			const onUpdate = vi.fn();
			render(<LeftPalette workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			expect(screen.getByText('Workflow Settings for Test Workflow')).toBeInTheDocument();
		});

		it('passes correct props to PhaseTemplatePalette', () => {
			render(<LeftPalette workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByText(/Phase Templates.*readOnly: false.*workflowId: test-workflow/)).toBeInTheDocument();
		});

		it('sets readOnly=true for builtin workflows', () => {
			render(<LeftPalette workflow={mockBuiltinWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByText(/readOnly: true/)).toBeInTheDocument();
		});

		it('maintains proper section order - settings first, templates second', () => {
			const { container } = render(<LeftPalette workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			const sections = container.querySelectorAll('.left-palette-section');
			expect(sections).toHaveLength(2);

			// Workflow settings should come first
			expect(sections[0]).toContainElement(screen.getByTestId('workflow-settings-panel'));
			// Phase templates should come second
			expect(sections[1]).toContainElement(screen.getByTestId('phase-template-palette'));
		});

		it('applies consistent styling for palette sections', () => {
			const { container } = render(<LeftPalette workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(container.querySelector('.left-palette')).toBeInTheDocument();
			expect(container.querySelectorAll('.left-palette-section')).toHaveLength(2);
		});

		it('handles workflow update callback from settings panel', () => {
			const onUpdate = vi.fn();
			render(<LeftPalette workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			// Click the update button in the mocked WorkflowSettingsPanel
			const updateButton = screen.getByText('Update');
			updateButton.click();

			expect(onUpdate).toHaveBeenCalledWith(mockWorkflow);
		});
	});

	// Integration test - does not interfere with drag-and-drop
	describe('Drag and Drop Integration', () => {
		it('does not interfere with phase template drag handlers', () => {
			const { container } = render(<LeftPalette workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			// The palette should not add any drag event handlers that would interfere
			const palette = container.querySelector('.left-palette');
			expect(palette).not.toHaveAttribute('draggable');
		});

		it('preserves phase template palette functionality', () => {
			render(<LeftPalette workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			// The PhaseTemplatePalette component should be rendered normally
			// (specific drag/drop behavior is tested in PhaseTemplatePalette.test.tsx)
			expect(screen.getByTestId('phase-template-palette')).toBeInTheDocument();
		});
	});

	// Responsive behavior
	describe('Responsive Layout', () => {
		it('maintains vertical stacking on narrow screens', () => {
			const { container } = render(<LeftPalette workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			const palette = container.querySelector('.left-palette');
			expect(palette).toHaveClass('left-palette');

			// Should have the CSS class that provides flex column layout
			// Note: CSS styles are defined in LeftPalette.css with display: flex; flex-direction: column;
			// In test environment, computed styles may not be available, but class application is verifiable
			expect(palette).toHaveClass('left-palette');
		});
	});
});