import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PhaseInspector } from './PhaseInspector';
import {
	createMockWorkflowPhase,
	createMockWorkflowWithDetails,
	createMockPhaseTemplate,
	createMockWorkflow,
} from '@/test/factories';
import { PromptSource, GateType } from '@/gen/orc/v1/workflow_pb';

// Mock window.matchMedia for responsive tests
const mockMatchMedia = (matches: boolean) => {
	Object.defineProperty(window, 'matchMedia', {
		writable: true,
		value: vi.fn().mockImplementation((query) => ({
			matches,
			media: query,
			onchange: null,
			addListener: vi.fn(),
			removeListener: vi.fn(),
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
			dispatchEvent: vi.fn(),
		})),
	});
};

// Mock ResizeObserver
const mockResizeObserver = vi.fn();
Object.defineProperty(window, 'ResizeObserver', {
	writable: true,
	value: vi.fn().mockImplementation(() => ({
		observe: mockResizeObserver,
		unobserve: vi.fn(),
		disconnect: vi.fn(),
	})),
});

vi.mock('@/lib/client', () => ({
	workflowClient: {
		updatePhase: vi.fn().mockResolvedValue({}),
	},
	configClient: {
		listAgents: vi.fn().mockResolvedValue({ agents: [] }),
		listHooks: vi.fn().mockResolvedValue({ hooks: [] }),
		listSkills: vi.fn().mockResolvedValue({ skills: [] }),
	},
	mcpClient: {
		listMCPServers: vi.fn().mockResolvedValue({ servers: [] }),
	},
}));

describe('PhaseInspector - Responsive Design and Edge Cases (TDD)', () => {
	const mockUser = userEvent.setup();

	const mockPhase = createMockWorkflowPhase({
		id: 1,
		sequence: 1,
		phaseTemplateId: 'spec',
		template: createMockPhaseTemplate({
			id: 'spec',
			name: 'Specification',
			isBuiltin: false,
			agentId: 'default-agent',
			maxIterations: 3,
			inputVariables: [],
			promptSource: PromptSource.EMBEDDED,
			promptContent: 'Write a spec',
			gateType: GateType.AUTO,
		}),
	});

	const mockWorkflowDetails = createMockWorkflowWithDetails({
		workflow: createMockWorkflow({ id: 'test-workflow', name: 'Test Workflow' }),
		phases: [mockPhase],
		variables: [],
	});

	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	// Tests for SC-26: Responsive design on mobile breakpoints
	describe('Mobile Responsive Design (SC-26)', () => {
		it('adapts layout for mobile viewport (< 640px)', async () => {
			mockMatchMedia(true); // Mobile breakpoint matches

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const inspector = screen.getByTestId('phase-inspector');

			// Should have mobile-specific classes
			expect(inspector).toHaveClass('phase-inspector--mobile');

			// Sections should stack vertically on mobile
			const sections = screen.getAllByRole('button', {
				name: /^(sub-agents|prompt|data flow|environment|advanced)$/i,
			});

			sections.forEach((section) => {
				expect(section.parentElement).toHaveClass('section--mobile-stack');
			});
		});

		it('maintains all functionality on mobile without loss of features', async () => {
			mockMatchMedia(true);

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// All form controls should remain accessible
			expect(screen.getByDisplayValue('Specification')).toBeInTheDocument();
			expect(screen.getByLabelText(/executor/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/model/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/max iterations/i)).toBeInTheDocument();

			// Collapsible sections should still work
			const subAgentsHeader = screen.getByRole('button', { name: /sub-agents/i });
			await mockUser.click(subAgentsHeader);

			const subAgentsContent = screen.getByTestId('sub-agents-content');
			expect(subAgentsContent).toBeVisible();
		});

		it('uses touch-friendly controls on mobile devices', async () => {
			mockMatchMedia(true);

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Section headers should have touch-friendly styling
			const sectionHeaders = screen.getAllByRole('button', {
				name: /^(sub-agents|prompt|data flow|environment|advanced)$/i,
			});

			sectionHeaders.forEach((header) => {
				expect(header).toHaveClass('touch-friendly');
				// Should have larger touch targets (44px minimum)
				const styles = window.getComputedStyle(header);
				expect(parseInt(styles.minHeight)).toBeGreaterThanOrEqual(44);
			});

			// Form controls should be touch-friendly
			const formControls = [
				screen.getByDisplayValue('Specification'),
				screen.getByLabelText(/executor/i),
				screen.getByLabelText(/model/i),
				screen.getByLabelText(/max iterations/i),
			];

			formControls.forEach((control) => {
				expect(control).toHaveClass('touch-friendly');
			});
		});

		it('adjusts spacing and layout for smaller screens', async () => {
			mockMatchMedia(true);

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const inspector = screen.getByTestId('phase-inspector');

			// Should have mobile-specific spacing
			expect(inspector).toHaveClass('inspector--compact-spacing');

			// Always-visible section should stack vertically on mobile
			const alwaysVisibleSection = screen.getByTestId('always-visible-section');
			expect(alwaysVisibleSection).toHaveClass('always-visible--mobile-stack');
		});
	});

	describe('Desktop Responsive Design', () => {
		it('maintains horizontal layout on desktop viewport (>= 640px)', async () => {
			mockMatchMedia(false); // Desktop breakpoint

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const inspector = screen.getByTestId('phase-inspector');

			// Should not have mobile classes
			expect(inspector).not.toHaveClass('phase-inspector--mobile');

			// Always-visible section should be horizontal on desktop
			const alwaysVisibleSection = screen.getByTestId('always-visible-section');
			expect(alwaysVisibleSection).toHaveClass('always-visible--horizontal');
		});

		it('uses optimal spacing for larger screens', async () => {
			mockMatchMedia(false);

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const inspector = screen.getByTestId('phase-inspector');
			expect(inspector).toHaveClass('inspector--desktop-spacing');
		});
	});

	// Edge Cases and Error Scenarios
	describe('Edge Cases', () => {
		it('handles empty sub-agents list gracefully', async () => {
			const emptyPhase = {
				...mockPhase,
				subAgentsOverride: [],
			};

			render(
				<PhaseInspector
					phase={emptyPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const subAgentsHeader = screen.getByRole('button', { name: /sub-agents/i });
			await mockUser.click(subAgentsHeader);

			// Should show "None assigned" placeholder
			expect(screen.getByText(/none assigned/i)).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /add agent/i })).toBeInTheDocument();
		});

		it('handles very long phase names with proper truncation', async () => {
			const longNamePhase = createMockWorkflowPhase({
				id: mockPhase.id,
				sequence: mockPhase.sequence,
				phaseTemplateId: mockPhase.phaseTemplateId,
				template: createMockPhaseTemplate({
					id: 'spec',
					name: 'This is an extremely long phase name that should be truncated to prevent layout issues and maintain proper UI proportions in the inspector panel',
				}),
			});

			render(
				<PhaseInspector
					phase={longNamePhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const nameInput = screen.getByDisplayValue(/extremely long phase name/);
			const container = nameInput.parentElement;

			// Should have truncation styling
			expect(container).toHaveClass('phase-name--truncated');

			// Should show full text in tooltip
			expect(nameInput).toHaveAttribute(
				'title',
				expect.stringContaining('extremely long phase name')
			);
		});

		it('handles network errors during agent loading', async () => {
			const { configClient } = await import('@/lib/client');
			(configClient.listAgents as any).mockRejectedValue(new Error('Network error'));

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/failed to load agents/i)).toBeInTheDocument();
			});

			// Should still allow other operations
			const subAgentsHeader = screen.getByRole('button', { name: /sub-agents/i });
			await mockUser.click(subAgentsHeader);
			expect(screen.getByTestId('sub-agents-content')).toBeVisible();
		});

		it('handles built-in phase template readonly state', async () => {
			const builtinPhase = createMockWorkflowPhase({
				id: mockPhase.id,
				sequence: mockPhase.sequence,
				phaseTemplateId: mockPhase.phaseTemplateId,
				template: createMockPhaseTemplate({
					id: 'spec',
					isBuiltin: true,
				}),
			});

			render(
				<PhaseInspector
					phase={builtinPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={true}
				/>
			);

			// Should show readonly notice
			expect(screen.getByText(/built-in template/i)).toBeInTheDocument();

			// All form fields should be disabled
			expect(screen.getByDisplayValue('Specification')).toBeDisabled();
			expect(screen.getByLabelText(/executor/i)).toBeDisabled();
			expect(screen.getByLabelText(/model/i)).toBeDisabled();
			expect(screen.getByLabelText(/max iterations/i)).toBeDisabled();

			// Sections should still be expandable for viewing
			const subAgentsHeader = screen.getByRole('button', { name: /sub-agents/i });
			await mockUser.click(subAgentsHeader);
			expect(screen.getByTestId('sub-agents-content')).toBeVisible();
		});

		it('handles rapid window resize events efficiently', async () => {
			const { rerender } = render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Start desktop
			mockMatchMedia(false);
			rerender(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Switch to mobile
			mockMatchMedia(true);
			rerender(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Switch back to desktop
			mockMatchMedia(false);
			rerender(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Should handle rapid changes without errors
			const inspector = screen.getByTestId('phase-inspector');
			expect(inspector).toBeInTheDocument();
			expect(inspector).not.toHaveClass('phase-inspector--mobile');
		});

		it('maintains scroll position during layout changes', async () => {
			const mockScrollTo = vi.fn();
			Object.defineProperty(window, 'scrollTo', { value: mockScrollTo });

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const inspector = screen.getByTestId('phase-inspector');

			// Simulate scrolled position
			Object.defineProperty(inspector, 'scrollTop', { value: 300, writable: true });

			// Expand a section (layout change)
			const envHeader = screen.getByRole('button', { name: /environment/i });
			await mockUser.click(envHeader);

			// Scroll position should be preserved
			expect(inspector.scrollTop).toBe(300);
		});

		it('handles missing template data gracefully', async () => {
			const phaseWithoutTemplate = createMockWorkflowPhase({
				...mockPhase,
				template: undefined,
			});

			render(
				<PhaseInspector
					phase={phaseWithoutTemplate}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Should show template not found state
			expect(screen.getByText(/template not found/i)).toBeInTheDocument();
			expect(screen.getByText(mockPhase.phaseTemplateId)).toBeInTheDocument();
		});

		it('handles empty workflow details gracefully', async () => {
			const emptyWorkflowDetails = createMockWorkflowWithDetails({
				workflow: undefined,
				phases: [],
				variables: [],
			});

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={emptyWorkflowDetails}
					readOnly={false}
				/>
			);

			// Should show loading state
			expect(screen.getByText(/loading/i)).toBeInTheDocument();
		});

		it('handles null phase gracefully', async () => {
			render(
				<PhaseInspector
					phase={null}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Should render nothing or empty state
			const inspector = screen.queryByTestId('phase-inspector');
			expect(inspector).not.toBeInTheDocument();
		});
	});

	describe('Performance Edge Cases', () => {
		it('debounces resize events to prevent excessive re-renders', async () => {
			vi.useFakeTimers();

			const renderSpy = vi.fn();
			const TestWrapper = () => {
				renderSpy();
				return (
					<PhaseInspector
						phase={mockPhase}
						workflowDetails={mockWorkflowDetails}
						readOnly={false}
					/>
				);
			};

			render(<TestWrapper />);

			// Initial render
			expect(renderSpy).toHaveBeenCalledTimes(1);

			// Simulate rapid resize events
			for (let i = 0; i < 10; i++) {
				fireEvent(window, new Event('resize'));
				vi.advanceTimersByTime(10);
			}

			// Should not cause excessive re-renders
			expect(renderSpy).toHaveBeenCalledTimes(1);

			vi.useRealTimers();
		});

		it('cleans up event listeners on unmount', async () => {
			const removeEventListenerSpy = vi.spyOn(window, 'removeEventListener');

			const { unmount } = render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			unmount();

			// Should clean up resize listener
			expect(removeEventListenerSpy).toHaveBeenCalledWith('resize', expect.any(Function));
		});
	});
});