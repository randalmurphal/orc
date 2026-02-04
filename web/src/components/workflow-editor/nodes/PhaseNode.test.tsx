import { describe, it, expect } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ReactFlowProvider } from '@xyflow/react';
import { PhaseNode } from './PhaseNode';
import type { PhaseNodeData } from './index';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import { TooltipProvider } from '@/components/ui/Tooltip';

// React Flow Handle components require ReactFlowProvider context
// TooltipProvider required for variable tooltips
function renderPhaseNode(
	data: PhaseNodeData,
	opts: { selected?: boolean } = {}
) {
	return render(
		<TooltipProvider delayDuration={0}>
			<ReactFlowProvider>
				<PhaseNode
					id="phase-1"
					type="phase"
					data={data}
					selected={opts.selected ?? false}
					isConnectable={true}
					positionAbsoluteX={0}
					positionAbsoluteY={0}
					zIndex={0}
					draggable={true}
					dragging={false}
					selectable={true}
					deletable={true}
				/>
			</ReactFlowProvider>
		</TooltipProvider>
	);
}

function createDefaultData(
	overrides: Partial<PhaseNodeData> = {}
): PhaseNodeData {
	return {
		phaseTemplateId: 'implement',
		templateName: 'Implement',
		sequence: 1,
		phaseId: 1,
		gateType: GateType.AUTO,
		...overrides,
	};
}

// NOTE: Browser API mocks (ResizeObserver, IntersectionObserver) provided by global test-setup.ts
describe('PhaseNode', () => {
	describe('SC-1: template name and ID display', () => {
		it('renders template name as header text', () => {
			renderPhaseNode(createDefaultData({ templateName: 'Specification' }));

			const nameEl = document.querySelector('.phase-node__name');
			expect(nameEl).not.toBeNull();
			expect(nameEl!.textContent).toBe('Specification');
		});

		it('renders phaseTemplateId as subtitle', () => {
			renderPhaseNode(
				createDefaultData({
					templateName: 'Specification',
					phaseTemplateId: 'spec',
				})
			);

			const templateIdEl = document.querySelector('.phase-node__id');
			expect(templateIdEl).not.toBeNull();
			expect(templateIdEl!.textContent).toBe('spec');
		});

		it('falls back to phaseTemplateId for name when templateName is missing', () => {
			renderPhaseNode(
				createDefaultData({
					templateName: '',
					phaseTemplateId: 'tdd_write',
				})
			);

			const nameEl = document.querySelector('.phase-node__name');
			expect(nameEl).not.toBeNull();
			expect(nameEl!.textContent).toBe('tdd_write');
		});
	});

	describe('SC-2: type badge display', () => {
		it('displays AI badge for AUTO gate type', () => {
			renderPhaseNode(createDefaultData({ gateType: GateType.AUTO }));

			const badge = document.querySelector('.phase-node__badge--ai');
			expect(badge).not.toBeNull();
			expect(badge!.textContent).toBe('AI');
		});

		it('displays Human badge for HUMAN gate type', () => {
			renderPhaseNode(createDefaultData({ gateType: GateType.HUMAN }));

			const badge = document.querySelector('.phase-node__badge--human');
			expect(badge).not.toBeNull();
			expect(badge!.textContent).toBe('Human');
		});

		it('displays Skip badge for SKIP gate type', () => {
			renderPhaseNode(createDefaultData({ gateType: GateType.SKIP }));

			const badge = document.querySelector('.phase-node__badge--skip');
			expect(badge).not.toBeNull();
			expect(badge!.textContent).toBe('Skip');
		});
	});

	describe('SC-4: React Flow handles', () => {
		it('renders target handle on left and source handle on right', () => {
			const { container } = renderPhaseNode(createDefaultData());

			// React Flow Handle components render with data-handlepos attribute
			const handles = container.querySelectorAll('.react-flow__handle');
			expect(handles.length).toBeGreaterThanOrEqual(2);

			const targetHandle = container.querySelector(
				'.react-flow__handle-left'
			);
			const sourceHandle = container.querySelector(
				'.react-flow__handle-right'
			);
			expect(targetHandle).not.toBeNull();
			expect(sourceHandle).not.toBeNull();
		});
	});

	describe('SC-7: execution status overlay', () => {
		it('applies running class with glow animation when status is running', () => {
			renderPhaseNode(
				createDefaultData({ status: 'running' as any })
			);

			const node = document.querySelector('.phase-node');
			expect(node).not.toBeNull();
			expect(node!.classList.contains('phase-node--running')).toBe(true);
		});

		it('applies completed class with green border when status is completed', () => {
			renderPhaseNode(
				createDefaultData({ status: 'completed' as any })
			);

			const node = document.querySelector('.phase-node');
			expect(node).not.toBeNull();
			expect(node!.classList.contains('phase-node--completed')).toBe(
				true
			);
		});

		it('applies failed class with red border when status is failed', () => {
			renderPhaseNode(
				createDefaultData({ status: 'failed' as any })
			);

			const node = document.querySelector('.phase-node');
			expect(node).not.toBeNull();
			expect(node!.classList.contains('phase-node--failed')).toBe(true);
		});

		it('applies skipped class with dimming and strikethrough when status is skipped', () => {
			renderPhaseNode(
				createDefaultData({ status: 'skipped' as any })
			);

			const node = document.querySelector('.phase-node');
			expect(node).not.toBeNull();
			expect(node!.classList.contains('phase-node--skipped')).toBe(true);
		});

		it('applies pending class when status is pending', () => {
			renderPhaseNode(
				createDefaultData({ status: 'pending' as any })
			);

			const node = document.querySelector('.phase-node');
			expect(node).not.toBeNull();
			expect(node!.classList.contains('phase-node--pending')).toBe(true);
		});

		it('renders default card with no status class when status is undefined', () => {
			renderPhaseNode(createDefaultData({ status: undefined }));

			const node = document.querySelector('.phase-node');
			expect(node).not.toBeNull();
			expect(node!.classList.contains('phase-node--running')).toBe(false);
			expect(node!.classList.contains('phase-node--completed')).toBe(
				false
			);
			expect(node!.classList.contains('phase-node--failed')).toBe(false);
			expect(node!.classList.contains('phase-node--skipped')).toBe(false);
		});

		it('renders default card with no status class when status is UNSPECIFIED', () => {
			renderPhaseNode(
				createDefaultData({ status: 'unspecified' as any })
			);

			const node = document.querySelector('.phase-node');
			expect(node).not.toBeNull();
			// UNSPECIFIED should render as default (no status modifier)
			expect(node!.classList.contains('phase-node--running')).toBe(false);
			expect(node!.classList.contains('phase-node--completed')).toBe(
				false
			);
		});
	});

	describe('SC-8: execution data display', () => {
		it('displays iteration count when provided', () => {
			renderPhaseNode(
				createDefaultData({
					status: 'completed' as any,
					iterations: 3,
				})
			);

			expect(screen.getByText(/3/)).toBeTruthy();
		});

		it('displays cost when provided', () => {
			renderPhaseNode(
				createDefaultData({
					status: 'completed' as any,
					costUsd: 0.42,
				})
			);

			// Cost should be displayed with dollar formatting
			expect(screen.getByText(/\$0\.42/)).toBeTruthy();
		});

		it('displays both iterations and cost when both provided', () => {
			renderPhaseNode(
				createDefaultData({
					status: 'completed' as any,
					iterations: 3,
					costUsd: 0.42,
				})
			);

			expect(screen.getByText(/3/)).toBeTruthy();
			expect(screen.getByText(/\$0\.42/)).toBeTruthy();
		});

		it('does not render execution footer when no execution data present', () => {
			renderPhaseNode(
				createDefaultData({
					status: undefined,
					iterations: undefined,
					costUsd: undefined,
				})
			);

			// No dollar sign or iteration display when in design-time view
			expect(screen.queryByText(/\$/)).toBeNull();
		});

		it('does NOT show cost badge when costUsd is 0 (SC-4: avoid clutter)', () => {
			renderPhaseNode(
				createDefaultData({
					status: 'running' as any,
					costUsd: 0,
				})
			);

			expect(screen.queryByText(/\$0\.00/)).toBeNull();
		});
	});

	describe('SC-9: selected state', () => {
		it('adds selected class when node is selected', () => {
			renderPhaseNode(createDefaultData(), { selected: true });

			const node = document.querySelector('.phase-node');
			expect(node).not.toBeNull();
			expect(node!.classList.contains('phase-node--selected')).toBe(true);
		});

		it('does not add selected class when node is not selected', () => {
			renderPhaseNode(createDefaultData(), { selected: false });

			const node = document.querySelector('.phase-node');
			expect(node).not.toBeNull();
			expect(node!.classList.contains('phase-node--selected')).toBe(false);
		});
	});

	describe('edge cases', () => {
		it('handles very long template name with truncation', () => {
			const longName =
				'This Is A Very Long Phase Template Name That Should Be Truncated';
			renderPhaseNode(createDefaultData({ templateName: longName }));

			const nameEl = document.querySelector('.phase-node__name');
			expect(nameEl).not.toBeNull();
			expect(nameEl!.textContent).toBe(longName);
			// CSS handles truncation — just verify the text is present
		});

		it('renders with description when provided', () => {
			renderPhaseNode(
				createDefaultData({
					description: 'Run the implementation phase',
				})
			);

			// Component renders without error when description is present
			const node = document.querySelector('.phase-node');
			expect(node).not.toBeNull();
		});

		it('renders Skip badge for SKIP gate type', () => {
			renderPhaseNode(
				createDefaultData({ gateType: GateType.SKIP })
			);

			const skipBadge = document.querySelector(
				'.phase-node__badge--skip'
			);
			expect(skipBadge).not.toBeNull();
			expect(skipBadge!.textContent).toBe('Skip');
		});
	});

	// TASK-730: Variable tooltip tests
	describe('variable tooltip on hover', () => {
		it('shows tooltip with inputs and output when hovering phase node', async () => {
			const user = userEvent.setup();
			renderPhaseNode(
				createDefaultData({
					inputVariables: ['SPEC_CONTENT', 'BREAKDOWN'],
					outputVarName: 'IMPLEMENTATION',
				})
			);

			const node = document.querySelector('.phase-node')!;
			await user.hover(node);

			await waitFor(() => {
				expect(screen.getByRole('tooltip')).toBeInTheDocument();
			});

			// Query within visible tooltip content (direct child, not the hidden a11y copy)
			const visibleTooltip = document.querySelector(
				'.tooltip-content > .phase-node__tooltip'
			) as HTMLElement;
			const tooltip = within(visibleTooltip);

			// Tooltip should show both inputs and output
			expect(tooltip.getByText(/Inputs:/)).toBeInTheDocument();
			expect(tooltip.getByText(/SPEC_CONTENT/)).toBeInTheDocument();
			expect(tooltip.getByText(/BREAKDOWN/)).toBeInTheDocument();
			expect(tooltip.getByText(/Output:/)).toBeInTheDocument();
			expect(tooltip.getByText(/IMPLEMENTATION/)).toBeInTheDocument();
		});

		it('shows only inputs section when outputVarName is not set', async () => {
			const user = userEvent.setup();
			renderPhaseNode(
				createDefaultData({
					inputVariables: ['TASK_DESCRIPTION', 'INITIATIVE_VISION'],
					outputVarName: undefined,
				})
			);

			const node = document.querySelector('.phase-node')!;
			await user.hover(node);

			await waitFor(() => {
				expect(screen.getByRole('tooltip')).toBeInTheDocument();
			});

			// Query within visible tooltip content (direct child, not hidden a11y copy)
			const visibleTooltip = document.querySelector(
				'.tooltip-content > .phase-node__tooltip'
			) as HTMLElement;
			const tooltip = within(visibleTooltip);

			// Should show inputs but not output
			expect(tooltip.getByText(/Inputs:/)).toBeInTheDocument();
			expect(tooltip.getByText(/TASK_DESCRIPTION/)).toBeInTheDocument();
			expect(tooltip.queryByText(/Output:/)).not.toBeInTheDocument();
		});

		it('shows only output section when inputVariables is empty', async () => {
			const user = userEvent.setup();
			renderPhaseNode(
				createDefaultData({
					inputVariables: [],
					outputVarName: 'SPEC_CONTENT',
				})
			);

			const node = document.querySelector('.phase-node')!;
			await user.hover(node);

			await waitFor(() => {
				expect(screen.getByRole('tooltip')).toBeInTheDocument();
			});

			// Query within visible tooltip content (direct child, not hidden a11y copy)
			const visibleTooltip = document.querySelector(
				'.tooltip-content > .phase-node__tooltip'
			) as HTMLElement;
			const tooltip = within(visibleTooltip);

			// Should show output but not inputs
			expect(tooltip.queryByText(/Inputs:/)).not.toBeInTheDocument();
			expect(tooltip.getByText(/Output:/)).toBeInTheDocument();
			expect(tooltip.getByText(/SPEC_CONTENT/)).toBeInTheDocument();
		});

		it('does not show tooltip when no variables are set', async () => {
			const user = userEvent.setup();
			renderPhaseNode(
				createDefaultData({
					inputVariables: [],
					outputVarName: undefined,
				})
			);

			const node = document.querySelector('.phase-node')!;
			await user.hover(node);

			// Give tooltip time to appear (it shouldn't)
			await new Promise((resolve) => setTimeout(resolve, 100));

			// No tooltip should appear when there's no variable content
			expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
		});

		it('does not show tooltip when inputVariables is undefined', async () => {
			const user = userEvent.setup();
			renderPhaseNode(
				createDefaultData({
					// inputVariables not set (undefined)
					outputVarName: undefined,
				})
			);

			const node = document.querySelector('.phase-node')!;
			await user.hover(node);

			// Give tooltip time to appear (it shouldn't)
			await new Promise((resolve) => setTimeout(resolve, 100));

			// No tooltip should appear
			expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
		});
	});
});
