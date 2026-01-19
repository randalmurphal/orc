import { RouteObject, Navigate, Outlet } from 'react-router-dom';
import { AppShell } from '@/components/layout/AppShell';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { Board } from '@/pages/Board';
import { Dashboard } from '@/pages/Dashboard';
import { TaskList } from '@/pages/TaskList';
import { TaskDetail } from '@/pages/TaskDetail';
import { InitiativeDetail } from '@/pages/InitiativeDetail';
import { AutomationPage } from '@/pages/AutomationPage';
import { Branches } from '@/pages/Branches';
import { Preferences } from '@/pages/Preferences';
import { NotFoundPage } from '@/pages/NotFoundPage';
import { EnvironmentLayout } from '@/pages/environment/EnvironmentLayout';
import { Settings } from '@/pages/environment/Settings';
import { Prompts } from '@/pages/environment/Prompts';
import { Scripts } from '@/pages/environment/Scripts';
import { Hooks } from '@/pages/environment/Hooks';
import { Skills } from '@/pages/environment/Skills';
import { Mcp } from '@/pages/environment/Mcp';
import { Config } from '@/pages/environment/Config';
import { ClaudeMd } from '@/pages/environment/ClaudeMd';
import { Tools } from '@/pages/environment/Tools';
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
 * | /initiatives | (task list filtered view) |
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
			// Initiatives - Task list with initiative focus
			{
				path: 'initiatives',
				element: <TaskList />,
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
			// Settings - Environment settings with nested routes
			{
				path: 'settings',
				element: <EnvironmentLayout />,
				children: [
					{
						index: true,
						element: <Navigate to="/settings/prompts/system" replace />,
					},
					// Prompts section
					{
						path: 'prompts',
						element: <Navigate to="/settings/prompts/system" replace />,
					},
					{
						path: 'prompts/:section',
						element: <Prompts />,
					},
					// Configuration section
					{
						path: 'configuration',
						element: <Navigate to="/settings/configuration/general" replace />,
					},
					{
						path: 'configuration/:section',
						element: <Settings />,
					},
					// Automation section
					{
						path: 'automation',
						element: <Navigate to="/settings/automation/hooks" replace />,
					},
					{
						path: 'automation/hooks',
						element: <Hooks />,
					},
					{
						path: 'automation/scripts',
						element: <Scripts />,
					},
					{
						path: 'automation/:section',
						element: <Hooks />,
					},
					// Advanced section
					{
						path: 'advanced',
						element: <Navigate to="/settings/advanced/mcp" replace />,
					},
					{
						path: 'advanced/mcp',
						element: <Mcp />,
					},
					{
						path: 'advanced/tools',
						element: <Tools />,
					},
					{
						path: 'advanced/skills',
						element: <Skills />,
					},
					{
						path: 'advanced/config',
						element: <Config />,
					},
					{
						path: 'advanced/claudemd',
						element: <ClaudeMd />,
					},
					{
						path: 'advanced/:section',
						element: <Mcp />,
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
