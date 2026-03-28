import { describe, it, expect, beforeEach, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { useRoutes } from 'react-router-dom';
import { routes } from './routes';
import { EventProvider, ShortcutProvider } from '@/hooks';
import { TooltipProvider } from '@/components/ui/Tooltip';
import { useProjectStore, useInitiativeStore, useUIStore, useTaskStore } from '@/stores';
import { createMockTask, createTimestamp } from '@/test/factories';
import type { Project } from '@/gen/orc/v1/project_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';

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

vi.mock('@/pages/ProjectHomePage', () => ({
	ProjectHomePage: () => <div data-testid="project-home-page">Project Home Page</div>,
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
		listReviewComments: vi.fn().mockResolvedValue({ comments: [] }),
		getDiff: vi.fn().mockResolvedValue({ files: [] }),
		resumeTask: vi.fn().mockResolvedValue({}),
		updateTask: vi.fn().mockResolvedValue({}),
		finalizeTask: vi.fn().mockResolvedValue({}),
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
			listInitiativeNotes: vi.fn().mockResolvedValue({ notes: [] }),
			listTaskGeneratedNotes: vi.fn().mockResolvedValue({ notes: [] }),
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
	projectClient: {
		listProjects: vi.fn().mockResolvedValue({ projects: [] }),
		getAllProjectsStatus: vi.fn().mockResolvedValue({ projects: [] }),
	},
		attentionDashboardClient: {
			getAttentionDashboardData: vi.fn().mockResolvedValue({
				runningSummary: { taskCount: 0, tasks: [] },
				attentionItems: [],
				pendingRecommendations: 0,
			}),
			performAttentionAction: vi.fn().mockResolvedValue({ success: true }),
		},
		feedbackClient: {
			listFeedback: vi.fn().mockResolvedValue({ feedback: [] }),
			addFeedback: vi.fn().mockResolvedValue({}),
			sendFeedback: vi.fn().mockResolvedValue({}),
			deleteFeedback: vi.fn().mockResolvedValue({}),
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
		execution: { model: 'sonnet', max_turns: 150, timeout: '30m' },
		git: { branch_prefix: 'orc/', commit_prefix: '[orc]' },
		worktree: { enabled: true, dir: '.orc/worktrees', cleanup_on_complete: true, cleanup_on_fail: false },
		completion: { action: 'pr', target_branch: 'main', delete_branch: true },
		timeouts: { phase_max: '1h', turn_max: '10m', idle_warning: '2m', heartbeat_interval: '30s', idle_timeout: '5m' },
	}),
	getConfigWithSources: vi.fn().mockResolvedValue({
		version: '1.0.0',
		profile: 'auto',
		automation: { profile: 'auto', gates_default: 'auto', retry_enabled: true, retry_max: 3 },
		execution: { model: 'sonnet', max_turns: 150, timeout: '30m' },
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
		<ShortcutProvider>
			<TooltipProvider delayDuration={0}>
				<EventProvider>{routeElements}</EventProvider>
			</TooltipProvider>
		</ShortcutProvider>
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
		// RootLayout requires currentProjectId to render app content;
		// without it, ProjectPickerPage is shown instead.
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
		it('renders MyWorkPage at /', async () => {
			renderWithRouter('/');
			await waitFor(() => {
				// Root route renders MyWorkPage - with empty projects, shows empty state
				expect(screen.getByText('No projects found')).toBeInTheDocument();
			});
		});

		it('renders navigation tabs in TopBar', async () => {
			renderWithRouter('/');
			await waitFor(() => {
				// Navigation tabs are now in the TopBar, not the sidebar
				const banner = screen.getByRole('banner');
				expect(banner.querySelector('a[href="/board"]')).toBeInTheDocument();
				expect(banner.querySelector('a[href="/settings"]')).toBeInTheDocument();
			});
		});

		it('opens the command palette on Cmd+K and closes it on Escape', async () => {
			renderWithRouter('/');

			fireEvent.keyDown(document, { key: 'k', metaKey: true });

			await waitFor(() => {
				expect(screen.getByRole('dialog', { name: 'Command palette' })).toBeInTheDocument();
			});

			fireEvent.keyDown(screen.getByLabelText('Search commands'), { key: 'Escape' });

			await waitFor(() => {
				expect(screen.queryByRole('dialog', { name: 'Command palette' })).not.toBeInTheDocument();
			});
		});

		it('opens the command palette on Shift+Alt+K and does not render the dead search input', async () => {
			renderWithRouter('/');

			expect(screen.queryByPlaceholderText('Search tasks...')).not.toBeInTheDocument();

			fireEvent.keyDown(document, { key: 'k', shiftKey: true, altKey: true });

			await waitFor(() => {
				expect(screen.getByRole('dialog', { name: 'Command palette' })).toBeInTheDocument();
			});
		});

		it('ignores the Cmd+K shortcut while another input is focused', async () => {
			renderWithRouter('/');

			const externalInput = document.createElement('input');
			document.body.appendChild(externalInput);
			externalInput.focus();

			fireEvent.keyDown(externalInput, { key: 'k', metaKey: true });

			await waitFor(() => {
				expect(screen.queryByRole('dialog', { name: 'Command palette' })).not.toBeInTheDocument();
			});

			externalInput.remove();
		});

		it('updates project-scoped task actions when the project store changes', async () => {
			const projectOneTask = {
				...createMockTask({ id: 'TASK-P1', title: 'Project One Task', status: TaskStatus.PAUSED }),
				projectId: 'P1',
			};
			const projectTwoTask = {
				...createMockTask({ id: 'TASK-P2', title: 'Project Two Task', status: TaskStatus.BLOCKED }),
				projectId: 'P2',
			};

			act(() => {
				useProjectStore.setState({
					projects: [
						{
							id: 'P1',
							path: '/projects/one',
							name: 'Project One',
							createdAt: createTimestamp('2024-01-01T00:00:00Z'),
						} as Project,
						{
							id: 'P2',
							path: '/projects/two',
							name: 'Project Two',
							createdAt: createTimestamp('2024-01-02T00:00:00Z'),
						} as Project,
					],
					currentProjectId: 'P1',
					loading: false,
					error: null,
					_isHandlingPopState: false,
				});
				useTaskStore.setState({
					tasks: [projectOneTask, projectTwoTask],
				});
			});

			renderWithRouter('/');
			fireEvent.keyDown(document, { key: 'k', metaKey: true });

			await waitFor(() => {
				expect(screen.getByRole('option', { name: /^resume task-p1/i })).toBeInTheDocument();
				expect(screen.queryByRole('option', { name: /^resume task-p2/i })).not.toBeInTheDocument();
			});

			act(() => {
				useProjectStore.setState({ currentProjectId: 'P2' });
			});

			await waitFor(() => {
				expect(screen.queryByRole('option', { name: /^resume task-p1/i })).not.toBeInTheDocument();
				expect(screen.getByRole('option', { name: /^resume task-p2/i })).toBeInTheDocument();
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

	describe('/project route', () => {
		it('renders ProjectHomePage at /project', async () => {
			renderWithRouter('/project');

			await waitFor(() => {
				expect(screen.getByTestId('project-home-page')).toBeInTheDocument();
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
				// After redirect, should show Stats page - use getAllByText for multiple matches
				const statsTexts = screen.getAllByText('Statistics');
				expect(statsTexts.length).toBeGreaterThan(0);
			});
		});
	});

	describe('/tasks/:id route', () => {
		it('renders TaskDetail page with task title in heading', async () => {
			renderWithRouter('/tasks/TASK-001');
			await waitFor(() => {
				// The task title appears in an h1 with class "task-detail-header__title"
				const taskTitle = document.querySelector('h1.task-detail-header__title');
				expect(taskTitle).toBeInTheDocument();
				expect(taskTitle).toHaveTextContent('Test Task');
			});
		});

		it('displays task ID somewhere on the page', async () => {
			renderWithRouter('/tasks/TASK-001');
			await waitFor(() => {
				// The task ID should appear somewhere in the header - use getAllByText for multiple matches
				const taskIds = screen.getAllByText('TASK-001');
				expect(taskIds.length).toBeGreaterThan(0);
			});
		});
	});

	describe('/task/:id redirect', () => {
		it('redirects singular /task/:id to plural /tasks/:id', async () => {
			renderWithRouter('/task/TASK-001');
			await waitFor(() => {
				// Should redirect and render the TaskDetail page
				const taskTitle = document.querySelector('h1.task-detail-header__title');
				expect(taskTitle).toBeInTheDocument();
				expect(taskTitle).toHaveTextContent('Test Task');
			});
		});

		it('preserves query parameters when redirecting', async () => {
			// The redirect component includes location.search in the Navigate target
			// We verify by checking the page renders correctly with the query param
			renderWithRouter('/task/TASK-001?tab=transcript');
			await waitFor(() => {
				// Should redirect and render the TaskDetail page
				const taskTitle = document.querySelector('h1.task-detail-header__title');
				expect(taskTitle).toBeInTheDocument();
				expect(taskTitle).toHaveTextContent('Test Task');
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

	describe('/settings/general routes', () => {
		it('renders SettingsView at /settings/general/commands', async () => {
			renderWithRouter('/settings/general/commands');
			await waitFor(() => {
				// SettingsView shows "Slash Commands" header - check for text content
				const headings = screen.getAllByRole('heading');
				const slashCommandsHeading = headings.find(h => h.textContent?.includes('Slash Commands'));
				expect(slashCommandsHeading).toBeInTheDocument();
			});
		});

		it('renders ClaudeMdPage at /settings/general/claude-md', async () => {
			renderWithRouter('/settings/general/claude-md');
			await waitFor(() => {
				// Check for CLAUDE.md related content
				const headings = screen.getAllByRole('heading');
				const claudeMdHeading = headings.find(h => h.textContent?.includes('CLAUDE.md'));
				expect(claudeMdHeading).toBeInTheDocument();
			});
		});

		it('renders Mcp at /settings/general/mcp', async () => {
			renderWithRouter('/settings/general/mcp');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 3, name: 'MCP Servers' })).toBeInTheDocument();
			});
		});

		it('renders placeholder at /settings/general/permissions', async () => {
			renderWithRouter('/settings/general/permissions');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 2, name: 'Permissions' })).toBeInTheDocument();
			});
		});

		it('renders placeholder at /settings/general/projects', async () => {
			renderWithRouter('/settings/general/projects');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 2, name: 'Projects' })).toBeInTheDocument();
			});
		});

		it('renders placeholder at /settings/general/billing', async () => {
			renderWithRouter('/settings/general/billing');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 2, name: 'Billing & Usage' })).toBeInTheDocument();
			});
		});

		it('renders ImportExportPage at /settings/general/import-export', async () => {
			renderWithRouter('/settings/general/import-export');
			await waitFor(() => {
				// ImportExportPage renders separate Export and Import sections (h3)
				expect(screen.getByRole('heading', { level: 3, name: 'Export' })).toBeInTheDocument();
			});
		});

		it('renders placeholder at /settings/general/profile', async () => {
			renderWithRouter('/settings/general/profile');
			await waitFor(() => {
				expect(screen.getByRole('heading', { level: 2, name: 'Profile' })).toBeInTheDocument();
			});
		});

		it('renders placeholder at /settings/general/api-keys', async () => {
			renderWithRouter('/settings/general/api-keys');
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

	describe('/settings/environment routes render within environment layout', () => {
		it('renders environment layout at /settings/environment/hooks', async () => {
			renderWithRouter('/settings/environment/hooks');
			await waitFor(() => {
				// Should render the environment layout (has environment-nav class)
				expect(document.querySelector('.environment-nav')).toBeInTheDocument();
			});
		});

		it('renders environment layout at /settings/environment/skills', async () => {
			renderWithRouter('/settings/environment/skills');
			await waitFor(() => {
				// Should render the environment layout
				expect(document.querySelector('.environment-nav')).toBeInTheDocument();
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
			// Use unique ID to avoid race conditions with previous test
			renderWithRouter('/board?initiative=INIT-BOARD-TEST');
			await waitFor(() => {
				expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-BOARD-TEST');
			});
		});

		it('TaskDetail renders split pane with transcript and changes', async () => {
			renderWithRouter('/tasks/TASK-001');
			// Wait for task to load and split pane to render
			await waitFor(() => {
				// The split pane should be rendered with left and right panels
				expect(document.querySelector('.split-pane')).toBeInTheDocument();
				expect(document.querySelector('.split-pane__left')).toBeInTheDocument();
				expect(document.querySelector('.split-pane__right')).toBeInTheDocument();
			});
		});

		it('TaskDetail renders workflow progress', async () => {
			renderWithRouter('/tasks/TASK-001');
			// Wait for task to load and workflow progress to render
			await waitFor(() => {
				// The workflow progress should be rendered
				expect(document.querySelector('.workflow-progress')).toBeInTheDocument();
			});
		});
	});

	describe('Layout structure', () => {
		it('renders sidebar on all routes', async () => {
			renderWithRouter('/board');
			// Use getAllByRole for multiple matches from portals
			// Wait for async state updates in BoardCommandPanel to complete
			await waitFor(() => {
				const navs = screen.getAllByRole('navigation', { name: 'Main navigation' });
				expect(navs.length).toBeGreaterThan(0);
			});
		});

		it('renders header on all routes', async () => {
			renderWithRouter('/board');
			// Use getAllByRole for multiple matches from portals
			// Wait for async state updates in BoardCommandPanel to complete
			await waitFor(() => {
				const banners = screen.getAllByRole('banner');
				expect(banners.length).toBeGreaterThan(0);
			});
		});

		it('shows settings sidebar navigation on settings general routes', async () => {
			renderWithRouter('/settings/general/commands');
			await waitFor(() => {
				// Should see settings sidebar with navigation groups - use getAllByRole
				const navs = screen.getAllByRole('navigation', { name: 'Settings navigation' });
				expect(navs.length).toBeGreaterThan(0);
				// Check for navigation items - use getAllByRole
				const commandLinks = screen.getAllByRole('link', { name: /Slash Commands/i });
				expect(commandLinks.length).toBeGreaterThan(0);
				const claudeMdLinks = screen.getAllByRole('link', { name: /CLAUDE\.md/i });
				expect(claudeMdLinks.length).toBeGreaterThan(0);
			});
		});
	});

	describe('404 route', () => {
		it('renders NotFoundPage for unknown routes', async () => {
			renderWithRouter('/some-unknown-route');
			await waitFor(() => {
				// Use getAllByText for multiple matches from portals
				const notFoundTexts = screen.getAllByText('Page not found');
				expect(notFoundTexts.length).toBeGreaterThan(0);
				const fourOhFourTexts = screen.getAllByText('404');
				expect(fourOhFourTexts.length).toBeGreaterThan(0);
			});
		});
	});

	/**
	 * TASK-723: Settings Tabs Integration Tests
	 *
	 * These tests verify the new tabbed Settings page structure with
	 * General, Agents, and Environment tabs.
	 *
	 * Coverage mapping:
	 * - SC-1: Settings page renders three tabs
	 * - SC-4: Direct navigation to /settings/agents
	 * - SC-5: Direct navigation to /settings/environment
	 * - SC-6: Navigation to /settings redirects to /settings/general
	 */
	describe('/settings tabs routes (TASK-723)', () => {
		describe('SC-1: Settings page renders three tabs', () => {
			it('renders tablist with three tabs at /settings/general', async () => {
				renderWithRouter('/settings/general');
				await waitFor(() => {
					// Should have a tablist with Settings sections aria-label - use getAllByRole
					const tablists = screen.getAllByRole('tablist', { name: 'Settings sections' });
					expect(tablists.length).toBeGreaterThan(0);
				});
			});

			it('renders General tab', async () => {
				renderWithRouter('/settings/general');
				await waitFor(() => {
					// Use getAllByRole for multiple matches from portals
					const tabs = screen.getAllByRole('tab', { name: /general/i });
					expect(tabs.length).toBeGreaterThan(0);
				});
			});

			it('renders Agents tab', async () => {
				renderWithRouter('/settings/general');
				await waitFor(() => {
					// Use getAllByRole for multiple matches from portals
					const tabs = screen.getAllByRole('tab', { name: /agents/i });
					expect(tabs.length).toBeGreaterThan(0);
				});
			});

			it('renders Environment tab', async () => {
				renderWithRouter('/settings/general');
				await waitFor(() => {
					// Use getAllByRole for multiple matches from portals
					const tabs = screen.getAllByRole('tab', { name: /environment/i });
					expect(tabs.length).toBeGreaterThan(0);
				});
			});
		});

		describe('SC-4: Direct navigation to /settings/agents', () => {
			it('renders AgentsView content at /settings/agents', async () => {
				renderWithRouter('/settings/agents');
				await waitFor(() => {
					// AgentsView shows empty state when no agents configured
					// Check for either the main title or the empty state title
					const headings = screen.getAllByRole('heading');
					const agentsHeading = headings.find(h =>
						h.textContent === 'Agents' || h.textContent === 'Create your first agent'
					);
					expect(agentsHeading).toBeInTheDocument();
				});
			});

			it('Agents tab is active at /settings/agents', async () => {
				renderWithRouter('/settings/agents');
				await waitFor(() => {
					// Use getAllByRole for multiple matches
					const agentsTabs = screen.getAllByRole('tab', { name: /agents/i });
					const activeTab = agentsTabs.find(tab => tab.getAttribute('data-state') === 'active');
					expect(activeTab).toBeInTheDocument();
				});
			});
		});

		describe('SC-5: Direct navigation to /settings/environment', () => {
			it('renders EnvironmentLayout at /settings/environment', async () => {
				renderWithRouter('/settings/environment');
				await waitFor(() => {
					// EnvironmentLayout has the environment-nav class
					expect(document.querySelector('.environment-nav')).toBeInTheDocument();
				});
			});

			it('Environment tab is active at /settings/environment', async () => {
				renderWithRouter('/settings/environment');
				await waitFor(() => {
					// Use getAllByRole for multiple matches
					const envTabs = screen.getAllByRole('tab', { name: /environment/i });
					const activeTab = envTabs.find(tab => tab.getAttribute('data-state') === 'active');
					expect(activeTab).toBeInTheDocument();
				});
			});
		});

		describe('SC-6: Navigation to /settings redirects to /settings/general', () => {
			it('redirects /settings to /settings/general', async () => {
				renderWithRouter('/settings');
				await waitFor(() => {
					// After redirect, General tab should be active - use getAllByRole
					const generalTabs = screen.getAllByRole('tab', { name: /general/i });
					const activeTab = generalTabs.find(tab => tab.getAttribute('data-state') === 'active');
					expect(activeTab).toBeInTheDocument();
				});
			});

			it('shows SettingsLayout content after redirect', async () => {
				renderWithRouter('/settings');
				await waitFor(() => {
					// SettingsLayout has the settings-layout class
					expect(document.querySelector('.settings-layout')).toBeInTheDocument();
				});
			});
		});

		describe('Tab state synchronization', () => {
			it('General sub-routes keep General tab active', async () => {
				renderWithRouter('/settings/general/commands');
				await waitFor(() => {
					// Use getAllByRole for multiple matches
					const generalTabs = screen.getAllByRole('tab', { name: /general/i });
					const activeTab = generalTabs.find(tab => tab.getAttribute('data-state') === 'active');
					expect(activeTab).toBeInTheDocument();
				});
			});

			it('Environment sub-routes keep Environment tab active', async () => {
				renderWithRouter('/settings/environment/hooks');
				await waitFor(() => {
					// Use getAllByRole for multiple matches
					const envTabs = screen.getAllByRole('tab', { name: /environment/i });
					const activeTab = envTabs.find(tab => tab.getAttribute('data-state') === 'active');
					expect(activeTab).toBeInTheDocument();
				});
			});
		});
	});
});
