import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { DependencyGraph } from './DependencyGraph';
import type { DependencyGraphNode, DependencyGraphEdge } from '@/lib/api';

// Mock useNavigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

describe('DependencyGraph', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	const renderGraph = (
		nodes: DependencyGraphNode[],
		edges: DependencyGraphEdge[],
		onNodeClick?: (nodeId: string) => void
	) => {
		return render(
			<MemoryRouter>
				<DependencyGraph nodes={nodes} edges={edges} onNodeClick={onNodeClick} />
			</MemoryRouter>
		);
	};

	describe('empty state', () => {
		it('shows empty message when no nodes', () => {
			renderGraph([], []);
			expect(screen.getByText('No tasks to display')).toBeInTheDocument();
		});
	});

	describe('with nodes', () => {
		const nodes: DependencyGraphNode[] = [
			{ id: 'TASK-001', title: 'First Task', status: 'done' },
			{ id: 'TASK-002', title: 'Second Task', status: 'running' },
			{ id: 'TASK-003', title: 'Third Task', status: 'blocked' },
		];

		const edges: DependencyGraphEdge[] = [
			{ from: 'TASK-001', to: 'TASK-002' },
			{ from: 'TASK-002', to: 'TASK-003' },
		];

		it('renders all nodes', () => {
			renderGraph(nodes, edges);
			expect(screen.getByText('TASK-001')).toBeInTheDocument();
			expect(screen.getByText('TASK-002')).toBeInTheDocument();
			expect(screen.getByText('TASK-003')).toBeInTheDocument();
		});

		it('shows node status labels', () => {
			renderGraph(nodes, edges);
			expect(screen.getByText('(done)')).toBeInTheDocument();
			expect(screen.getByText('(running)')).toBeInTheDocument();
			expect(screen.getByText('(blocked)')).toBeInTheDocument();
		});

		it('renders SVG with nodes and edges', () => {
			const { container } = renderGraph(nodes, edges);
			// Check SVG is rendered
			expect(container.querySelector('svg.graph-svg')).toBeInTheDocument();
			// Check nodes are rendered as groups
			expect(container.querySelectorAll('g.node')).toHaveLength(3);
			// Check edges are rendered as paths
			expect(container.querySelectorAll('path.edge')).toHaveLength(2);
		});
	});

	describe('toolbar', () => {
		const nodes: DependencyGraphNode[] = [
			{ id: 'TASK-001', title: 'Test', status: 'done' },
		];

		it('renders zoom in button', () => {
			renderGraph(nodes, []);
			expect(screen.getByTitle('Zoom In')).toBeInTheDocument();
		});

		it('renders zoom out button', () => {
			renderGraph(nodes, []);
			expect(screen.getByTitle('Zoom Out')).toBeInTheDocument();
		});

		it('renders fit to view button', () => {
			renderGraph(nodes, []);
			expect(screen.getByTitle('Fit to View')).toBeInTheDocument();
		});

		it('renders export PNG button', () => {
			renderGraph(nodes, []);
			expect(screen.getByTitle('Export PNG')).toBeInTheDocument();
		});
	});

	describe('legend', () => {
		it('displays legend with status colors', () => {
			renderGraph([], []);
			expect(screen.getByText('Legend:')).toBeInTheDocument();
			expect(screen.getByText('done')).toBeInTheDocument();
			expect(screen.getByText('ready')).toBeInTheDocument();
			expect(screen.getByText('blocked')).toBeInTheDocument();
			expect(screen.getByText('running')).toBeInTheDocument();
		});
	});

	describe('node interaction', () => {
		const nodes: DependencyGraphNode[] = [
			{ id: 'TASK-001', title: 'Test Task', status: 'done' },
		];

		it('navigates to task on click by default', () => {
			const { container } = renderGraph(nodes, []);
			const node = container.querySelector('g.node');
			expect(node).toBeInTheDocument();
			fireEvent.click(node!);
			expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-001');
		});

		it('calls custom onNodeClick handler when provided', () => {
			const onNodeClick = vi.fn();
			const { container } = renderGraph(nodes, [], onNodeClick);
			const node = container.querySelector('g.node');
			fireEvent.click(node!);
			expect(onNodeClick).toHaveBeenCalledWith('TASK-001');
			expect(mockNavigate).not.toHaveBeenCalled();
		});

		it('supports keyboard navigation (Enter key)', () => {
			const { container } = renderGraph(nodes, []);
			const node = container.querySelector('g.node');
			fireEvent.keyDown(node!, { key: 'Enter' });
			expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-001');
		});

		it('has correct aria attributes', () => {
			const { container } = renderGraph(nodes, []);
			const node = container.querySelector('g.node');
			expect(node).toHaveAttribute('role', 'button');
			expect(node).toHaveAttribute('tabindex', '0');
			expect(node).toHaveAttribute(
				'aria-label',
				'TASK-001: Test Task (done)'
			);
		});
	});

	describe('tooltip', () => {
		const nodes: DependencyGraphNode[] = [
			{ id: 'TASK-001', title: 'Test Task Title', status: 'running' },
		];

		it('shows tooltip on node hover', () => {
			const { container } = renderGraph(nodes, []);
			const node = container.querySelector('g.node');

			// Initially no tooltip
			expect(screen.queryByText('Test Task Title')).not.toBeInTheDocument();

			// Hover shows tooltip
			fireEvent.mouseEnter(node!, { clientX: 100, clientY: 100 });
			expect(screen.getByText('Test Task Title')).toBeInTheDocument();
			expect(screen.getByText('TASK-001 - running')).toBeInTheDocument();
		});

		it('hides tooltip on mouse leave', () => {
			const { container } = renderGraph(nodes, []);
			const node = container.querySelector('g.node');

			fireEvent.mouseEnter(node!, { clientX: 100, clientY: 100 });
			expect(screen.getByText('Test Task Title')).toBeInTheDocument();

			fireEvent.mouseLeave(node!);
			// Use queryByText to check it's gone
			expect(screen.queryByText('Test Task Title')).not.toBeInTheDocument();
		});
	});

	describe('status styling', () => {
		it('applies correct CSS class for running status', () => {
			const nodes: DependencyGraphNode[] = [
				{ id: 'TASK-001', title: 'Test', status: 'running' },
			];
			const { container } = renderGraph(nodes, []);
			expect(container.querySelector('g.node-running')).toBeInTheDocument();
		});

		it('applies correct CSS class for done status', () => {
			const nodes: DependencyGraphNode[] = [
				{ id: 'TASK-001', title: 'Test', status: 'done' },
			];
			const { container } = renderGraph(nodes, []);
			expect(container.querySelector('g.node-done')).toBeInTheDocument();
		});

		it('applies correct CSS class for blocked status', () => {
			const nodes: DependencyGraphNode[] = [
				{ id: 'TASK-001', title: 'Test', status: 'blocked' },
			];
			const { container } = renderGraph(nodes, []);
			expect(container.querySelector('g.node-blocked')).toBeInTheDocument();
		});
	});
});
