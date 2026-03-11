import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { ProjectSidebar } from './ProjectSidebar';
import { AppShellProvider } from './AppShellContext';
import { TooltipProvider } from '@/components/ui';
import { useProjectStore, useSessionStore } from '@/stores';
import { useThreadStore } from '@/stores/threadStore';
import { useTaskStore } from '@/stores/taskStore';
import { createTimestamp, createMockTask } from '@/test/factories';
import { ProjectSchema } from '@/gen/orc/v1/project_pb';
import { ThreadSchema } from '@/gen/orc/v1/thread_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';

// Mock threadClient for createThread tests
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

function TestWrapper({ children }: { children: React.ReactNode }) {
	return (
		<MemoryRouter>
			<TooltipProvider delayDuration={0}>
				<AppShellProvider>
					{children}
				</AppShellProvider>
			</TooltipProvider>
		</MemoryRouter>
	);
}

function renderWithProviders(ui: React.ReactElement) {
	return render(ui, { wrapper: TestWrapper });
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
			createMockTask({ id: 'TASK-002', status: TaskStatus.RUNNING }),
			createMockTask({ id: 'TASK-003', status: TaskStatus.COMPLETED }),
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
// SC-1: ProjectSidebar renders in AppShell grid replacing IconNav
// =============================================================================

describe('ProjectSidebar rendering (SC-1)', () => {
	it('should render project name from current project', () => {
		renderWithProviders(<ProjectSidebar />);

		expect(screen.getByText('Test Project')).toBeInTheDocument();
	});

	it('should render thread section heading', () => {
		renderWithProviders(<ProjectSidebar />);

		expect(screen.getByText('Threads')).toBeInTheDocument();
	});

	it('should render running task count from taskStore', () => {
		renderWithProviders(<ProjectSidebar />);

		// 2 tasks are RUNNING in our setup
		expect(screen.getByText('2')).toBeInTheDocument();
	});

	it('should render project switcher trigger button', () => {
		renderWithProviders(<ProjectSidebar />);

		const projectButton = screen.getByText('Test Project').closest('button');
		expect(projectButton).toBeInTheDocument();
	});

	it('should call onProjectChange when project switcher is clicked', () => {
		const onProjectChange = vi.fn();
		renderWithProviders(<ProjectSidebar onProjectChange={onProjectChange} />);

		const projectButton = screen.getByText('Test Project').closest('button');
		fireEvent.click(projectButton!);

		expect(onProjectChange).toHaveBeenCalledOnce();
	});

	it('should have navigation role', () => {
		renderWithProviders(<ProjectSidebar />);

		expect(screen.getByRole('navigation')).toBeInTheDocument();
	});
});

// =============================================================================
// SC-2: Thread list loads from ThreadService API with status indicators
// =============================================================================

describe('ProjectSidebar thread list (SC-2)', () => {
	it('should display threads from threadStore', () => {
		useThreadStore.setState({
			threads: [
				createMockThread({ id: 'thread-001', title: 'Design Discussion' }),
				createMockThread({ id: 'thread-002', title: 'Bug Triage', status: 'archived' }),
			],
		});

		renderWithProviders(<ProjectSidebar />);

		expect(screen.getByText('Design Discussion')).toBeInTheDocument();
		expect(screen.getByText('Bug Triage')).toBeInTheDocument();
	});

	it('should show status indicators for open threads', () => {
		useThreadStore.setState({
			threads: [
				createMockThread({ id: 'thread-001', title: 'Open Thread', status: 'open' }),
			],
		});

		renderWithProviders(<ProjectSidebar />);

		// Status indicator should be present (a colored dot)
		const openItem = screen.getByText('Open Thread').closest('[class*="thread"]');
		expect(openItem).toBeInTheDocument();
	});

	it('should show status indicators for archived threads', () => {
		useThreadStore.setState({
			threads: [
				createMockThread({ id: 'thread-001', title: 'Archived Thread', status: 'archived' }),
			],
		});

		renderWithProviders(<ProjectSidebar />);

		const archivedItem = screen.getByText('Archived Thread').closest('[class*="thread"]');
		expect(archivedItem).toBeInTheDocument();
	});

	it('should show empty state when no threads exist', () => {
		useThreadStore.setState({ threads: [] });

		renderWithProviders(<ProjectSidebar />);

		expect(screen.getByText(/no discussion threads/i)).toBeInTheDocument();
		expect(screen.getByText(/separate from running tasks/i)).toBeInTheDocument();
	});

	it('should show error state when thread loading fails', () => {
		useThreadStore.setState({
			threads: [],
			error: 'Failed to load threads',
		});

		renderWithProviders(<ProjectSidebar />);

		expect(screen.getByText(/failed to load threads/i)).toBeInTheDocument();
	});

	it('should show retry button on error', () => {
		useThreadStore.setState({
			threads: [],
			error: 'Failed to load threads',
		});

		renderWithProviders(<ProjectSidebar />);

		expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
	});
});

// =============================================================================
// SC-3: Clicking thread opens context panel in discussion mode
// =============================================================================

describe('ProjectSidebar thread click (SC-3)', () => {
	it('should select thread in store when clicked', () => {
		useThreadStore.setState({
			threads: [
				createMockThread({ id: 'thread-001', title: 'Click Me' }),
			],
		});

		renderWithProviders(<ProjectSidebar />);

		fireEvent.click(screen.getByText('Click Me'));

		expect(useThreadStore.getState().selectedThreadId).toBe('thread-001');
	});

	it('should highlight the selected thread', () => {
		useThreadStore.setState({
			threads: [
				createMockThread({ id: 'thread-001', title: 'Selected Thread' }),
				createMockThread({ id: 'thread-002', title: 'Other Thread' }),
			],
			selectedThreadId: 'thread-001',
		});

		renderWithProviders(<ProjectSidebar />);

		const selectedItem = screen.getByText('Selected Thread').closest('[class*="thread"]');
		expect(selectedItem?.className).toMatch(/active|selected/);
	});
});

// =============================================================================
// SC-4: New Thread button creates thread and opens in discussion mode
// =============================================================================

describe('ProjectSidebar new thread (SC-4)', () => {
	it('should render New Thread button', () => {
		renderWithProviders(<ProjectSidebar />);

		expect(screen.getByRole('button', { name: /new thread/i })).toBeInTheDocument();
	});

	it('should call createThread when New Thread is clicked', async () => {
		const createThread = vi.fn().mockResolvedValue(
			createMockThread({ id: 'thread-new', title: 'New Thread' })
		);
		useThreadStore.setState({ createThread } as never);

		renderWithProviders(<ProjectSidebar />);

		fireEvent.click(screen.getByRole('button', { name: /new thread/i }));

		await waitFor(() => {
			expect(createThread).toHaveBeenCalledWith('proj-001', expect.any(String));
		});
	});
});

// =============================================================================
// EDGE CASES
// =============================================================================

describe('ProjectSidebar edge cases', () => {
	it('should show prompt when no project is selected', () => {
		useProjectStore.setState({
			projects: [],
			currentProjectId: null,
		});

		renderWithProviders(<ProjectSidebar />);

		expect(screen.getByText(/select a project/i)).toBeInTheDocument();
	});

	it('should handle 50+ threads with scrollable list', () => {
		const manyThreads = Array.from({ length: 55 }, (_, i) =>
			createMockThread({ id: `thread-${i}`, title: `Thread ${i}` })
		);
		useThreadStore.setState({ threads: manyThreads });

		renderWithProviders(<ProjectSidebar />);

		// Thread list section should have overflow-y: auto or similar
		// Verify threads are rendered
		expect(screen.getByText('Thread 0')).toBeInTheDocument();
		expect(screen.getByText('Thread 54')).toBeInTheDocument();
	});
});
