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

// Mock AgentsPalette to verify wiring (SC-1: Agents section appears in left palette)
vi.mock('./AgentsPalette', () => ({
	AgentsPalette: ({ readOnly, onAgentClick, onAgentAssign }: any) => (
		<div data-testid="agents-palette" data-readonly={String(readOnly)}>
			Agents Palette (readOnly: {String(readOnly)})
			<button onClick={() => onAgentClick?.({ id: 'test' })}>Click Agent</button>
			<button onClick={() => onAgentAssign?.({ id: 'test' })}>Assign Agent</button>
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

// Default props for LeftPalette - all required props must be provided
const defaultProps = {
	workflow: mockWorkflow,
	onWorkflowUpdate: vi.fn(),
	onAgentClick: vi.fn(),
	onAgentAssign: vi.fn(),
	selectedNodeId: null as string | null,
};

describe('LeftPalette', () => {
	// ─────────────────────────────────────────────────────────────────────────────
	// TASK-725: Integration tests for AgentsPalette wiring
	// These tests verify that AgentsPalette is properly integrated into LeftPalette
	// and will FAIL if the wiring is missing (dead code prevention)
	// ─────────────────────────────────────────────────────────────────────────────

	// SC-1: Agents section appears in left palette below Workflow Settings
	describe('AgentsPalette Integration (SC-1)', () => {
		it('renders AgentsPalette section', () => {
			render(<LeftPalette {...defaultProps} />);

			// This test FAILS if AgentsPalette is not imported and rendered by LeftPalette
			expect(screen.getByTestId('agents-palette')).toBeInTheDocument();
		});

		it('passes readOnly=false to AgentsPalette for custom workflows', () => {
			render(<LeftPalette {...defaultProps} />);

			const agentsPalette = screen.getByTestId('agents-palette');
			expect(agentsPalette).toHaveAttribute('data-readonly', 'false');
		});

		it('passes readOnly=true to AgentsPalette for builtin workflows', () => {
			render(<LeftPalette {...defaultProps} workflow={mockBuiltinWorkflow} />);

			const agentsPalette = screen.getByTestId('agents-palette');
			expect(agentsPalette).toHaveAttribute('data-readonly', 'true');
		});

		it('renders AgentsPalette between WorkflowSettings and PhaseTemplates', () => {
			const { container } = render(
				<LeftPalette {...defaultProps} />
			);

			const sections = container.querySelectorAll('.left-palette-section');
			// Should now have 3 sections: settings, agents, templates
			expect(sections).toHaveLength(3);

			// Verify order: settings first, agents second, templates third
			expect(sections[0]).toContainElement(screen.getByTestId('workflow-settings-panel'));
			expect(sections[1]).toContainElement(screen.getByTestId('agents-palette'));
			expect(sections[2]).toContainElement(screen.getByTestId('phase-template-palette'));
		});

		it('provides onAgentClick callback to AgentsPalette', () => {
			const onAgentClick = vi.fn();
			render(
				<LeftPalette
					{...defaultProps}
					onAgentClick={onAgentClick}
				/>
			);

			// Click the mock agent click button
			const clickButton = screen.getByText('Click Agent');
			clickButton.click();

			expect(onAgentClick).toHaveBeenCalledWith({ id: 'test' });
		});

		it('provides onAgentAssign callback to AgentsPalette', () => {
			const onAgentAssign = vi.fn();
			render(
				<LeftPalette
					{...defaultProps}
					onAgentAssign={onAgentAssign}
				/>
			);

			// Click the mock agent assign button
			const assignButton = screen.getByText('Assign Agent');
			assignButton.click();

			expect(onAgentAssign).toHaveBeenCalledWith({ id: 'test' });
		});
	});

	// Integration test - SC-6: Integration with existing editor
	describe('Integration with Editor Components', () => {
		it('renders all three palette sections (settings, agents, templates)', () => {
			render(<LeftPalette {...defaultProps} />);

			expect(screen.getByTestId('workflow-settings-panel')).toBeInTheDocument();
			expect(screen.getByTestId('agents-palette')).toBeInTheDocument();
			expect(screen.getByTestId('phase-template-palette')).toBeInTheDocument();
		});

		it('passes correct props to WorkflowSettingsPanel', () => {
			const onUpdate = vi.fn();
			render(<LeftPalette {...defaultProps} onWorkflowUpdate={onUpdate} />);

			expect(screen.getByText('Workflow Settings for Test Workflow')).toBeInTheDocument();
		});

		it('passes correct props to PhaseTemplatePalette', () => {
			render(<LeftPalette {...defaultProps} />);

			expect(screen.getByText(/Phase Templates.*readOnly: false.*workflowId: test-workflow/)).toBeInTheDocument();
		});

		it('sets readOnly=true for builtin workflows', () => {
			render(<LeftPalette {...defaultProps} workflow={mockBuiltinWorkflow} />);

			// Both AgentsPalette and PhaseTemplatePalette show readOnly: true
			const readOnlyElements = screen.getAllByText(/readOnly: true/);
			expect(readOnlyElements.length).toBeGreaterThanOrEqual(1);
		});

		it('maintains proper section order - settings first, agents second, templates third', () => {
			const { container } = render(<LeftPalette {...defaultProps} />);

			const sections = container.querySelectorAll('.left-palette-section');
			expect(sections).toHaveLength(3);

			// Workflow settings should come first
			expect(sections[0]).toContainElement(screen.getByTestId('workflow-settings-panel'));
			// Agents should come second
			expect(sections[1]).toContainElement(screen.getByTestId('agents-palette'));
			// Phase templates should come third
			expect(sections[2]).toContainElement(screen.getByTestId('phase-template-palette'));
		});

		it('applies consistent styling for palette sections', () => {
			const { container } = render(<LeftPalette {...defaultProps} />);

			expect(container.querySelector('.left-palette')).toBeInTheDocument();
			expect(container.querySelectorAll('.left-palette-section')).toHaveLength(3);
		});

		it('handles workflow update callback from settings panel', () => {
			const onUpdate = vi.fn();
			render(<LeftPalette {...defaultProps} onWorkflowUpdate={onUpdate} />);

			// Click the update button in the mocked WorkflowSettingsPanel
			const updateButton = screen.getByText('Update');
			updateButton.click();

			expect(onUpdate).toHaveBeenCalledWith(mockWorkflow);
		});
	});

	// Integration test - does not interfere with drag-and-drop
	describe('Drag and Drop Integration', () => {
		it('does not interfere with phase template drag handlers', () => {
			const { container } = render(<LeftPalette {...defaultProps} />);

			// The palette should not add any drag event handlers that would interfere
			const palette = container.querySelector('.left-palette');
			expect(palette).not.toHaveAttribute('draggable');
		});

		it('preserves phase template palette functionality', () => {
			render(<LeftPalette {...defaultProps} />);

			// The PhaseTemplatePalette component should be rendered normally
			// (specific drag/drop behavior is tested in PhaseTemplatePalette.test.tsx)
			expect(screen.getByTestId('phase-template-palette')).toBeInTheDocument();
		});
	});

	// Responsive behavior
	describe('Responsive Layout', () => {
		it('maintains vertical stacking on narrow screens', () => {
			const { container } = render(<LeftPalette {...defaultProps} />);

			const palette = container.querySelector('.left-palette');
			expect(palette).toHaveClass('left-palette');

			// Should have the CSS class that provides flex column layout
			// Note: CSS styles are defined in LeftPalette.css with display: flex; flex-direction: column;
			// In test environment, computed styles may not be available, but class application is verifiable
			expect(palette).toHaveClass('left-palette');
		});
	});
});