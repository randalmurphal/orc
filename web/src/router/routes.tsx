/* eslint-disable react-refresh/only-export-components */
import { useState, useCallback, lazy, Suspense } from 'react';
import { RouteObject, Navigate, Outlet } from 'react-router-dom';
import { AppShell } from '@/components/layout/AppShell';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { PageLoader } from '@/components/ui/PageLoader';
import { NewTaskModal, ProjectSwitcher } from '@/components/overlays';
import { useTaskStore } from '@/stores/taskStore';

// Lazy-loaded page components for code splitting
// Each becomes a separate chunk, loaded on-demand when the route is visited
const Board = lazy(() => import('@/pages/Board').then(m => ({ default: m.Board })));
const InitiativesPage = lazy(() => import('@/pages/InitiativesPage').then(m => ({ default: m.InitiativesPage })));
const InitiativeDetailPage = lazy(() => import('@/pages/InitiativeDetailPage').then(m => ({ default: m.InitiativeDetailPage })));
const StatsPage = lazy(() => import('@/pages/StatsPage').then(m => ({ default: m.StatsPage })));
const TimelinePage = lazy(() => import('@/pages/TimelinePage').then(m => ({ default: m.TimelinePage })));
const TaskDetail = lazy(() => import('@/pages/TaskDetail').then(m => ({ default: m.TaskDetail })));
const SettingsPage = lazy(() => import('@/pages/SettingsPage').then(m => ({ default: m.SettingsPage })));
const AgentsView = lazy(() => import('@/components/agents/AgentsView').then(m => ({ default: m.AgentsView })));
const Mcp = lazy(() => import('@/pages/environment/Mcp').then(m => ({ default: m.Mcp })));
const WorkflowsPage = lazy(() => import('@/pages/WorkflowsPage').then(m => ({ default: m.WorkflowsPage })));
const WorkflowEditorPage = lazy(() => import('@/components/workflow-editor').then(m => ({ default: m.WorkflowEditorPage })));
const NotFoundPage = lazy(() => import('@/pages/NotFoundPage').then(m => ({ default: m.NotFoundPage })));

// Settings sub-components (loaded with SettingsPage chunk)
const SettingsView = lazy(() => import('@/components/settings').then(m => ({ default: m.SettingsView })));
const SettingsPlaceholder = lazy(() => import('@/components/settings').then(m => ({ default: m.SettingsPlaceholder })));
const ConstitutionPage = lazy(() => import('@/pages/settings/Constitution').then(m => ({ default: m.ConstitutionPage })));
const ClaudeMdPage = lazy(() => import('@/pages/settings/ClaudeMdPage').then(m => ({ default: m.ClaudeMdPage })));
const ImportExportPage = lazy(() => import('@/pages/settings/ImportExport').then(m => ({ default: m.ImportExportPage })));

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
 * | / | redirects to /board |
 * | /board | ?project, ?initiative, ?dependency_status |
 * | /initiatives | initiatives overview with stats and cards |
 * | /initiatives/:id | - |
 * | /stats | ?project (dashboard stats) |
 * | /agents | - |
 * | /settings/* | various sections |
 * | /tasks/:id | ?tab |
 *
 * Keyboard Shortcuts:
 * - g b -> navigate to /board
 * - g i -> navigate to /initiatives
 * - g s -> navigate to /stats
 * - g a -> navigate to /agents
 * - g , -> navigate to /settings
 */

/**
 * Suspense wrapper for lazy-loaded route components
 */
function LazyRoute({ children }: { children: React.ReactNode }) {
	return <Suspense fallback={<PageLoader />}>{children}</Suspense>;
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

	const handleNewTask = useCallback(() => {
		setShowNewTaskModal(true);
	}, []);

	const handleProjectChange = useCallback(() => {
		setShowProjectSwitcher(true);
	}, []);

	return (
		<>
			<AppShell
				onNewTask={handleNewTask}
				onProjectChange={handleProjectChange}
			>
				<Outlet />
			</AppShell>
			<NewTaskModal
				open={showNewTaskModal}
				onClose={() => setShowNewTaskModal(false)}
				onCreate={(task) => useTaskStore.getState().addTask(task)}
			/>
			<ProjectSwitcher
				open={showProjectSwitcher}
				onClose={() => setShowProjectSwitcher(false)}
			/>
		</>
	);
}

export const routes: RouteObject[] = [
	{
		path: '/',
		element: <AppShellLayout />,
		errorElement: <ErrorBoundary />,
		children: [
			// Index route redirects to board
			{
				index: true,
				element: <Navigate to="/board" replace />,
			},
			// Board - Main kanban board view
			{
				path: 'board',
				element: (
					<LazyRoute>
						<Board />
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
			// Agents - Agent configuration
			{
				path: 'agents',
				element: (
					<LazyRoute>
						<AgentsView />
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
			// Settings - New settings layout with 240px sidebar
			{
				path: 'settings',
				element: (
					<LazyRoute>
						<SettingsPage />
					</LazyRoute>
				),
				children: [
					// Default redirect to commands
					{
						index: true,
						element: <Navigate to="/settings/commands" replace />,
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
			// Legacy environment routes redirect to settings
			{
				path: 'environment',
				element: <Navigate to="/settings" replace />,
			},
			{
				path: 'environment/*',
				element: <Navigate to="/settings" replace />,
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
