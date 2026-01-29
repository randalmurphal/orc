import { describe, it, expect, beforeAll } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ReactFlowProvider } from '@xyflow/react';
import { PhaseNode } from './PhaseNode';
import type { PhaseNodeData } from './index';
import { GateType } from '@/gen/orc/v1/workflow_pb';

// React Flow Handle components require ReactFlowProvider context
function renderPhaseNode(
	data: PhaseNodeData,
	opts: { selected?: boolean } = {}
) {
	return render(
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
		maxIterations: 3,
		...overrides,
	};
}

// Mock IntersectionObserver for React Flow internals
beforeAll(() => {
	class MockIntersectionObserver {
		observe() {}
		unobserve() {}
		disconnect() {}
	}
	Object.defineProperty(window, 'IntersectionObserver', {
		value: MockIntersectionObserver,
		writable: true,
	});
});

describe('PhaseNode', () => {
	describe('SC-1: template name and ID display', () => {
		it('renders template name as header text', () => {
			renderPhaseNode(createDefaultData({ templateName: 'Specification' }));

			const nameEl = document.querySelector('.phase-node-name');
			expect(nameEl).not.toBeNull();
			expect(nameEl!.textContent).toBe('Specification');
		});

		it('renders phaseTemplateId in monospace as subtitle', () => {
			renderPhaseNode(
				createDefaultData({
					templateName: 'Specification',
					phaseTemplateId: 'spec',
				})
			);

			const templateIdEl = document.querySelector(
				'.phase-node-template-id'
			);
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

			const nameEl = document.querySelector('.phase-node-name');
			expect(nameEl).not.toBeNull();
			expect(nameEl!.textContent).toBe('tdd_write');
		});
	});

	describe('SC-2: sequence number badge', () => {
		it('displays sequence number in badge', () => {
			renderPhaseNode(createDefaultData({ sequence: 3 }));

			const seqBadge = document.querySelector('.phase-node-sequence');
			expect(seqBadge).not.toBeNull();
			expect(seqBadge!.textContent).toBe('3');
		});

		it('displays sequence 1 for first phase', () => {
			renderPhaseNode(createDefaultData({ sequence: 1 }));

			const seqBadge = document.querySelector('.phase-node-sequence');
			expect(seqBadge).not.toBeNull();
			expect(seqBadge!.textContent).toBe('1');
		});
	});

	describe('SC-3: conditional metadata badges', () => {
		it('renders gate badge with cyan styling when gateType is HUMAN', () => {
			renderPhaseNode(
				createDefaultData({ gateType: GateType.HUMAN })
			);

			const gateBadge = document.querySelector(
				'.phase-node-badge--gate'
			);
			expect(gateBadge).not.toBeNull();
		});

		it('renders iterations badge with orange styling when maxIterations is non-default', () => {
			renderPhaseNode(
				createDefaultData({ maxIterations: 5 })
			);

			const iterBadge = document.querySelector(
				'.phase-node-badge--iterations'
			);
			expect(iterBadge).not.toBeNull();
			expect(iterBadge!.textContent).toContain('5');
		});

		it('renders model badge with purple styling when modelOverride is set', () => {
			renderPhaseNode(
				createDefaultData({ modelOverride: 'opus' })
			);

			const modelBadge = document.querySelector(
				'.phase-node-badge--model'
			);
			expect(modelBadge).not.toBeNull();
			expect(modelBadge!.textContent).toContain('opus');
		});

		it('renders all 3 badges when all overrides are present', () => {
			renderPhaseNode(
				createDefaultData({
					gateType: GateType.HUMAN,
					maxIterations: 5,
					modelOverride: 'opus',
				})
			);

			expect(
				document.querySelector('.phase-node-badge--gate')
			).not.toBeNull();
			expect(
				document.querySelector('.phase-node-badge--iterations')
			).not.toBeNull();
			expect(
				document.querySelector('.phase-node-badge--model')
			).not.toBeNull();
		});

		it('renders no badges when data has defaults (AUTO gate, no model override)', () => {
			renderPhaseNode(
				createDefaultData({
					gateType: GateType.AUTO,
					maxIterations: 1,
					modelOverride: undefined,
				})
			);

			expect(
				document.querySelector('.phase-node-badge--gate')
			).toBeNull();
			expect(
				document.querySelector('.phase-node-badge--iterations')
			).toBeNull();
			expect(
				document.querySelector('.phase-node-badge--model')
			).toBeNull();
		});

		it('does not render gate badge for AUTO gate type', () => {
			renderPhaseNode(
				createDefaultData({ gateType: GateType.AUTO })
			);

			expect(
				document.querySelector('.phase-node-badge--gate')
			).toBeNull();
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

		it('shows $0.00 when costUsd is 0', () => {
			renderPhaseNode(
				createDefaultData({
					status: 'running' as any,
					costUsd: 0,
				})
			);

			expect(screen.getByText(/\$0\.00/)).toBeTruthy();
		});
	});

	describe('SC-9: selected state', () => {
		it('adds selected class when node is selected', () => {
			renderPhaseNode(createDefaultData(), { selected: true });

			const node = document.querySelector('.phase-node');
			expect(node).not.toBeNull();
			expect(node!.classList.contains('selected')).toBe(true);
		});

		it('does not add selected class when node is not selected', () => {
			renderPhaseNode(createDefaultData(), { selected: false });

			const node = document.querySelector('.phase-node');
			expect(node).not.toBeNull();
			expect(node!.classList.contains('selected')).toBe(false);
		});
	});

	describe('edge cases', () => {
		it('handles very long template name with truncation', () => {
			const longName =
				'This Is A Very Long Phase Template Name That Should Be Truncated';
			renderPhaseNode(createDefaultData({ templateName: longName }));

			const nameEl = document.querySelector('.phase-node-name');
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

		it('renders gate badge for SKIP gate type', () => {
			renderPhaseNode(
				createDefaultData({ gateType: GateType.SKIP })
			);

			const gateBadge = document.querySelector(
				'.phase-node-badge--gate'
			);
			expect(gateBadge).not.toBeNull();
		});
	});
});
