import { describe, it, expect, beforeAll } from 'vitest';
import { render } from '@testing-library/react';
import { ReactFlowProvider } from '@xyflow/react';
import { StartEndNode } from './StartEndNode';
import type { StartEndNodeData } from './index';

function renderStartEndNode(data: StartEndNodeData) {
	return render(
		<ReactFlowProvider>
			<StartEndNode
				id="__start__"
				type="startEnd"
				data={data}
				selected={false}
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

describe('StartEndNode', () => {
	describe('SC-5: variant rendering', () => {
		it('renders "Start" label for start variant', () => {
			const { container } = renderStartEndNode({
				variant: 'start',
				label: 'Start',
			});

			const node = container.querySelector('.start-end-node');
			expect(node).not.toBeNull();
			expect(node!.textContent).toContain('Start');
		});

		it('renders "End" label for end variant', () => {
			const { container } = renderStartEndNode({
				variant: 'end',
				label: 'End',
			});

			const node = container.querySelector('.start-end-node');
			expect(node).not.toBeNull();
			expect(node!.textContent).toContain('End');
		});

		it('applies start variant CSS class with green accent', () => {
			const { container } = renderStartEndNode({
				variant: 'start',
				label: 'Start',
			});

			const node = container.querySelector('.start-end-node');
			expect(node).not.toBeNull();
			expect(
				node!.classList.contains('start-end-node--start')
			).toBe(true);
		});

		it('applies end variant CSS class with primary accent', () => {
			const { container } = renderStartEndNode({
				variant: 'end',
				label: 'End',
			});

			const node = container.querySelector('.start-end-node');
			expect(node).not.toBeNull();
			expect(
				node!.classList.contains('start-end-node--end')
			).toBe(true);
		});
	});

	describe('SC-6: handle configuration', () => {
		it('start variant has only source handle (right side)', () => {
			const { container } = renderStartEndNode({
				variant: 'start',
				label: 'Start',
			});

			const sourceHandle = container.querySelector(
				'.react-flow__handle-right'
			);
			const targetHandle = container.querySelector(
				'.react-flow__handle-left'
			);

			expect(sourceHandle).not.toBeNull();
			expect(targetHandle).toBeNull();
		});

		it('end variant has only target handle (left side)', () => {
			const { container } = renderStartEndNode({
				variant: 'end',
				label: 'End',
			});

			const targetHandle = container.querySelector(
				'.react-flow__handle-left'
			);
			const sourceHandle = container.querySelector(
				'.react-flow__handle-right'
			);

			expect(targetHandle).not.toBeNull();
			expect(sourceHandle).toBeNull();
		});
	});
});
