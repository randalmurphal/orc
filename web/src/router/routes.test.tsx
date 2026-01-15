import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { useRoutes } from 'react-router-dom';
import { routes } from './routes';
import { WebSocketProvider } from '@/hooks';
import { useProjectStore, useInitiativeStore, useUIStore, useTaskStore } from '@/stores';

// Mock WebSocket to prevent actual connections
vi.mock('@/lib/websocket', () => ({
	OrcWebSocket: vi.fn().mockImplementation(() => ({
		connect: vi.fn(),
		disconnect: vi.fn(),
		subscribe: vi.fn(),
		unsubscribe: vi.fn(),
		subscribeGlobal: vi.fn(),
		setPrimarySubscription: vi.fn(),
		on: vi.fn().mockReturnValue(() => {}),
		onStatusChange: vi.fn().mockReturnValue(() => {}),
		isConnected: vi.fn().mockReturnValue(false),
		getTaskId: vi.fn().mockReturnValue(null),
		command: vi.fn(),
	})),
	GLOBAL_TASK_ID: '*',
}));

// Mock API to prevent actual fetch calls
vi.mock('@/lib/api', () => ({
	listProjectTasks: vi.fn().mockResolvedValue([]),
	listInitiatives: vi.fn().mockResolvedValue([]),
	getDashboardStats: vi.fn().mockResolvedValue({
		running: 1,
		paused: 0,
		blocked: 2,
		completed: 5,
		failed: 0,
		today: 3,
		total: 10,
		tokens: 50000,
		cost: 0.5,
	}),
	runProjectTask: vi.fn(),
	pauseProjectTask: vi.fn(),
	resumeProjectTask: vi.fn(),
	escalateProjectTask: vi.fn(),
	updateTask: vi.fn(),
	triggerFinalize: vi.fn(),
	// Initiative detail page APIs
	getInitiative: vi.fn().mockResolvedValue({
		id: 'INIT-001',
		title: 'Test Initiative',
		status: 'active',
		tasks: [],
		decisions: [],
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		version: 1,
	}),
	updateInitiative: vi.fn(),
	addInitiativeTask: vi.fn(),
	removeInitiativeTask: vi.fn(),
	addInitiativeDecision: vi.fn(),
	listTasks: vi.fn().mockResolvedValue([]),
	getInitiativeDependencyGraph: vi.fn().mockResolvedValue({ nodes: [], edges: [] }),
	// TaskDetail page API
	getTask: vi.fn().mockResolvedValue({
		id: 'TASK-001',
		title: 'Test Task',
		status: 'created',
		weight: 'medium',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString(),
	}),
	getTaskPlan: vi.fn().mockResolvedValue({
		phases: [{ id: 'implement-1', name: 'implement', status: 'pending', iterations: 0 }],
	}),
	getTaskDependencies: vi.fn().mockResolvedValue({
		blocked_by: [],
		blocks: [],
		related_to: [],
		referenced_by: [],
	}),
	getTaskTimeline: vi.fn().mockResolvedValue([]),
}));

// Test wrapper component
function TestApp() {
	const routeElements = useRoutes(routes);
	return <WebSocketProvider autoConnect={false}>{routeElements}</WebSocketProvider>;
}

function renderWithRouter(initialPath: string = '/') {
	return render(
		<MemoryRouter initialEntries={[initialPath]}>
			<TestApp />
		</MemoryRouter>
	);
}

describe('Routes', () => {
	beforeEach(() => {
		// Reset stores between tests
		useProjectStore.setState({
			projects: [],
			currentProjectId: null,
			loading: false,
			error: null,
			_isHandlingPopState: false,
		});
		useInitiativeStore.setState({
			initiatives: new Map(),
			currentInitiativeId: null,
			loading: false,
			error: null,
			hasLoaded: false,
			_isHandlingPopState: false,
		});
		useUIStore.setState({
			sidebarExpanded: true,
			wsStatus: 'disconnected',
			toasts: [],
		});
		useTaskStore.setState({
			tasks: [],
			taskStates: new Map(),
			loading: false,
			error: null,
		});
	});

	describe('Root route (/)', () => {
		it('renders TaskList page', async () => {
			renderWithRouter('/');
			await waitFor(() => {
				expect(screen.getByText('Task List')).toBeInTheDocument();
			});
		});

		it('renders sidebar navigation', async () => {
			renderWithRouter('/');
			await waitFor(() => {
				// Check sidebar nav links exist by href attribute
				const nav = screen.getByRole('navigation', { name: 'Main navigation' });
				expect(nav.querySelector('a[href="/"]')).toBeInTheDocument();
				expect(nav.querySelector('a[href="/board"]')).toBeInTheDocument();
				expect(nav.querySelector('a[href="/dashboard"]')).toBeInTheDocument();
			});
		});

		it('renders header', async () => {
			renderWithRouter('/');
			await waitFor(() => {
				// Header title should be "Tasks" for root route
				expect(screen.getAllByText('Tasks').length).toBeGreaterThan(0);
			});
		});
	});

	describe('/board route', () => {
		it('renders Board page with project selected', async () => {
			// Board page requires a project to be selected
			// Set state before render - Zustand stores are synchronous
			useProjectStore.setState({
				projects: [
					{
						id: 'test-project',
						path: '/test/project',
						name: 'Test Project',
						created_at: '2024-01-01T00:00:00Z',
					},
				],
				currentProjectId: 'test-project',
				loading: false,
				error: null,
				_isHandlingPopState: false,
			});

			// Render with project param in URL (UrlParamSync syncs URL -> store)
			renderWithRouter('/board?project=test-project');

			// Wait for the component to render and show the board
			await waitFor(() => {
				// "Board" appears as h2 heading
				expect(screen.getByRole('heading', { level: 2, name: 'Board' })).toBeInTheDocument();
			});
		});

		it('renders empty state when no project selected', async () => {
			renderWithRouter('/board');
			await waitFor(() => {
				expect(screen.getByText('No Project Selected')).toBeInTheDocument();
			});
		});
	});

	describe('/dashboard route', () => {
		beforeEach(() => {
			// Mock API responses for dashboard
			vi.mocked(fetch).mockImplementation((url) => {
				const urlStr = typeof url === 'string' ? url : url.toString();
				if (urlStr.includes('/api/dashboard/stats')) {
					return Promise.resolve({
						ok: true,
						json: () =>
							Promise.resolve({
								running: 1,
								paused: 0,
								blocked: 2,
								completed: 5,
								failed: 0,
								today: 3,
								total: 10,
								tokens: 50000,
								cost: 0.5,
							}),
					} as Response);
				}
				if (urlStr.includes('/api/initiatives')) {
					return Promise.resolve({
						ok: true,
						json: () => Promise.resolve([]),
					} as Response);
				}
				return Promise.resolve({
					ok: true,
					json: () => Promise.resolve({}),
				} as Response);
			});
		});

		it('renders Dashboard page with Quick Stats', async () => {
			renderWithRouter('/dashboard');
			await waitFor(() => {
				// "Dashboard" appears in header h1
				expect(screen.getByRole('heading', { level: 1, name: 'Dashboard' })).toBeInTheDocument();
				// Dashboard shows "Quick Stats" section
				expect(screen.getByText('Quick Stats')).toBeInTheDocument();
			});
		});
	});

	describe('/tasks/:id route', () => {
		it('renders TaskDetail page with task title in heading', async () => {
			renderWithRouter('/tasks/TASK-001');
			await waitFor(() => {
				// The task title appears in an h1 with class "task-title"
				const taskTitle = document.querySelector('h1.task-title');
				expect(taskTitle).toBeInTheDocument();
				expect(taskTitle).toHaveTextContent('Test Task');
			});
		});

		it('displays task ID somewhere on the page', async () => {
			renderWithRouter('/tasks/TASK-001');
			await waitFor(() => {
				// The task ID should appear somewhere in the header
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});
		});
	});

	describe('/initiatives/:id route', () => {
		it('renders InitiativeDetail page with initiative title', async () => {
			renderWithRouter('/initiatives/INIT-001');
			await waitFor(() => {
				// The page now loads initiative data and shows the title in h1
				// Use getByText since there are multiple headings in the layout
				expect(screen.getByText('Test Initiative')).toBeInTheDocument();
			});
		});
	});

	describe('/preferences route', () => {
		it('renders Preferences page', async () => {
			renderWithRouter('/preferences');
			await waitFor(() => {
				// There's "Preferences" in sidebar and in page heading
				// Check the h2 specifically
				expect(
					screen.getByRole('heading', { level: 2, name: 'Preferences' })
				).toBeInTheDocument();
			});
		});
	});

	describe('/environment routes', () => {
		it('redirects /environment to /environment/settings', async () => {
			renderWithRouter('/environment');
			await waitFor(() => {
				// Environment pages use h3 headings
				expect(screen.getByRole('heading', { level: 3, name: 'Settings' })).toBeInTheDocument();
			});
		});

		it('renders Settings page at /environment/settings', async () => {
			renderWithRouter('/environment/settings');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 3, name: 'Settings' })).toBeInTheDocument();
			});
		});

		it('renders Prompts page at /environment/prompts', async () => {
			renderWithRouter('/environment/prompts');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 3, name: 'Prompts' })).toBeInTheDocument();
			});
		});

		it('renders Scripts page at /environment/scripts', async () => {
			renderWithRouter('/environment/scripts');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 3, name: 'Scripts' })).toBeInTheDocument();
			});
		});

		it('renders Hooks page at /environment/hooks', async () => {
			renderWithRouter('/environment/hooks');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 3, name: 'Hooks' })).toBeInTheDocument();
			});
		});

		it('renders Skills page at /environment/skills', async () => {
			renderWithRouter('/environment/skills');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 3, name: 'Skills' })).toBeInTheDocument();
			});
		});

		it('renders MCP page at /environment/mcp', async () => {
			renderWithRouter('/environment/mcp');
			await waitFor(() => {
				expect(
					screen.getByRole('heading', { level: 3, name: 'MCP Servers' })
				).toBeInTheDocument();
			});
		});

		it('renders Config page at /environment/config', async () => {
			renderWithRouter('/environment/config');
			await waitFor(() => {
				expect(
					screen.getByRole('heading', { level: 3, name: 'Configuration' })
				).toBeInTheDocument();
			});
		});

		it('renders ClaudeMd page at /environment/claudemd', async () => {
			renderWithRouter('/environment/claudemd');
			await waitFor(() => {
				expect(
					screen.getByRole('heading', { level: 3, name: 'CLAUDE.md' })
				).toBeInTheDocument();
			});
		});

		it('renders Tools page at /environment/tools', async () => {
			renderWithRouter('/environment/tools');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 3, name: 'Tools' })).toBeInTheDocument();
			});
		});

		it('renders Agents page at /environment/agents', async () => {
			renderWithRouter('/environment/agents');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 3, name: 'Agents' })).toBeInTheDocument();
			});
		});
	});

	describe('URL parameters', () => {
		it('syncs project param from URL to store', async () => {
			renderWithRouter('/?project=test-project');
			await waitFor(() => {
				expect(useProjectStore.getState().currentProjectId).toBe('test-project');
			});
		});

		it('syncs initiative param from URL to store on root route', async () => {
			renderWithRouter('/?initiative=INIT-001');
			await waitFor(() => {
				expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-001');
			});
		});

		it('syncs initiative param from URL to store on board route', async () => {
			renderWithRouter('/board?initiative=INIT-002');
			await waitFor(() => {
				expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-002');
			});
		});

		it('TaskDetail reads tab param from URL', async () => {
			renderWithRouter('/tasks/TASK-001?tab=transcript');
			// Wait for task to load and tabs to render
			await waitFor(() => {
				// The tab button should be rendered
				expect(screen.getByRole('tab', { name: /transcript/i })).toBeInTheDocument();
			});
		});

		it('TaskDetail defaults to timeline tab when no tab param', async () => {
			renderWithRouter('/tasks/TASK-001');
			// Wait for task to load and tabs to render
			await waitFor(() => {
				// The timeline tab button should be rendered
				expect(screen.getByRole('tab', { name: /timeline/i })).toBeInTheDocument();
			});
		});
	});

	describe('Layout structure', () => {
		it('renders sidebar on all routes', async () => {
			renderWithRouter('/');
			expect(screen.getByRole('navigation', { name: 'Main navigation' })).toBeInTheDocument();
		});

		it('renders header on all routes', async () => {
			renderWithRouter('/');
			expect(screen.getByRole('banner')).toBeInTheDocument();
		});

		it('shows environment sub-navigation on environment routes', async () => {
			renderWithRouter('/environment/settings');
			await waitFor(() => {
				// Should see sub-navigation links
				expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument();
				expect(screen.getByRole('link', { name: 'Prompts' })).toBeInTheDocument();
			});
		});
	});
});
