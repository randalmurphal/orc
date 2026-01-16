import { RouteObject, Navigate } from 'react-router-dom';
import { AppLayout } from '@/components/layout/AppLayout';
import { TaskList } from '@/pages/TaskList';
import { Board } from '@/pages/Board';
import { Dashboard } from '@/pages/Dashboard';
import { TaskDetail } from '@/pages/TaskDetail';
import { InitiativeDetail } from '@/pages/InitiativeDetail';
import { Branches } from '@/pages/Branches';
import { Preferences } from '@/pages/Preferences';
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
 * URL Parameters by Route
 *
 * | Route | Params |
 * |-------|--------|
 * | / | ?project, ?initiative, ?dependency_status |
 * | /board | ?project, ?initiative, ?dependency_status |
 * | /dashboard | ?project |
 * | /tasks/:id | ?tab |
 * | /initiatives/:id | - |
 * | /preferences | - |
 * | /environment/* | - |
 */

export const routes: RouteObject[] = [
	{
		path: '/',
		element: <AppLayout />,
		children: [
			{
				index: true,
				element: <TaskList />,
			},
			{
				path: 'board',
				element: <Board />,
			},
			{
				path: 'dashboard',
				element: <Dashboard />,
			},
			{
				path: 'tasks/:id',
				element: <TaskDetail />,
			},
			{
				path: 'initiatives/:id',
				element: <InitiativeDetail />,
			},
			{
				path: 'branches',
				element: <Branches />,
			},
			{
				path: 'preferences',
				element: <Preferences />,
			},
			{
				path: 'environment',
				element: <EnvironmentLayout />,
				children: [
					{
						index: true,
						element: <Navigate to="settings" replace />,
					},
					{
						path: 'settings',
						element: <Settings />,
					},
					{
						path: 'prompts',
						element: <Prompts />,
					},
					{
						path: 'scripts',
						element: <Scripts />,
					},
					{
						path: 'hooks',
						element: <Hooks />,
					},
					{
						path: 'skills',
						element: <Skills />,
					},
					{
						path: 'mcp',
						element: <Mcp />,
					},
					{
						path: 'config',
						element: <Config />,
					},
					{
						path: 'claudemd',
						element: <ClaudeMd />,
					},
					{
						path: 'tools',
						element: <Tools />,
					},
					{
						path: 'agents',
						element: <Agents />,
					},
				],
			},
		],
	},
];
