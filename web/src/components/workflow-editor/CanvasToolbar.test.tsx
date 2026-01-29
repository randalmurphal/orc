/**
 * TDD Tests for CanvasToolbar (SC-12)
 *
 * Tests for TASK-640: Canvas toolbar controls
 *
 * Success Criteria Coverage:
 * - SC-12: Canvas toolbar has Fit View, Reset Layout, and Zoom controls
 *
 * Behaviors:
 * - Fit View: All nodes visible in viewport
 * - Reset Layout: Clears stored positions, re-runs dagre
 * - Zoom: +/- zoom level controls
 */

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { CanvasToolbar } from './CanvasToolbar';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { workflowClient } from '@/lib/client';
import {
	createMockWorkflow,
	createMockWorkflowWithDetails,
	createMockWorkflowPhase,
	createMockSaveWorkflowLayoutResponse,
	createMockGetWorkflowResponse,
} from '@/test/factories';

// Mock the workflow client
vi.mock('@/lib/client', () => ({
	workflowClient: {
		saveWorkflowLayout: vi.fn(),
		getWorkflow: vi.fn(),
	},
}));

// Mock React Flow hooks
const mockFitView = vi.fn();
const mockZoomIn = vi.fn();
const mockZoomOut = vi.fn();

vi.mock('@xyflow/react', async () => {
	const actual = await vi.importActual('@xyflow/react');
	return {
		...actual,
		useReactFlow: () => ({
			fitView: mockFitView,
			zoomIn: mockZoomIn,
			zoomOut: mockZoomOut,
			getZoom: () => 1,
		}),
	};
});

// Mock IntersectionObserver for React Flow
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

function loadCustomWorkflow() {
	const details = createMockWorkflowWithDetails({
		workflow: createMockWorkflow({ id: 'custom-wf', isBuiltin: false }),
		phases: [
			createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
			createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
		],
	});
	useWorkflowEditorStore.getState().loadFromWorkflow(details);
	return details;
}

function loadBuiltinWorkflow() {
	const details = createMockWorkflowWithDetails({
		workflow: createMockWorkflow({ id: 'medium', isBuiltin: true }),
		phases: [
			createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
		],
	});
	useWorkflowEditorStore.getState().loadFromWorkflow(details);
	return details;
}

describe('CanvasToolbar', () => {
	beforeEach(() => {
		useWorkflowEditorStore.getState().reset();
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-12: Toolbar controls exist', () => {
		it('renders Fit View button', () => {
			loadCustomWorkflow();

			render(<CanvasToolbar />);

			expect(screen.getByRole('button', { name: /fit view/i })).toBeInTheDocument();
		});

		it('renders Reset Layout button', () => {
			loadCustomWorkflow();

			render(<CanvasToolbar />);

			expect(screen.getByRole('button', { name: /reset layout/i })).toBeInTheDocument();
		});

		it('renders Zoom In button', () => {
			loadCustomWorkflow();

			render(<CanvasToolbar />);

			expect(screen.getByRole('button', { name: /zoom in/i })).toBeInTheDocument();
		});

		it('renders Zoom Out button', () => {
			loadCustomWorkflow();

			render(<CanvasToolbar />);

			expect(screen.getByRole('button', { name: /zoom out/i })).toBeInTheDocument();
		});
	});

	describe('Fit View behavior', () => {
		it('calls React Flow fitView when clicked', () => {
			loadCustomWorkflow();

			render(<CanvasToolbar />);

			const fitViewBtn = screen.getByRole('button', { name: /fit view/i });
			fireEvent.click(fitViewBtn);

			expect(mockFitView).toHaveBeenCalledTimes(1);
		});

		it('calls fitView with padding option for better visibility', () => {
			loadCustomWorkflow();

			render(<CanvasToolbar />);

			fireEvent.click(screen.getByRole('button', { name: /fit view/i }));

			expect(mockFitView).toHaveBeenCalledWith(
				expect.objectContaining({
					padding: expect.any(Number),
				})
			);
		});
	});

	describe('Reset Layout behavior', () => {
		it('clears stored positions and re-runs dagre layout', async () => {
			loadCustomWorkflow();
			const mockSaveLayout = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSaveLayout.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));

			render(<CanvasToolbar />);

			const resetBtn = screen.getByRole('button', { name: /reset layout/i });
			fireEvent.click(resetBtn);

			await waitFor(() => {
				// Should save layout with all positions cleared (or call a clear endpoint)
				expect(mockSaveLayout).toHaveBeenCalledWith(
					expect.objectContaining({
						workflowId: 'custom-wf',
						positions: [], // Empty array clears all positions
					})
				);
			});
		});

		it('refreshes workflow data after reset', async () => {
			loadCustomWorkflow();
			const mockSaveLayout = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSaveLayout.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));
			const mockGetWorkflow = vi.mocked(workflowClient.getWorkflow);
			mockGetWorkflow.mockResolvedValue(createMockGetWorkflowResponse(createMockWorkflowWithDetails()));

			render(<CanvasToolbar />);

			fireEvent.click(screen.getByRole('button', { name: /reset layout/i }));

			await waitFor(() => {
				// Should trigger workflow refresh to reload with dagre positions
				expect(mockGetWorkflow).toHaveBeenCalled();
			});
		});

		it('is disabled in read-only mode', () => {
			loadBuiltinWorkflow();

			render(<CanvasToolbar />);

			const resetBtn = screen.getByRole('button', { name: /reset layout/i });
			expect(resetBtn).toBeDisabled();
		});
	});

	describe('Zoom controls behavior', () => {
		it('calls React Flow zoomIn when Zoom In clicked', () => {
			loadCustomWorkflow();

			render(<CanvasToolbar />);

			fireEvent.click(screen.getByRole('button', { name: /zoom in/i }));

			expect(mockZoomIn).toHaveBeenCalledTimes(1);
		});

		it('calls React Flow zoomOut when Zoom Out clicked', () => {
			loadCustomWorkflow();

			render(<CanvasToolbar />);

			fireEvent.click(screen.getByRole('button', { name: /zoom out/i }));

			expect(mockZoomOut).toHaveBeenCalledTimes(1);
		});
	});

	describe('Read-only mode', () => {
		it('allows Fit View and Zoom in read-only mode', () => {
			loadBuiltinWorkflow();

			render(<CanvasToolbar />);

			const fitViewBtn = screen.getByRole('button', { name: /fit view/i });
			const zoomInBtn = screen.getByRole('button', { name: /zoom in/i });
			const zoomOutBtn = screen.getByRole('button', { name: /zoom out/i });

			expect(fitViewBtn).not.toBeDisabled();
			expect(zoomInBtn).not.toBeDisabled();
			expect(zoomOutBtn).not.toBeDisabled();
		});

		it('only disables Reset Layout in read-only mode', () => {
			loadBuiltinWorkflow();

			render(<CanvasToolbar />);

			const resetBtn = screen.getByRole('button', { name: /reset layout/i });
			expect(resetBtn).toBeDisabled();

			// Others should work
			fireEvent.click(screen.getByRole('button', { name: /fit view/i }));
			expect(mockFitView).toHaveBeenCalled();
		});
	});

	describe('Edge case: Reset Layout with no phases', () => {
		it('handles empty workflow without error', async () => {
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'empty-wf', isBuiltin: false }),
				phases: [],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(details);

			const mockSaveLayout = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSaveLayout.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));

			render(<CanvasToolbar />);

			const resetBtn = screen.getByRole('button', { name: /reset layout/i });
			fireEvent.click(resetBtn);

			await waitFor(() => {
				expect(mockSaveLayout).toHaveBeenCalledWith(
					expect.objectContaining({
						positions: [],
					})
				);
			});
		});
	});

	describe('Accessibility', () => {
		it('all buttons have accessible names', () => {
			loadCustomWorkflow();

			render(<CanvasToolbar />);

			const buttons = screen.getAllByRole('button');
			buttons.forEach((btn) => {
				expect(btn).toHaveAccessibleName();
			});
		});

		it('toolbar is keyboard navigable', () => {
			loadCustomWorkflow();

			render(<CanvasToolbar />);

			const fitViewBtn = screen.getByRole('button', { name: /fit view/i });
			fitViewBtn.focus();

			// Tab should move to next button
			fireEvent.keyDown(fitViewBtn, { key: 'Tab' });

			// Buttons should be focusable
			const buttons = screen.getAllByRole('button');
			buttons.forEach((btn) => {
				expect(btn.tabIndex).toBeGreaterThanOrEqual(0);
			});
		});
	});
});
