/**
 * Tests for TASK-722: Remove /agents and /environment routes
 *
 * These tests verify that:
 * - SC-2: /agents route returns 404
 * - SC-2: /environment route returns 404
 */
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, useRoutes } from 'react-router-dom';
import { routes } from './routes';
import { EventProvider } from '@/hooks';
import { TooltipProvider } from '@/components/ui/Tooltip';
import { useProjectStore, useInitiativeStore, useUIStore, useTaskStore } from '@/stores';
import { createTimestamp } from '@/test/factories';
import type { Project } from '@/gen/orc/v1/project_pb';

// Mock events module
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

// Mock fetch
vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
	ok: true,
	json: () => Promise.resolve({}),
}));

// Mock Connect RPC client
vi.mock('@/lib/client', () => ({
	taskClient: {
		getTask: vi.fn().mockResolvedValue({ task: { id: 'TASK-001', title: 'Test', status: 1, weight: 2 } }),
		getTaskPlan: vi.fn().mockResolvedValue({ plan: { phases: [] } }),
		listTasks: vi.fn().mockResolvedValue({ tasks: [] }),
	},
	initiativeClient: {
		getInitiative: vi.fn().mockResolvedValue({ initiative: { id: 'INIT-001', title: 'Test', status: 1, tasks: [], decisions: [] } }),
		listInitiatives: vi.fn().mockResolvedValue({ initiatives: [] }),
	},
	configClient: {
		getConfigStats: vi.fn().mockResolvedValue({ stats: { slashCommandsCount: 0, claudeMdSize: BigInt(0), mcpServersCount: 0, permissionsProfile: 'default' } }),
		listAgents: vi.fn().mockResolvedValue({ agents: [] }),
		getConfig: vi.fn().mockResolvedValue({ config: { executionSettings: {}, toolPermissions: {} } }),
		listSkills: vi.fn().mockResolvedValue({ skills: [] }),
		getClaudeMd: vi.fn().mockResolvedValue({ files: [] }),
	},
	decisionClient: { resolveDecision: vi.fn().mockResolvedValue({}) },
	knowledgeClient: { listKnowledge: vi.fn().mockResolvedValue({ entries: [] }), getKnowledgeStatus: vi.fn().mockResolvedValue({ status: null }) },
	mcpClient: { listMCPServers: vi.fn().mockResolvedValue({ servers: [] }) },
}));

// Mock API
vi.mock('@/lib/api', () => ({
	listProjectTasks: vi.fn().mockResolvedValue([]),
	listInitiatives: vi.fn().mockResolvedValue([]),
	getDashboardStats: vi.fn().mockResolvedValue({ running: 0, paused: 0, blocked: 0, completed: 0, failed: 0, today: 0, total: 0, tokens: 0, cost: 0 }),
	runProjectTask: vi.fn(),
	pauseProjectTask: vi.fn(),
	resumeProjectTask: vi.fn(),
	escalateProjectTask: vi.fn(),
	updateTask: vi.fn(),
	triggerFinalize: vi.fn(),
	getInitiative: vi.fn().mockResolvedValue({ id: 'INIT-001', title: 'Test', status: 'active', tasks: [], decisions: [] }),
	updateInitiative: vi.fn(),
	addInitiativeTask: vi.fn(),
	removeInitiativeTask: vi.fn(),
	addInitiativeDecision: vi.fn(),
	listTasks: vi.fn().mockResolvedValue([]),
	getInitiativeDependencyGraph: vi.fn().mockResolvedValue({ nodes: [], edges: [] }),
	getTask: vi.fn().mockResolvedValue({ id: 'TASK-001', title: 'Test', status: 'created', weight: 'medium' }),
	getTaskPlan: vi.fn().mockResolvedValue({ phases: [] }),
	getTaskDependencies: vi.fn().mockResolvedValue({ blocked_by: [], blocks: [], related_to: [], referenced_by: [] }),
	getTaskTimeline: vi.fn().mockResolvedValue([]),
	getConfigStats: vi.fn().mockResolvedValue({ slashCommandsCount: 0, claudeMdSize: 0, mcpServersCount: 0, permissionsProfile: 'default' }),
}));

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

describe('Routes - Navigation Removal (TASK-722)', () => {
	beforeEach(() => {
		useProjectStore.setState({
			projects: [{ id: 'test-project', path: '/test/project', name: 'Test Project', createdAt: createTimestamp('2024-01-01T00:00:00Z') } as Project],
			currentProjectId: 'test-project',
			loading: false,
			error: null,
			_isHandlingPopState: false,
		});
		useInitiativeStore.setState({ initiatives: new Map(), currentInitiativeId: null, loading: false, error: null, hasLoaded: false, _isHandlingPopState: false });
		useUIStore.setState({ sidebarExpanded: true, wsStatus: 'disconnected', toasts: [] });
		useTaskStore.setState({ tasks: [], taskStates: new Map(), loading: false, error: null });
	});

	it('SC-2: /agents route should return 404', async () => {
		renderWithRouter('/agents');
		await waitFor(() => {
			expect(screen.getByText('Page not found')).toBeInTheDocument();
			expect(screen.getByText('404')).toBeInTheDocument();
		});
	});

	it('SC-2: /environment route should return 404', async () => {
		renderWithRouter('/environment');
		await waitFor(() => {
			expect(screen.getByText('Page not found')).toBeInTheDocument();
			expect(screen.getByText('404')).toBeInTheDocument();
		});
	});

	it('SC-2: /environment/hooks route should return 404', async () => {
		renderWithRouter('/environment/hooks');
		await waitFor(() => {
			expect(screen.getByText('Page not found')).toBeInTheDocument();
			expect(screen.getByText('404')).toBeInTheDocument();
		});
	});
});
