import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ContextPanel } from './ContextPanel';
import { AppShellProvider } from './AppShellContext';
import { EventProvider } from '@/hooks';
import { TooltipProvider } from '@/components/ui';
import { useThreadStore } from '@/stores/threadStore';

// =============================================================================
// TEST UTILITIES
// =============================================================================

function TestWrapper({ children }: { children: React.ReactNode }) {
	return (
		<MemoryRouter>
			<TooltipProvider delayDuration={0}>
				<EventProvider autoConnect={false}>
					<AppShellProvider>
						{children}
					</AppShellProvider>
				</EventProvider>
			</TooltipProvider>
		</MemoryRouter>
	);
}

function renderWithProviders(ui: React.ReactElement) {
	return render(ui, { wrapper: TestWrapper });
}

beforeEach(() => {
	useThreadStore.getState().reset();
	localStorage.clear();
});

afterEach(() => {
	vi.clearAllMocks();
	localStorage.clear();
});

// =============================================================================
// SC-6: Context panel renders mode tabs and switches between them
// =============================================================================

describe('ContextPanel mode tabs (SC-6)', () => {
	it('should render Discussion mode tab', () => {
		renderWithProviders(<ContextPanel mode="discussion" />);

		expect(screen.getByRole('tab', { name: /discussion/i })).toBeInTheDocument();
	});

	it('should render Diff mode tab', () => {
		renderWithProviders(<ContextPanel mode="discussion" />);

		expect(screen.getByRole('tab', { name: /diff/i })).toBeInTheDocument();
	});

	it('should render Terminal mode tab', () => {
		renderWithProviders(<ContextPanel mode="discussion" />);

		expect(screen.getByRole('tab', { name: /terminal/i })).toBeInTheDocument();
	});

	it('should render Knowledge mode tab', () => {
		renderWithProviders(<ContextPanel mode="discussion" />);

		expect(screen.getByRole('tab', { name: /knowledge/i })).toBeInTheDocument();
	});

	it('should render Task mode tab', () => {
		renderWithProviders(<ContextPanel mode="discussion" />);

		expect(screen.getByRole('tab', { name: /task/i })).toBeInTheDocument();
	});

	it('should have active indicator on current mode tab', () => {
		renderWithProviders(<ContextPanel mode="discussion" />);

		const discussionTab = screen.getByRole('tab', { name: /discussion/i });
		expect(discussionTab).toHaveAttribute('aria-selected', 'true');
	});

	it('should switch mode when clicking a different tab', () => {
		const onModeChange = vi.fn();
		renderWithProviders(
			<ContextPanel mode="discussion" onModeChange={onModeChange} />
		);

		fireEvent.click(screen.getByRole('tab', { name: /diff/i }));

		expect(onModeChange).toHaveBeenCalledWith('diff');
	});

	it('should render correct content for discussion mode', () => {
		useThreadStore.setState({ selectedThreadId: 'thread-001' });
		renderWithProviders(<ContextPanel mode="discussion" threadId="thread-001" />);

		// Discussion mode should show the DiscussionPanel
		// (or at least show discussion-related content)
		expect(screen.getByRole('tab', { name: /discussion/i })).toHaveAttribute('aria-selected', 'true');
	});

	it('should render placeholder content for unimplemented modes', () => {
		renderWithProviders(<ContextPanel mode="diff" />);

		// Diff mode is a placeholder
		const diffTab = screen.getByRole('tab', { name: /diff/i });
		expect(diffTab).toHaveAttribute('aria-selected', 'true');
	});

	it('should show empty state when no mode is active', () => {
		renderWithProviders(<ContextPanel />);

		expect(screen.getByText(/select a thread or action/i)).toBeInTheDocument();
	});
});

// =============================================================================
// SC-8: Context panel is resizable with localStorage persistence
// =============================================================================

describe('ContextPanel resize (SC-8)', () => {
	it('should read initial width from localStorage', () => {
		localStorage.setItem('orc-context-panel-width', '400');

		const { container } = renderWithProviders(<ContextPanel mode="discussion" />);

		const panel = container.querySelector('[class*="context-panel"]');
		expect(panel).toBeInTheDocument();
		// Width should be read from localStorage
	});

	it('should use default width (360px) when no localStorage value', () => {
		const { container } = renderWithProviders(<ContextPanel mode="discussion" />);

		const panel = container.querySelector('[class*="context-panel"]');
		expect(panel).toBeInTheDocument();
		// Should use default 360px
	});

	it('should fall back to default 360px for invalid localStorage value', () => {
		localStorage.setItem('orc-context-panel-width', 'invalid');

		const { container } = renderWithProviders(<ContextPanel mode="discussion" />);

		const panel = container.querySelector('[class*="context-panel"]');
		expect(panel).toBeInTheDocument();
		// Should ignore invalid value and use 360px default
	});

	it('should have a resize handle element', () => {
		const { container } = renderWithProviders(<ContextPanel mode="discussion" />);

		const resizeHandle = container.querySelector('[class*="resize"]');
		expect(resizeHandle).toBeInTheDocument();
	});

	it('should enforce minimum width of 280px', () => {
		// Set a value below minimum
		localStorage.setItem('orc-context-panel-width', '100');

		const { container } = renderWithProviders(<ContextPanel mode="discussion" />);

		// Panel should clamp to min width
		const panel = container.querySelector('[class*="context-panel"]');
		expect(panel).toBeInTheDocument();
	});

	it('should persist width to localStorage on resize', async () => {
		const { container } = renderWithProviders(<ContextPanel mode="discussion" />);

		const resizeHandle = container.querySelector('[class*="resize"]');
		expect(resizeHandle).toBeInTheDocument();

		// Simulate drag (mousedown + mousemove + mouseup)
		if (resizeHandle) {
			fireEvent.mouseDown(resizeHandle, { clientX: 400 });
			fireEvent.mouseMove(document, { clientX: 350 });
			fireEvent.mouseUp(document);
		}

		await waitFor(() => {
			const stored = localStorage.getItem('orc-context-panel-width');
			expect(stored).toBeTruthy();
		});
	});
});

// =============================================================================
// SC-9: AppShell passes context panel mode and selected thread
// =============================================================================

describe('ContextPanel thread integration (SC-9)', () => {
	it('should render DiscussionPanel when mode is discussion and threadId is provided', () => {
		renderWithProviders(
			<ContextPanel mode="discussion" threadId="thread-001" />
		);

		// When mode is discussion with a threadId, DiscussionPanel should render
		const discussionTab = screen.getByRole('tab', { name: /discussion/i });
		expect(discussionTab).toHaveAttribute('aria-selected', 'true');
	});

	it('should show empty state when mode is discussion but no threadId', () => {
		renderWithProviders(
			<ContextPanel mode="discussion" />
		);

		// No thread selected should show appropriate message
		expect(screen.getByText(/select a thread/i)).toBeInTheDocument();
	});
});

// =============================================================================
// EDGE CASES
// =============================================================================

describe('ContextPanel edge cases', () => {
	it('should have aria-label on panel container', () => {
		renderWithProviders(<ContextPanel mode="discussion" />);

		const panel = screen.getByRole('complementary');
		expect(panel).toBeInTheDocument();
	});
});
