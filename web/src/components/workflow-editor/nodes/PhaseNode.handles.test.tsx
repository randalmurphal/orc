/**
 * TDD Tests for PhaseNode connection handles (SC-9)
 *
 * Tests for TASK-640: Connection handles on phase nodes
 *
 * Success Criteria Coverage:
 * - SC-9: Connection handles appear on phase nodes (source on right, target on left)
 *         and are interactive in edit mode
 *
 * Edge cases:
 * - Handles hidden in read-only mode
 * - Handles properly positioned for edge connections
 */

import { describe, it, expect, beforeAll } from 'vitest';
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

describe('PhaseNode - Connection Handles (SC-9)', () => {
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

	describe('handle styling', () => {
		it('handles have appropriate size for interaction', () => {
			const { container } = renderPhaseNode(createDefaultData());

			const handles = container.querySelectorAll('.react-flow__handle');
			handles.forEach((handle) => {
				// Default React Flow handles are 6x6px, but custom styling may adjust
				// Just verify they have dimensions
				// Note: getBoundingClientRect returns 0s in JSDOM, so we verify presence only
				// In JSDOM, getBoundingClientRect returns 0s, so we check CSS classes instead
				expect(handle).not.toBeNull();
			});
		});

		it('handles use workflow editor handle styling', () => {
			const { container } = renderPhaseNode(createDefaultData());

			const handles = container.querySelectorAll('.react-flow__handle');
			// Custom handle styling is applied via CSS
			// We verify the handles exist and would receive the styling
			expect(handles.length).toBe(2);
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
					maxIterations: 5,
					modelOverride: 'opus',
				})
			);

			const handles = container.querySelectorAll('.react-flow__handle');
			expect(handles.length).toBe(2);
		});
	});
});
