/**
 * Integration Tests for Routes - My Work page as default landing
 *
 * Success Criteria Coverage:
 * - SC-5: Index route (/) renders MyWorkPage instead of redirecting to /board
 * - SC-6: RootLayout renders AppShellLayout regardless of project selection state
 *
 * INTEGRATION TEST:
 * These tests verify that routes.tsx has been modified to:
 * 1. Render MyWorkPage at / instead of redirecting to /board
 * 2. Render AppShellLayout even when no project is selected
 *
 * If the implementation doesn't modify routes.tsx, these tests fail because:
 * - / will redirect to /board (SC-5 fails)
 * - No project selected will show ProjectPickerPage (SC-6 fails)
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { routes } from './routes';
import { TooltipProvider } from '@/components/ui';

// Mock all lazy-loaded page components to avoid loading real implementations
vi.mock('@/pages/MyWorkPage', () => ({
	MyWorkPage: () => <div data-testid="my-work-page">My Work Page</div>,
}));

vi.mock('@/pages/Board', () => ({
	Board: () => <div data-testid="board-page">Board Page</div>,
}));

vi.mock('@/pages/ProjectPickerPage', () => ({
	ProjectPickerPage: () => <div data-testid="project-picker-page">Project Picker</div>,
}));

// Mock the API client
vi.mock('@/lib/client', () => ({
	projectClient: {
		getAllProjectsStatus: vi.fn().mockResolvedValue({ projects: [] }),
		listProjects: vi.fn().mockResolvedValue({ projects: [] }),
		getDefaultProject: vi.fn().mockResolvedValue({}),
	},
	taskClient: {
		listTasks: vi.fn().mockResolvedValue({ tasks: [] }),
	},
	initiativeClient: {
		listInitiatives: vi.fn().mockResolvedValue({ initiatives: [] }),
	},
	eventClient: {
		subscribe: vi.fn(),
	},
	dashboardClient: {
		getDashboard: vi.fn().mockResolvedValue({}),
	},
	workflowClient: {
		listWorkflows: vi.fn().mockResolvedValue({ workflows: [] }),
	},
}));

// Mock task store
vi.mock('@/stores/taskStore', () => ({
	useTaskStore: Object.assign(
		vi.fn((selector?: (state: Record<string, unknown>) => unknown) => {
			const state = {
				tasks: [],
				taskStates: new Map(),
				addTask: vi.fn(),
			};
			return selector ? selector(state) : state;
		}),
		{
			getState: vi.fn(() => ({
				addTask: vi.fn(),
				tasks: [],
				taskStates: new Map(),
			})),
			subscribe: vi.fn(() => vi.fn()),
			setState: vi.fn(),
		}
	),
	useActiveTasks: vi.fn(() => []),
	useRecentTasks: vi.fn(() => []),
	useRunningTasks: vi.fn(() => []),
	useStatusCounts: vi.fn(() => ({})),
	useTask: vi.fn(() => undefined),
	useTaskState: vi.fn(() => undefined),
	useTaskActivity: vi.fn(() => []),
}));

// Mock project store - test with NO project selected
const mockSelectProject = vi.fn();
vi.mock('@/stores/projectStore', () => ({
	useProjectStore: Object.assign(
		vi.fn((selector?: (state: Record<string, unknown>) => unknown) => {
			const state = {
				projects: [],
				currentProjectId: null,
				selectProject: mockSelectProject,
				loading: false,
				error: null,
			};
			return selector ? selector(state) : state;
		}),
		{
			getState: vi.fn(() => ({
				selectProject: mockSelectProject,
				currentProjectId: null,
			})),
			subscribe: vi.fn(() => vi.fn()),
		}
	),
	useCurrentProjectId: vi.fn(() => null),
	useProjectLoading: vi.fn(() => false),
	useCurrentProject: vi.fn(() => undefined),
	useProjects: vi.fn(() => []),
}));

// Mock other stores that may be used by layout components
vi.mock('@/stores/uiStore', () => ({
	useUIStore: vi.fn((selector?: (state: Record<string, unknown>) => unknown) => {
		const state = { sidebarExpanded: true, wsStatus: 'connected' };
		return selector ? selector(state) : state;
	}),
	useSidebarExpanded: vi.fn(() => true),
	useMobileMenuOpen: vi.fn(() => false),
	useWsStatus: vi.fn(() => 'connected'),
	useToasts: vi.fn(() => []),
	toast: vi.fn(),
}));

vi.mock('@/stores/initiativeStore', () => ({
	useInitiativeStore: vi.fn(() => ({
		initiatives: [],
		currentInitiativeId: null,
	})),
	useInitiatives: vi.fn(() => []),
	useCurrentInitiative: vi.fn(() => undefined),
	useCurrentInitiativeId: vi.fn(() => null),
	UNASSIGNED_INITIATIVE: 'unassigned',
	truncateInitiativeTitle: vi.fn((s: string) => s),
	getInitiativeBadgeTitle: vi.fn((s: string) => s),
}));

vi.mock('@/stores/sessionStore', () => ({
	useSessionStore: vi.fn(() => ({})),
	useSessionId: vi.fn(() => null),
	useStartTime: vi.fn(() => null),
	useTotalTokens: vi.fn(() => 0),
	useTotalCost: vi.fn(() => 0),
	useIsPaused: vi.fn(() => false),
	useActiveTaskCount: vi.fn(() => 0),
	useFormattedDuration: vi.fn(() => '0:00'),
	useFormattedCost: vi.fn(() => '$0.00'),
	useFormattedTokens: vi.fn(() => '0'),
	useSessionMetrics: vi.fn(() => ({})),
	formatDuration: vi.fn(() => '0:00'),
}));

function renderRoute(initialPath: string) {
	const router = createMemoryRouter(routes, {
		initialEntries: [initialPath],
	});

	return render(
		<TooltipProvider delayDuration={0}>
			<RouterProvider router={router} />
		</TooltipProvider>
	);
}

describe('Routes - My Work integration', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('SC-5: index route renders MyWorkPage', () => {
		it('should render MyWorkPage when navigating to /', async () => {
			renderRoute('/');

			await waitFor(() => {
				expect(screen.getByTestId('my-work-page')).toBeInTheDocument();
			});
		});

		it('should NOT redirect to /board from /', async () => {
			renderRoute('/');

			await waitFor(() => {
				// MyWorkPage should be visible, NOT board
				expect(screen.getByTestId('my-work-page')).toBeInTheDocument();
				expect(screen.queryByTestId('board-page')).not.toBeInTheDocument();
			});
		});
	});

	describe('SC-6: RootLayout renders AppShellLayout without project selection', () => {
		it('should NOT show ProjectPickerPage when no project is selected', async () => {
			renderRoute('/');

			await waitFor(() => {
				// AppShellLayout content should be visible (MyWorkPage)
				expect(screen.getByTestId('my-work-page')).toBeInTheDocument();
				// ProjectPickerPage should NOT be shown
				expect(screen.queryByTestId('project-picker-page')).not.toBeInTheDocument();
			});
		});

		it('should render navigation (AppShellLayout) when no project selected', async () => {
			renderRoute('/');

			await waitFor(() => {
				// AppShellLayout includes IconNav which has navigation role
				const nav = screen.queryByRole('navigation', { name: 'Main navigation' });
				expect(nav).toBeInTheDocument();
			});
		});
	});
});
