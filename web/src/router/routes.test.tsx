import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { useRoutes } from 'react-router-dom';
import { routes } from './routes';
import { EventProvider } from '@/hooks';
import { TooltipProvider } from '@/components/ui/Tooltip';
import { useProjectStore, useInitiativeStore, useUIStore, useTaskStore } from '@/stores';
import { createTimestamp } from '@/test/factories';
import type { Project } from '@/gen/orc/v1/project_pb';

// Mock events module to prevent actual connections
vi.mock('@/lib/events', () => ({
	EventSubscription: vi.fn().mockImplementation(() => ({
		connect: vi.fn(),
		disconnect: vi.fn(),
		on: vi.fn().mockReturnValue(() => {}),
		onStatusChange: vi.fn().mockReturnValue(() => {}),
		getStatus: vi.fn().mockReturnValue('disconnected'),
	})),
	handleEvent: vi.fn(),
}));

// Mock global fetch for DependencySidebar raw API calls
vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
	if (typeof url === 'string' && url.includes('/dependencies')) {
		return Promise.resolve({
			ok: true,
			json: () => Promise.resolve({ blocked_by: [], blocks: [], related_to: [], referenced_by: [] }),
		});
	}
	// Return empty response for other endpoints
	return Promise.resolve({
		ok: true,
		json: () => Promise.resolve({}),
	});
}));

// Mock the Connect RPC client for TaskDetail component
vi.mock('@/lib/client', () => ({
	taskClient: {
		getTask: vi.fn().mockResolvedValue({
			task: {
				id: 'TASK-001',
				title: 'Test Task',
				status: 1, // TaskStatus.CREATED
				weight: 2, // TaskWeight.MEDIUM
			},
		}),
		getTaskPlan: vi.fn().mockResolvedValue({
			plan: {
				phases: [{ id: 'implement-1', name: 'implement', status: 0, iterations: 0 }],
			},
		}),
		listTasks: vi.fn().mockResolvedValue({ tasks: [] }),
	},
	initiativeClient: {
		getInitiative: vi.fn().mockResolvedValue({
			initiative: {
				id: 'INIT-001',
				title: 'Test Initiative',
				status: 1, // InitiativeStatus.ACTIVE
				tasks: [],
				decisions: [],
			},
		}),
		listInitiatives: vi.fn().mockResolvedValue({ initiatives: [] }),
	},
	configClient: {
		getConfigStats: vi.fn().mockResolvedValue({
			stats: {
				slashCommandsCount: 0,
				claudeMdSize: BigInt(0),
				mcpServersCount: 0,
				permissionsProfile: 'default',
			},
		}),
		listAgents: vi.fn().mockResolvedValue({
			agents: [],
		}),
		getConfig: vi.fn().mockResolvedValue({
			config: {
				executionSettings: {
					maxConcurrent: 3,
					defaultModel: 'claude-sonnet',
				},
				toolPermissions: {
					allow: [],
					deny: [],
				},
			},
		}),
		listSkills: vi.fn().mockResolvedValue({
			skills: [],
		}),
		getClaudeMd: vi.fn().mockResolvedValue({
			files: [],
		}),
	},
	decisionClient: {
		resolveDecision: vi.fn().mockResolvedValue({}),
	},
	knowledgeClient: {
		listKnowledge: vi.fn().mockResolvedValue({ entries: [] }),
		getKnowledgeStatus: vi.fn().mockResolvedValue({ status: null }),
	},
	mcpClient: {
		listMCPServers: vi.fn().mockResolvedValue({ servers: [] }),
	},
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
		createdAt: '2024-01-01T00:00:00Z',
		updatedAt: '2024-01-01T00:00:00Z',
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
		createdAt: new Date().toISOString(),
		updatedAt: new Date().toISOString(),
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
	// Environment pages APIs
	getSettingsHierarchy: vi.fn().mockResolvedValue({
		merged: null,
		global: { path: '~/.claude/settings.json', settings: {} },
		project: { path: '.claude/settings.json', settings: {} },
		sources: {},
	}),
	updateSettings: vi.fn(),
	updateGlobalSettings: vi.fn(),
	listPrompts: vi.fn().mockResolvedValue([]),
	getPrompt: vi.fn().mockResolvedValue({ phase: 'implement', content: '', source: 'embedded', variables: [] }),
	getPromptDefault: vi.fn().mockResolvedValue({ phase: 'implement', content: '', source: 'embedded', variables: [] }),
	listScripts: vi.fn().mockResolvedValue([]),
	discoverScripts: vi.fn().mockResolvedValue([]),
	listHooks: vi.fn().mockResolvedValue({}),
	getHookTypes: vi.fn().mockResolvedValue(['PreToolUse', 'PostToolUse']),
	listSkills: vi.fn().mockResolvedValue([]),
	getSkill: vi.fn().mockResolvedValue({ name: 'test', description: '', content: '' }),
	listMCPServers: vi.fn().mockResolvedValue([]),
	getMCPServer: vi.fn().mockResolvedValue({ name: 'test', type: 'stdio', disabled: false }),
	getConfig: vi.fn().mockResolvedValue({
		version: '1.0.0',
		profile: 'auto',
		automation: { profile: 'auto', gates_default: 'auto', retry_enabled: true, retry_max: 3 },
		execution: { model: 'claude-3-opus', max_iterations: 10, timeout: '30m' },
		git: { branch_prefix: 'orc/', commit_prefix: '[orc]' },
		worktree: { enabled: true, dir: '.orc/worktrees', cleanup_on_complete: true, cleanup_on_fail: false },
		completion: { action: 'pr', target_branch: 'main', delete_branch: true },
		timeouts: { phase_max: '1h', turn_max: '10m', idle_warning: '2m', heartbeat_interval: '30s', idle_timeout: '5m' },
	}),
	getConfigWithSources: vi.fn().mockResolvedValue({
		version: '1.0.0',
		profile: 'auto',
		automation: { profile: 'auto', gates_default: 'auto', retry_enabled: true, retry_max: 3 },
		execution: { model: 'claude-3-opus', max_iterations: 10, timeout: '30m' },
		git: { branch_prefix: 'orc/', commit_prefix: '[orc]' },
		worktree: { enabled: true, dir: '.orc/worktrees', cleanup_on_complete: true, cleanup_on_fail: false },
		completion: { action: 'pr', target_branch: 'main', delete_branch: true },
		timeouts: { phase_max: '1h', turn_max: '10m', idle_warning: '2m', heartbeat_interval: '30s', idle_timeout: '5m' },
		sources: {},
	}),
	updateConfig: vi.fn(),
	getClaudeMDHierarchy: vi.fn().mockResolvedValue({
		global: null,
		user: null,
		project: { path: 'CLAUDE.md', content: '# Test', is_global: false, source: 'project' },
		local: [],
	}),
	listToolsByCategory: vi.fn().mockResolvedValue({}),
	getToolPermissions: vi.fn().mockResolvedValue({ allow: [], deny: [] }),
	listAgents: vi.fn().mockResolvedValue([]),
	getAgent: vi.fn().mockResolvedValue({ name: 'test', description: '' }),
	// Config stats for TopBar
	getConfigStats: vi.fn().mockResolvedValue({
		slashCommandsCount: 0,
		claudeMdSize: 0,
		mcpServersCount: 0,
		permissionsProfile: 'default',
	}),
}));

// Test wrapper component
function TestApp() {
	const routeElements = useRoutes(routes);
	return (
		<TooltipProvider delayDuration={0}>
			<EventProvider>{routeElements}</EventProvider>
		</TooltipProvider>
	);
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
		it('redirects to /board', async () => {
			// Set project to see the board (not empty state)
			useProjectStore.setState({
				projects: [
					{
						id: 'test-project',
						path: '/test/project',
						name: 'Test Project',
						createdAt: createTimestamp('2024-01-01T00:00:00Z'),
					} as Project,
				],
				currentProjectId: 'test-project',
				loading: false,
				error: null,
				_isHandlingPopState: false,
			});

			renderWithRouter('/');
			await waitFor(() => {
				// After redirect, should show BoardView
				const boardView = document.querySelector('.board-view');
				expect(boardView).toBeInTheDocument();
			});
		});

		it('renders sidebar navigation', async () => {
			renderWithRouter('/');
			await waitFor(() => {
				// Check sidebar nav links exist by href attribute
				const nav = screen.getByRole('navigation', { name: 'Main navigation' });
				expect(nav.querySelector('a[href="/board"]')).toBeInTheDocument();
				expect(nav.querySelector('a[href="/initiatives"]')).toBeInTheDocument();
				expect(nav.querySelector('a[href="/stats"]')).toBeInTheDocument();
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
						createdAt: createTimestamp('2024-01-01T00:00:00Z'),
					} as Project,
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
				// BoardView renders with the board-view class
				const boardView = document.querySelector('.board-view');
				expect(boardView).toBeInTheDocument();
			});
		});

		it('renders board view component', async () => {
			renderWithRouter('/board');
			await waitFor(() => {
				// BoardView renders with the board-view class
				const boardView = document.querySelector('.board-view');
				expect(boardView).toBeInTheDocument();
			});
		});
	});

	describe('/stats route (was /dashboard)', () => {
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

		it('renders Stats page at /stats', async () => {
			renderWithRouter('/stats');
			await waitFor(() => {
				// StatsView shows "Statistics" title
				expect(screen.getByText('Statistics')).toBeInTheDocument();
			});
		});

		it('redirects /dashboard to /stats', async () => {
			renderWithRouter('/dashboard');
			await waitFor(() => {
				// After redirect, should show Stats page
				expect(screen.getByText('Statistics')).toBeInTheDocument();
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

	describe('/initiatives route', () => {
		it('renders InitiativesView page at /initiatives', async () => {
			renderWithRouter('/initiatives');
			await waitFor(
				() => {
					// InitiativesView shows the header with title and button
					const title = screen.getByRole('heading', { level: 1, name: 'Initiatives' });
					expect(title).toBeInTheDocument();
				},
				{ timeout: 3000 }
			);
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

	describe('/agents route', () => {
		it('renders Agents page', async () => {
			renderWithRouter('/agents');
			// Just verify the route doesn't error and renders something
			await waitFor(() => {
				expect(screen.getByRole('navigation')).toBeInTheDocument();
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

	describe('/settings routes', () => {
		it('redirects /settings to /settings/commands', async () => {
			renderWithRouter('/settings');
			await waitFor(() => {
				// Commands page shows "Slash Commands" in sidebar and content
				expect(screen.getByRole('heading', { level: 1, name: 'Settings' })).toBeInTheDocument();
				// Check that we're on the commands page by looking at the active nav
				const activeLink = document.querySelector('.settings-nav-item--active');
				expect(activeLink).toHaveTextContent('Slash Commands');
			});
		});

		it('renders SettingsView at /settings/commands', async () => {
			renderWithRouter('/settings/commands');
			await waitFor(() => {
				// SettingsView shows "Slash Commands" header
				expect(screen.getByRole('heading', { level: 2, name: 'Slash Commands' })).toBeInTheDocument();
			});
		});

		it('renders ClaudeMdPage at /settings/claude-md', async () => {
			renderWithRouter('/settings/claude-md');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 3, name: 'CLAUDE.md Editor' })).toBeInTheDocument();
			});
		});

		it('renders Mcp at /settings/mcp', async () => {
			renderWithRouter('/settings/mcp');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 3, name: 'MCP Servers' })).toBeInTheDocument();
			});
		});

		it('renders Memory at /settings/memory', async () => {
			renderWithRouter('/settings/memory');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 3, name: 'Memory' })).toBeInTheDocument();
			});
		});

		it('renders placeholder at /settings/permissions', async () => {
			renderWithRouter('/settings/permissions');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 2, name: 'Permissions' })).toBeInTheDocument();
			});
		});

		it('renders placeholder at /settings/projects', async () => {
			renderWithRouter('/settings/projects');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 2, name: 'Projects' })).toBeInTheDocument();
			});
		});

		it('renders placeholder at /settings/billing', async () => {
			renderWithRouter('/settings/billing');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 2, name: 'Billing & Usage' })).toBeInTheDocument();
			});
		});

		it('renders ImportExportPage at /settings/import-export', async () => {
			renderWithRouter('/settings/import-export');
			await waitFor(() => {
				// ImportExportPage renders separate Export and Import sections (h3)
				expect(screen.getByRole('heading', { level: 3, name: 'Export' })).toBeInTheDocument();
			});
		});

		it('renders placeholder at /settings/profile', async () => {
			renderWithRouter('/settings/profile');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 2, name: 'Profile' })).toBeInTheDocument();
			});
		});

		it('renders placeholder at /settings/api-keys', async () => {
			renderWithRouter('/settings/api-keys');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 2, name: 'API Keys' })).toBeInTheDocument();
			});
		});

		it('renders 404 for unknown settings paths', async () => {
			renderWithRouter('/settings/unknown-route');
			await waitFor(() => {
				expect(screen.getByText('Page not found')).toBeInTheDocument();
			});
		});
	});

	describe('Legacy /environment routes redirect to /settings', () => {
		it('redirects /environment to /settings', async () => {
			renderWithRouter('/environment');
			await waitFor(() => {
				// Should redirect to /settings which redirects to /settings/commands
				expect(screen.getByRole('heading', { level: 1, name: 'Settings' })).toBeInTheDocument();
			});
		});

		it('redirects /environment/settings to /settings', async () => {
			renderWithRouter('/environment/settings');
			await waitFor(() => {
				// Should redirect to /settings which redirects to /settings/commands
				expect(screen.getByRole('heading', { level: 1, name: 'Settings' })).toBeInTheDocument();
			});
		});
	});

	describe('URL parameters', () => {
		it('syncs project param from URL to store', async () => {
			renderWithRouter('/board?project=test-project');
			await waitFor(() => {
				expect(useProjectStore.getState().currentProjectId).toBe('test-project');
			});
		});

		it('syncs initiative param from URL to store on initiatives route', async () => {
			renderWithRouter('/initiatives?initiative=INIT-001');
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
			renderWithRouter('/board');
			expect(screen.getByRole('navigation', { name: 'Main navigation' })).toBeInTheDocument();
		});

		it('renders header on all routes', async () => {
			renderWithRouter('/board');
			expect(screen.getByRole('banner')).toBeInTheDocument();
		});

		it('shows settings sidebar navigation on settings routes', async () => {
			renderWithRouter('/settings/commands');
			await waitFor(() => {
				// Should see settings sidebar with navigation groups
				expect(screen.getByRole('navigation', { name: 'Settings navigation' })).toBeInTheDocument();
				// Check for navigation items
				expect(screen.getByRole('link', { name: /Slash Commands/i })).toBeInTheDocument();
				expect(screen.getByRole('link', { name: /CLAUDE\.md/i })).toBeInTheDocument();
			});
		});
	});

	describe('404 route', () => {
		it('renders NotFoundPage for unknown routes', async () => {
			renderWithRouter('/some-unknown-route');
			await waitFor(() => {
				expect(screen.getByText('Page not found')).toBeInTheDocument();
				expect(screen.getByText('404')).toBeInTheDocument();
			});
		});
	});
});
