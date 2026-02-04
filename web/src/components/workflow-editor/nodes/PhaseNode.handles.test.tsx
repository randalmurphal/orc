/**
 * TDD Tests for PhaseNode connection handles
 *
 * TASK-684: Visual editor - add connection handles to phase nodes
 * (Originally from TASK-640, extended for TASK-684 CSS styling)
 *
 * Success Criteria (derived from task description):
 * - SC-1: Target handle rendered for incoming dependency edges
 * - SC-2: Source handle rendered for outgoing dependency edges
 * - SC-3: Target handle positioned on left side (horizontal flow)
 * - SC-4: Source handle positioned on right side (horizontal flow)
 * - SC-5: Handles interactive in edit mode (isConnectable=true)
 * - SC-6: Handles non-interactive in read-only mode (isConnectable=false)
 * - SC-7: Handles styled with CSS (themed, consistent sizing)
 * - SC-8: Handles present across all node states
 */

import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { ReactFlowProvider } from '@xyflow/react';
import { PhaseNode } from './PhaseNode';
import type { PhaseNodeData } from './index';
import { GateType } from '@/gen/orc/v1/workflow_pb';

// React Flow Handle components require ReactFlowProvider context
function renderPhaseNode(
	data: PhaseNodeData,
	opts: { selected?: boolean; isConnectable?: boolean } = {}
) {
	return render(
		<ReactFlowProvider>
			<PhaseNode
				id="phase-1"
				type="phase"
				data={data}
				selected={opts.selected ?? false}
				isConnectable={opts.isConnectable ?? true}
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

function createDefaultData(overrides: Partial<PhaseNodeData> = {}): PhaseNodeData {
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
describe('PhaseNode - Connection Handles (TASK-684)', () => {
	describe('handle presence and position', () => {
		it('renders target handle on left side', () => {
			const { container } = renderPhaseNode(createDefaultData());

			const targetHandle = container.querySelector(
				'.react-flow__handle-left[data-handlepos="left"]'
			);
			expect(targetHandle).not.toBeNull();
		});

		it('renders source handle on right side', () => {
			const { container } = renderPhaseNode(createDefaultData());

			const sourceHandle = container.querySelector(
				'.react-flow__handle-right[data-handlepos="right"]'
			);
			expect(sourceHandle).not.toBeNull();
		});

		it('target handle has type="target"', () => {
			const { container } = renderPhaseNode(createDefaultData());

			const targetHandle = container.querySelector('.react-flow__handle-left');
			expect(targetHandle).not.toBeNull();
			// React Flow sets data-handletype on Handle components
			expect(targetHandle?.getAttribute('data-handletype')).toBe('target');
		});

		it('source handle has type="source"', () => {
			const { container } = renderPhaseNode(createDefaultData());

			const sourceHandle = container.querySelector('.react-flow__handle-right');
			expect(sourceHandle).not.toBeNull();
			expect(sourceHandle?.getAttribute('data-handletype')).toBe('source');
		});

		it('renders exactly 2 handles (one source, one target)', () => {
			const { container } = renderPhaseNode(createDefaultData());

			const handles = container.querySelectorAll('.react-flow__handle');
			expect(handles.length).toBe(2);
		});
	});

	describe('handle interactivity in edit mode', () => {
		it('handles are connectable when isConnectable=true', () => {
			const { container } = renderPhaseNode(createDefaultData(), {
				isConnectable: true,
			});

			const handles = container.querySelectorAll('.react-flow__handle');
			handles.forEach((handle) => {
				// Connectable handles have the connectable class
				expect(handle.classList.contains('connectable')).toBe(true);
			});
		});

		it('handles are visible in edit mode', () => {
			const { container } = renderPhaseNode(createDefaultData(), {
				isConnectable: true,
			});

			const handles = container.querySelectorAll('.react-flow__handle');
			handles.forEach((handle) => {
				// Handles should not have visibility:hidden or display:none
				const style = window.getComputedStyle(handle as Element);
				expect(style.visibility).not.toBe('hidden');
				expect(style.display).not.toBe('none');
			});
		});

		it('source handle allows drag to initiate connection', () => {
			const { container } = renderPhaseNode(createDefaultData(), {
				isConnectable: true,
			});

			const sourceHandle = container.querySelector('.react-flow__handle-right');
			expect(sourceHandle).not.toBeNull();

			// Source handles should be draggable for connection creation
			// React Flow handles this internally; we verify the element exists and is connectable
			expect(sourceHandle?.classList.contains('source')).toBe(true);
		});

		it('target handle accepts incoming connections', () => {
			const { container } = renderPhaseNode(createDefaultData(), {
				isConnectable: true,
			});

			const targetHandle = container.querySelector('.react-flow__handle-left');
			expect(targetHandle).not.toBeNull();
			expect(targetHandle?.classList.contains('target')).toBe(true);
		});
	});

	describe('read-only mode', () => {
		it('handles are not connectable when isConnectable=false', () => {
			const { container } = renderPhaseNode(createDefaultData(), {
				isConnectable: false,
			});

			const handles = container.querySelectorAll('.react-flow__handle');
			handles.forEach((handle) => {
				// Non-connectable handles should not have connectable class
				expect(handle.classList.contains('connectable')).toBe(false);
			});
		});

		it('handles are still visible in read-only mode (for viewing connections)', () => {
			const { container } = renderPhaseNode(createDefaultData(), {
				isConnectable: false,
			});

			const handles = container.querySelectorAll('.react-flow__handle');
			// Handles should exist even when not connectable
			expect(handles.length).toBe(2);
		});
	});

	describe('handle styling (SC-7)', () => {
		it('both handles have custom class for themed CSS styling', () => {
			const { container } = renderPhaseNode(createDefaultData());

			const handles = container.querySelectorAll('.react-flow__handle');
			expect(handles.length).toBe(2);
			handles.forEach((handle) => {
				// Handles must have a custom class for themed styling
				// (small circles, visible on hover, themed colors)
				expect(handle.classList.contains('phase-node__handle')).toBe(true);
			});
		});

		it('target handle has custom styling class', () => {
			const { container } = renderPhaseNode(createDefaultData());

			const targetHandle = container.querySelector('.react-flow__handle-left');
			expect(targetHandle).not.toBeNull();
			expect(targetHandle?.classList.contains('phase-node__handle')).toBe(
				true
			);
		});

		it('source handle has custom styling class', () => {
			const { container } = renderPhaseNode(createDefaultData());

			const sourceHandle = container.querySelector(
				'.react-flow__handle-right'
			);
			expect(sourceHandle).not.toBeNull();
			expect(sourceHandle?.classList.contains('phase-node__handle')).toBe(
				true
			);
		});

		it('handles are scoped within phase-node for CSS targeting', () => {
			const { container } = renderPhaseNode(createDefaultData());

			const handles = container.querySelectorAll('.react-flow__handle');
			handles.forEach((handle) => {
				// Handles must be inside .phase-node for CSS selectors to work
				expect(handle.closest('.phase-node')).not.toBeNull();
			});
		});
	});

	describe('handle position with different node states', () => {
		it('handles maintain position when node is selected', () => {
			const { container } = renderPhaseNode(createDefaultData(), {
				selected: true,
			});

			const targetHandle = container.querySelector('.react-flow__handle-left');
			const sourceHandle = container.querySelector('.react-flow__handle-right');

			expect(targetHandle).not.toBeNull();
			expect(sourceHandle).not.toBeNull();
		});

		it('handles maintain position with status overlay', () => {
			const { container } = renderPhaseNode(
				createDefaultData({ status: 'running' as any })
			);

			const handles = container.querySelectorAll('.react-flow__handle');
			expect(handles.length).toBe(2);
		});

		it('handles maintain position with all badges displayed', () => {
			const { container } = renderPhaseNode(
				createDefaultData({
					gateType: GateType.HUMAN,
					agentId: 'custom-agent',
				})
			);

			const handles = container.querySelectorAll('.react-flow__handle');
			expect(handles.length).toBe(2);
		});
	});
});
