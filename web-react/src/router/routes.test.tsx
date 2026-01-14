import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { useRoutes } from 'react-router-dom';
import { routes } from './routes';
import { WebSocketProvider } from '@/hooks';
import { useProjectStore, useInitiativeStore, useUIStore } from '@/stores';

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
		it('renders Board page', async () => {
			renderWithRouter('/board');
			await waitFor(() => {
				// "Board" appears in sidebar and as h2 heading
				expect(screen.getByRole('heading', { level: 2, name: 'Board' })).toBeInTheDocument();
			});
		});
	});

	describe('/dashboard route', () => {
		it('renders Dashboard page', async () => {
			renderWithRouter('/dashboard');
			await waitFor(() => {
				// "Dashboard" appears in sidebar and as h2 heading
				expect(
					screen.getByRole('heading', { level: 2, name: 'Dashboard' })
				).toBeInTheDocument();
			});
		});
	});

	describe('/tasks/:id route', () => {
		it('renders TaskDetail page with task ID in heading', async () => {
			renderWithRouter('/tasks/TASK-001');
			await waitFor(() => {
				// The h2 contains "Task: " followed by the ID
				expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Task: TASK-001');
			});
		});

		it('displays correct task ID from route params', async () => {
			renderWithRouter('/tasks/TASK-123');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Task: TASK-123');
			});
		});
	});

	describe('/initiatives/:id route', () => {
		it('renders InitiativeDetail page with initiative ID', async () => {
			renderWithRouter('/initiatives/INIT-001');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(
					'Initiative: INIT-001'
				);
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
			await waitFor(() => {
				// The tab value should be rendered somewhere in the page
				expect(screen.getByText('transcript')).toBeInTheDocument();
			});
		});

		it('TaskDetail defaults to overview tab when no tab param', async () => {
			renderWithRouter('/tasks/TASK-001');
			await waitFor(() => {
				expect(screen.getByText('overview')).toBeInTheDocument();
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
