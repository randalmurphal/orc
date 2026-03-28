/* eslint-disable react-refresh/only-export-components */
import { useState, useCallback, lazy, Suspense, useEffect } from 'react';
import { RouteObject, Navigate, Outlet, useParams, useLocation } from 'react-router-dom';
import { AppShell } from '@/components/layout/AppShell';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { PageLoader } from '@/components/ui/PageLoader';
import { CommandPalette, NewTaskWorkflowModal, ProjectSwitcher } from '@/components/overlays';
import { useGlobalShortcuts } from '@/hooks';
import { useTaskStore } from '@/stores/taskStore';
import { useProjectLoading } from '@/stores';
import type { Task } from '@/gen/orc/v1/task_pb';

// Lazy-loaded page components for code splitting
// Each becomes a separate chunk, loaded on-demand when the route is visited
const Board = lazy(() => import('@/pages/Board').then(m => ({ default: m.Board })));
const InitiativesPage = lazy(() => import('@/pages/InitiativesPage').then(m => ({ default: m.InitiativesPage })));
const InitiativeDetailPage = lazy(() => import('@/pages/InitiativeDetailPage').then(m => ({ default: m.InitiativeDetailPage })));
const StatsPage = lazy(() => import('@/pages/StatsPage').then(m => ({ default: m.StatsPage })));
const TimelinePage = lazy(() => import('@/pages/TimelinePage').then(m => ({ default: m.TimelinePage })));
const TaskDetail = lazy(() => import('@/pages/TaskDetail').then(m => ({ default: m.TaskDetail })));
const SettingsPage = lazy(() => import('@/pages/SettingsPage').then(m => ({ default: m.SettingsPage })));
const Mcp = lazy(() => import('@/pages/environment/Mcp').then(m => ({ default: m.Mcp })));
const WorkflowsPage = lazy(() => import('@/pages/WorkflowsPage').then(m => ({ default: m.WorkflowsPage })));
const WorkflowEditorPage = lazy(() => import('@/components/workflow-editor').then(m => ({ default: m.WorkflowEditorPage })));
const NotFoundPage = lazy(() => import('@/pages/NotFoundPage').then(m => ({ default: m.NotFoundPage })));
const MyWorkPage = lazy(() => import('@/pages/MyWorkPage').then(m => ({ default: m.MyWorkPage })));
const ProjectHomePage = lazy(() => import('@/pages/ProjectHomePage').then(m => ({ default: m.ProjectHomePage })));
const RecommendationsPage = lazy(() => import('@/pages/RecommendationsPage').then(m => ({ default: m.RecommendationsPage })));

// Environment pages (accessed via Settings navigation)
const EnvHooks = lazy(() => import('@/pages/environment/Hooks').then(m => ({ default: m.Hooks })));
const EnvSkills = lazy(() => import('@/pages/environment/Skills').then(m => ({ default: m.Skills })));
const EnvConfig = lazy(() => import('@/pages/environment/Config').then(m => ({ default: m.Config })));
const EnvTools = lazy(() => import('@/pages/environment/Tools').then(m => ({ default: m.Tools })));

// Settings sub-components (loaded with SettingsPage chunk)
// SettingsTabs is used internally by SettingsPage, not directly in routes
const SettingsLayout = lazy(() => import('@/components/settings').then(m => ({ default: m.SettingsLayout })));
const SettingsView = lazy(() => import('@/components/settings').then(m => ({ default: m.SettingsView })));
const SettingsPlaceholder = lazy(() => import('@/components/settings').then(m => ({ default: m.SettingsPlaceholder })));
const ConstitutionPage = lazy(() => import('@/pages/settings/Constitution').then(m => ({ default: m.ConstitutionPage })));
const ClaudeMdPage = lazy(() => import('@/pages/settings/ClaudeMdPage').then(m => ({ default: m.ClaudeMdPage })));
const ImportExportPage = lazy(() => import('@/pages/settings/ImportExport').then(m => ({ default: m.ImportExportPage })));
const GitSettingsPage = lazy(() => import('@/pages/settings/GitSettings').then(m => ({ default: m.GitSettingsPage })));

// Agents page
const AgentsView = lazy(() => import('@/components/agents').then(m => ({ default: m.AgentsView })));

// Environment layout
const EnvironmentLayout = lazy(() => import('@/pages/environment/EnvironmentLayout').then(m => ({ default: m.EnvironmentLayout })));

// Legacy pages (lower priority, separate chunks)
const AutomationPage = lazy(() => import('@/pages/AutomationPage').then(m => ({ default: m.AutomationPage })));
const Branches = lazy(() => import('@/pages/Branches').then(m => ({ default: m.Branches })));
const Preferences = lazy(() => import('@/pages/Preferences').then(m => ({ default: m.Preferences })));

/**
 * Application Routes
 *
 * URL Parameters by Route:
 *
 * | Route | Params |
 * |-------|--------|
 * | / | My Work dashboard |
 * | /board | ?project, ?initiative, ?dependency_status |
 * | /initiatives | initiatives overview with stats and cards |
 * | /initiatives/:id | - |
 * | /stats | ?project (dashboard stats) |
 * | /settings/* | various sections |
 * | /tasks/:id | ?tab |
 *
 * Keyboard Shortcuts:
 * - g b -> navigate to /board
 * - g i -> navigate to /initiatives
 * - g s -> navigate to /stats
 * - g , -> navigate to /settings
 */

/**
 * Suspense wrapper for lazy-loaded route components
 */
function LazyRoute({ children }: { children: React.ReactNode }) {
	return <Suspense fallback={<PageLoader />}>{children}</Suspense>;
}

/**
 * Redirect from singular /task/:id to plural /tasks/:id
 * Preserves query parameters (e.g., ?tab=transcript)
 */
function TaskRedirect() {
	const { id } = useParams<{ id: string }>();
	const location = useLocation();
	return <Navigate to={`/tasks/${id}${location.search}`} replace />;
}

/**
 * Root layout wrapper that renders AppShellLayout with navigation.
 * My Work page serves as the landing page regardless of project selection state.
 */
function RootLayout() {
	const loading = useProjectLoading();

	// Show loader while projects are being fetched
	if (loading) {
		return <PageLoader />;
	}

	return <AppShellLayout />;
}

/**
 * AppShell wrapper component that provides the main layout structure.
 *
 * Renders:
 * - IconNav (56px sidebar)
 * - TopBar (48px header)
 * - Main content area with Outlet
 * - RightPanel (300px, collapsible)
 * - Modals for New Task and Project Switcher
 */
function AppShellLayout() {
	const [showNewTaskModal, setShowNewTaskModal] = useState(false);
	const [showProjectSwitcher, setShowProjectSwitcher] = useState(false);
	const [showCommandPalette, setShowCommandPalette] = useState(false);

	const handleNewTask = useCallback(() => {
		setShowNewTaskModal(true);
	}, []);

	const handleProjectChange = useCallback(() => {
		setShowProjectSwitcher(true);
	}, []);

	const handleCommandPalette = useCallback(() => {
		setShowCommandPalette(true);
	}, []);

	useGlobalShortcuts({
		onCommandPalette: handleCommandPalette,
	});

	useEffect(() => {
		const handleCommandPaletteEvent = () => {
			setShowCommandPalette(true);
		};
		const handleNewTaskEvent = () => {
			setShowNewTaskModal(true);
		};
		const handleProjectSwitcherEvent = () => {
			setShowProjectSwitcher(true);
		};
		const handleMetaShortcut = (event: KeyboardEvent) => {
			if (!(event.metaKey || event.ctrlKey) || event.key.toLowerCase() !== 'k') {
				return;
			}
			const target = event.target as HTMLElement | null;
			if (
				target &&
				(target.tagName === 'INPUT' ||
					target.tagName === 'TEXTAREA' ||
					target.tagName === 'SELECT' ||
					target.isContentEditable)
			) {
				return;
			}
			event.preventDefault();
			setShowCommandPalette(true);
		};

		window.addEventListener('orc:command-palette', handleCommandPaletteEvent);
		window.addEventListener('orc:new-task', handleNewTaskEvent);
		window.addEventListener('orc:project-switcher', handleProjectSwitcherEvent);
		document.addEventListener('keydown', handleMetaShortcut);

		return () => {
			window.removeEventListener('orc:command-palette', handleCommandPaletteEvent);
			window.removeEventListener('orc:new-task', handleNewTaskEvent);
			window.removeEventListener('orc:project-switcher', handleProjectSwitcherEvent);
			document.removeEventListener('keydown', handleMetaShortcut);
		};
	}, []);

	return (
		<>
			<AppShell
				onNewTask={handleNewTask}
				onProjectChange={handleProjectChange}
			>
				<Outlet />
			</AppShell>
			<NewTaskWorkflowModal
				open={showNewTaskModal}
				onClose={() => setShowNewTaskModal(false)}
				onCreate={(task: Task) => useTaskStore.getState().addTask(task)}
			/>
			<ProjectSwitcher
				open={showProjectSwitcher}
				onClose={() => setShowProjectSwitcher(false)}
			/>
			<CommandPalette
				open={showCommandPalette}
				onClose={() => setShowCommandPalette(false)}
			/>
		</>
	);
}

export const routes: RouteObject[] = [
	{
		path: '/',
		element: <RootLayout />,
		errorElement: <ErrorBoundary />,
		children: [
			// Index route - My Work dashboard
			{
				index: true,
				element: (
					<LazyRoute>
						<MyWorkPage />
					</LazyRoute>
				),
			},
			// Board - Main kanban board view
			{
				path: 'project',
				element: (
					<LazyRoute>
						<ProjectHomePage />
					</LazyRoute>
				),
			},
			{
				path: 'board',
				element: (
					<LazyRoute>
						<Board />
					</LazyRoute>
				),
			},
			{
				path: 'recommendations',
				element: (
					<LazyRoute>
						<RecommendationsPage />
					</LazyRoute>
				),
			},
			// Initiatives - Overview page with stats and cards
			{
				path: 'initiatives',
				element: (
					<LazyRoute>
						<InitiativesPage />
					</LazyRoute>
				),
			},
			// Initiative detail
			{
				path: 'initiatives/:id',
				element: (
					<LazyRoute>
						<InitiativeDetailPage />
					</LazyRoute>
				),
			},
			// Timeline - Activity timeline feed
			{
				path: 'timeline',
				element: (
					<LazyRoute>
						<TimelinePage />
					</LazyRoute>
				),
			},
			// Stats - Statistics overview with analytics
			{
				path: 'stats',
				element: (
					<LazyRoute>
						<StatsPage />
					</LazyRoute>
				),
			},
			// Workflows - Workflow and phase template configuration
			{
				path: 'workflows',
				element: (
					<LazyRoute>
						<WorkflowsPage />
					</LazyRoute>
				),
			},
			// Workflow Editor - Visual pipeline editor
			{
				path: 'workflows/:id',
				element: (
					<LazyRoute>
						<WorkflowEditorPage />
					</LazyRoute>
				),
			},
			// Settings - Tabbed layout with General, Agents, Environment tabs
			{
				path: 'settings',
				element: (
					<LazyRoute>
						<SettingsPage />
					</LazyRoute>
				),
				children: [
					// Default redirect to general tab
					{
						index: true,
						element: <Navigate to="/settings/general" replace />,
					},
					// General tab - SettingsLayout with sidebar navigation
					{
						path: 'general',
						element: (
							<LazyRoute>
								<SettingsLayout />
							</LazyRoute>
						),
						children: [
							// Default redirect to commands
							{
								index: true,
								element: <Navigate to="/settings/general/commands" replace />,
							},
							// CLAUDE CODE section
							{
								path: 'commands',
								element: (
									<LazyRoute>
										<SettingsView />
									</LazyRoute>
								),
							},
							{
								path: 'claude-md',
								element: (
									<LazyRoute>
										<ClaudeMdPage />
									</LazyRoute>
								),
							},
							{
								path: 'mcp',
								element: (
									<LazyRoute>
										<Mcp />
									</LazyRoute>
								),
							},
							{
								path: 'permissions',
								element: (
									<LazyRoute>
										<SettingsPlaceholder
											title="Permissions"
											description="Configure tool permissions and access controls"
											icon="shield"
										/>
									</LazyRoute>
								),
							},
							// ORC section
							{
								path: 'projects',
								element: (
									<LazyRoute>
										<SettingsPlaceholder
											title="Projects"
											description="Manage your ORC projects and repositories"
											icon="folder"
										/>
									</LazyRoute>
								),
							},
							{
								path: 'git',
								element: (
									<LazyRoute>
										<GitSettingsPage />
									</LazyRoute>
								),
							},
							{
								path: 'billing',
								element: (
									<LazyRoute>
										<SettingsPlaceholder
											title="Billing & Usage"
											description="View your usage statistics and billing information"
											icon="dollar"
										/>
									</LazyRoute>
								),
							},
							{
								path: 'import-export',
								element: (
									<LazyRoute>
										<ImportExportPage />
									</LazyRoute>
								),
							},
							{
								path: 'constitution',
								element: (
									<LazyRoute>
										<ConstitutionPage />
									</LazyRoute>
								),
							},
							// ACCOUNT section
							{
								path: 'profile',
								element: (
									<LazyRoute>
										<SettingsPlaceholder
											title="Profile"
											description="Manage your account profile and preferences"
											icon="user"
										/>
									</LazyRoute>
								),
							},
							{
								path: 'api-keys',
								element: (
									<LazyRoute>
										<SettingsPlaceholder
											title="API Keys"
											description="Manage your API keys and authentication tokens"
											icon="settings"
										/>
									</LazyRoute>
								),
							},
							// 404 for unknown general settings paths
							{
								path: '*',
								element: (
									<LazyRoute>
										<NotFoundPage />
									</LazyRoute>
								),
							},
						],
					},
					// Agents tab - AgentsView
					{
						path: 'agents',
						element: (
							<LazyRoute>
								<AgentsView />
							</LazyRoute>
						),
					},
					// Environment tab - EnvironmentLayout with sub-navigation
					{
						path: 'environment',
						element: (
							<LazyRoute>
								<EnvironmentLayout />
							</LazyRoute>
						),
						children: [
							// Default redirect to hooks
							{
								index: true,
								element: <Navigate to="/settings/environment/hooks" replace />,
							},
							{
								path: 'hooks',
								element: <LazyRoute><EnvHooks /></LazyRoute>,
							},
							{
								path: 'skills',
								element: <LazyRoute><EnvSkills /></LazyRoute>,
							},
							{
								path: 'tools',
								element: <LazyRoute><EnvTools /></LazyRoute>,
							},
							{
								path: 'config',
								element: <LazyRoute><EnvConfig /></LazyRoute>,
							},
							// 404 for unknown environment paths
							{
								path: '*',
								element: (
									<LazyRoute>
										<NotFoundPage />
									</LazyRoute>
								),
							},
						],
					},
					// 404 for unknown settings paths
					{
						path: '*',
						element: (
							<LazyRoute>
								<NotFoundPage />
							</LazyRoute>
						),
					},
				],
			},
			// Task detail (existing route)
			{
				path: 'tasks/:id',
				element: (
					<LazyRoute>
						<TaskDetail />
					</LazyRoute>
				),
			},
			// Redirect singular /task/:id to plural /tasks/:id
			{
				path: 'task/:id',
				element: <TaskRedirect />,
			},
			// Legacy routes with redirects
			{
				path: 'dashboard',
				element: <Navigate to="/stats" replace />,
			},
			{
				path: 'automation',
				element: (
					<LazyRoute>
						<AutomationPage />
					</LazyRoute>
				),
			},
			{
				path: 'branches',
				element: (
					<LazyRoute>
						<Branches />
					</LazyRoute>
				),
			},
			{
				path: 'preferences',
				element: (
					<LazyRoute>
						<Preferences />
					</LazyRoute>
				),
			},
			// 404 catch-all
			{
				path: '*',
				element: (
					<LazyRoute>
						<NotFoundPage />
					</LazyRoute>
				),
			},
		],
	},
];
