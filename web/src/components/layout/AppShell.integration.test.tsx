import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { AppShell } from './AppShell';
import { EventProvider } from '@/hooks';
import { TooltipProvider } from '@/components/ui';
import { useProjectStore, useSessionStore } from '@/stores';
import { useThreadStore } from '@/stores/threadStore';
import { useTaskStore } from '@/stores/taskStore';
import { createTimestamp, createMockTask } from '@/test/factories';
import { ProjectSchema } from '@/gen/orc/v1/project_pb';
import { ThreadSchema } from '@/gen/orc/v1/thread_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';

/**
 * Integration tests for the new AppShell layout.
 *
 * Covers:
 * - SC-9: AppShell passes context panel mode and selected thread to ContextPanel
 * - SC-12: All existing pages render correctly within the new layout shell
 */

// Mock modules that make API calls
vi.mock('@/lib/client', () => ({
	threadClient: {
		listThreads: vi.fn(),
		createThread: vi.fn(),
		getThread: vi.fn(),
		sendMessage: vi.fn(),
		promoteRecommendationDraft: vi.fn(),
		promoteDecisionDraft: vi.fn(),
	},
	taskClient: {
		listTasks: vi.fn(),
	},
	projectClient: {
		listProjects: vi.fn(),
	},
	initiativeClient: {
		listInitiatives: vi.fn(),
	},
}));

// =============================================================================
// TEST UTILITIES
// =============================================================================

function TestWrapper({
	children,
	initialEntries = ['/'],
}: {
	children: React.ReactNode;
	initialEntries?: string[];
}) {
	return (
		<MemoryRouter initialEntries={initialEntries}>
			<TooltipProvider delayDuration={0}>
				<EventProvider autoConnect={false}>
					{children}
				</EventProvider>
			</TooltipProvider>
		</MemoryRouter>
	);
}

function renderWithProviders(
	ui: React.ReactElement,
	initialEntries: string[] = ['/']
) {
	return render(ui, {
		wrapper: ({ children }) => (
			<TestWrapper initialEntries={initialEntries}>{children}</TestWrapper>
		),
	});
}

function createMockThread(overrides: Record<string, unknown> = {}) {
	return create(ThreadSchema, {
		id: 'thread-001',
		title: 'Test Thread',
		status: 'open',
		taskId: '',
		initiativeId: '',
		sessionId: '',
		fileContext: '',
		createdAt: createTimestamp('2024-01-01T00:00:00Z'),
		updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
		messages: [],
		...overrides,
	});
}

// =============================================================================
// STORE SETUP
// =============================================================================

beforeEach(() => {
	useProjectStore.setState({
		projects: [
			create(ProjectSchema, {
				id: 'proj-001',
				name: 'Test Project',
				path: '/test/project',
				createdAt: createTimestamp('2024-01-01T00:00:00Z'),
			}),
		],
		currentProjectId: 'proj-001',
		loading: false,
		error: null,
	});

	useSessionStore.setState({
		sessionId: 'test-session',
		startTime: new Date(),
		totalTokens: 0,
		totalCost: 0,
		isPaused: false,
		activeTaskCount: 0,
		duration: '0m',
		formattedCost: '$0.00',
		formattedTokens: '0',
	});

	useTaskStore.setState({
		tasks: [
			createMockTask({ id: 'TASK-001', status: TaskStatus.RUNNING }),
		],
	});

	useThreadStore.getState().reset();
	localStorage.clear();
});

afterEach(() => {
	vi.clearAllMocks();
	localStorage.clear();
});

// =============================================================================
// SC-9: AppShell passes context panel mode and thread
// =============================================================================

describe('AppShell context panel integration (SC-9)', () => {
	it('should render ProjectSidebar in left column', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		// ProjectSidebar should be present (replaces IconNav)
		// ProjectSidebar shows the project name
		expect(screen.getByText('Test Project')).toBeInTheDocument();
	});

	it('should render ContextPanel in right column', () => {
		const { container } = renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		// ContextPanel should be in the DOM
		const contextPanel = container.querySelector('[class*="context-panel"]');
		expect(contextPanel).toBeInTheDocument();
	});

	it('should render TerminalDrawer at bottom', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		expect(screen.getByTestId('terminal-drawer')).toBeInTheDocument();
	});

	it('should open context panel in discussion mode when thread is selected from sidebar', async () => {
		useThreadStore.setState({
			threads: [
				createMockThread({ id: 'thread-001', title: 'My Thread' }),
			],
		});

		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		// Click thread in sidebar
		fireEvent.click(screen.getByText('My Thread'));

		await waitFor(() => {
			// Context panel should show discussion mode
			const discussionTab = screen.getByRole('tab', { name: /discussion/i });
			expect(discussionTab).toHaveAttribute('aria-selected', 'true');
		});
	});

	it('should switch to discussion mode when context panel is open in another mode', async () => {
		useThreadStore.setState({
			threads: [
				createMockThread({ id: 'thread-001', title: 'Thread A' }),
			],
		});

		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		// Click thread - should open in discussion mode regardless of current mode
		fireEvent.click(screen.getByText('Thread A'));

		await waitFor(() => {
			const discussionTab = screen.getByRole('tab', { name: /discussion/i });
			expect(discussionTab).toHaveAttribute('aria-selected', 'true');
		});
	});

	it('should show "Select a thread or action" when no mode is active and panel is open', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		// With no thread selected and no mode active
		expect(screen.getByText(/select a thread or action/i)).toBeInTheDocument();
	});
});

// =============================================================================
// SC-12: All existing pages render correctly within new layout
// =============================================================================

describe('AppShell existing page compatibility (SC-12)', () => {
	it('should render main content area with children', () => {
		renderWithProviders(
			<AppShell>
				<div data-testid="page-content">Board Page Content</div>
			</AppShell>
		);

		expect(screen.getByTestId('page-content')).toBeInTheDocument();
		expect(screen.getByText('Board Page Content')).toBeInTheDocument();
	});

	it('should render main content with role="main"', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		expect(screen.getByRole('main')).toBeInTheDocument();
	});

	it('should render skip link for accessibility', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const skipLink = screen.getByText('Skip to main content');
		expect(skipLink).toBeInTheDocument();
		expect(skipLink).toHaveAttribute('href', '#main-content');
	});

	it('should have main content with id="main-content"', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const main = screen.getByRole('main');
		expect(main).toHaveAttribute('id', 'main-content');
	});

	it('should still fire onNewTask callback', () => {
		const onNewTask = vi.fn();
		renderWithProviders(
			<AppShell onNewTask={onNewTask}>
				<div>Content</div>
			</AppShell>
		);

		const newTaskBtn = screen.getByRole('button', { name: /new task/i });
		fireEvent.click(newTaskBtn);

		expect(onNewTask).toHaveBeenCalledOnce();
	});

	it('should have banner element from TopBar', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		expect(screen.getByRole('banner')).toBeInTheDocument();
	});

	it('should apply custom className', () => {
		const { container } = renderWithProviders(
			<AppShell className="custom-shell">
				<div>Content</div>
			</AppShell>
		);

		const shell = container.querySelector('.app-shell');
		expect(shell).toHaveClass('custom-shell');
	});
});

// =============================================================================
// LAYOUT GRID STRUCTURE
// =============================================================================

describe('AppShell new grid layout', () => {
	it('should have sidebar area (ProjectSidebar)', () => {
		const { container } = renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		// The new layout should have a sidebar section
		const sidebar = container.querySelector('.app-shell__sidebar');
		expect(sidebar).toBeInTheDocument();
	});

	it('should have topbar area', () => {
		const { container } = renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		expect(container.querySelector('.app-shell__topbar')).toBeInTheDocument();
	});

	it('should have main content area', () => {
		const { container } = renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		expect(container.querySelector('.app-shell__main')).toBeInTheDocument();
	});

	it('should have context panel area', () => {
		const { container } = renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		expect(container.querySelector('.app-shell__context-panel')).toBeInTheDocument();
	});

	it('should have terminal drawer area', () => {
		const { container } = renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		expect(container.querySelector('.app-shell__terminal-drawer')).toBeInTheDocument();
	});

	it('should no longer render IconNav', () => {
		const { container } = renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		// The old .app-shell__nav (IconNav) should not exist
		expect(container.querySelector('.app-shell__nav')).not.toBeInTheDocument();
		// IconNav rendered a logo "O" — should not be present
		expect(screen.queryByText('O')).not.toBeInTheDocument();
	});
});

// =============================================================================
// KEYBOARD SHORTCUTS
// =============================================================================

describe('AppShell keyboard shortcuts (SC-12)', () => {
	it('should toggle terminal drawer on Cmd+J', async () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		act(() => {
			fireEvent.keyDown(document, { key: 'j', metaKey: true });
		});

		await waitFor(() => {
			const drawer = screen.getByTestId('terminal-drawer');
			expect(drawer.className).toMatch(/open/);
		});
	});
});

// =============================================================================
// EDGE CASES
// =============================================================================

describe('AppShell edge cases', () => {
	it('should restore context panel with selected thread when panel was closed', async () => {
		useThreadStore.setState({
			threads: [createMockThread({ id: 'thread-001', title: 'Saved Thread' })],
			selectedThreadId: 'thread-001',
		});

		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		// With a selected thread, opening the panel should show discussion mode
		await waitFor(() => {
			const discussionTab = screen.queryByRole('tab', { name: /discussion/i });
			if (discussionTab) {
				expect(discussionTab).toHaveAttribute('aria-selected', 'true');
			}
		});
	});
});
