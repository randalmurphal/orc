import { RouteObject, Navigate, Outlet } from 'react-router-dom';
import { AppShell } from '@/components/layout/AppShell';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { Board } from '@/pages/Board';
import { Dashboard } from '@/pages/Dashboard';
import { InitiativesPage } from '@/pages/InitiativesPage';
import { TaskDetail } from '@/pages/TaskDetail';
import { InitiativeDetail } from '@/pages/InitiativeDetail';
import { AutomationPage } from '@/pages/AutomationPage';
import { Branches } from '@/pages/Branches';
import { Preferences } from '@/pages/Preferences';
import { NotFoundPage } from '@/pages/NotFoundPage';
import { SettingsPage } from '@/pages/SettingsPage';
import { SettingsView, SettingsPlaceholder } from '@/components/settings';
import { Agents } from '@/pages/environment/Agents';

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
 * AppShell wrapper component that provides the main layout structure.
 *
 * Renders:
 * - IconNav (56px sidebar)
 * - TopBar (48px header)
 * - Main content area with Outlet
 * - RightPanel (300px, collapsible)
 */
function AppShellLayout() {
	return (
		<AppShell>
			<Outlet />
		</AppShell>
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
				element: <Board />,
			},
			// Initiatives - Overview page with stats and cards
			{
				path: 'initiatives',
				element: <InitiativesPage />,
			},
			// Initiative detail
			{
				path: 'initiatives/:id',
				element: <InitiativeDetail />,
			},
			// Stats - Dashboard with analytics
			{
				path: 'stats',
				element: <Dashboard />,
			},
			// Agents - Agent configuration
			{
				path: 'agents',
				element: <Agents />,
			},
			// Settings - New settings layout with 240px sidebar
			{
				path: 'settings',
				element: <SettingsPage />,
				children: [
					// Default redirect to commands
					{
						index: true,
						element: <Navigate to="/settings/commands" replace />,
					},
					// CLAUDE CODE section
					{
						path: 'commands',
						element: <SettingsView />,
					},
					{
						path: 'claude-md',
						element: (
							<SettingsPlaceholder
								title="CLAUDE.md"
								description="Edit your project's CLAUDE.md instructions file"
								icon="file-text"
							/>
						),
					},
					{
						path: 'mcp',
						element: (
							<SettingsPlaceholder
								title="MCP Servers"
								description="Configure Model Context Protocol servers for extended capabilities"
								icon="mcp"
							/>
						),
					},
					{
						path: 'memory',
						element: (
							<SettingsPlaceholder
								title="Memory"
								description="Manage Claude's persistent memory across conversations"
								icon="database"
							/>
						),
					},
					{
						path: 'permissions',
						element: (
							<SettingsPlaceholder
								title="Permissions"
								description="Configure tool permissions and access controls"
								icon="shield"
							/>
						),
					},
					// ORC section
					{
						path: 'projects',
						element: (
							<SettingsPlaceholder
								title="Projects"
								description="Manage your ORC projects and repositories"
								icon="folder"
							/>
						),
					},
					{
						path: 'billing',
						element: (
							<SettingsPlaceholder
								title="Billing & Usage"
								description="View your usage statistics and billing information"
								icon="dollar"
							/>
						),
					},
					{
						path: 'import-export',
						element: (
							<SettingsPlaceholder
								title="Import / Export"
								description="Import and export tasks, initiatives, and settings"
								icon="export"
							/>
						),
					},
					// ACCOUNT section
					{
						path: 'profile',
						element: (
							<SettingsPlaceholder
								title="Profile"
								description="Manage your account profile and preferences"
								icon="user"
							/>
						),
					},
					{
						path: 'api-keys',
						element: (
							<SettingsPlaceholder
								title="API Keys"
								description="Manage your API keys and authentication tokens"
								icon="settings"
							/>
						),
					},
					// 404 for unknown settings paths
					{
						path: '*',
						element: <NotFoundPage />,
					},
				],
			},
			// Task detail (existing route)
			{
				path: 'tasks/:id',
				element: <TaskDetail />,
			},
			// Legacy routes with redirects
			{
				path: 'dashboard',
				element: <Navigate to="/stats" replace />,
			},
			{
				path: 'automation',
				element: <AutomationPage />,
			},
			{
				path: 'branches',
				element: <Branches />,
			},
			{
				path: 'preferences',
				element: <Preferences />,
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
				element: <NotFoundPage />,
			},
		],
	},
];
