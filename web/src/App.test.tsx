import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from './App';
import { useProjectStore, useTaskStore, useInitiativeStore } from '@/stores';

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

// Mock API calls
vi.mock('@/lib/api', () => ({
	listProjects: vi.fn().mockResolvedValue([
		{ id: 'project-1', name: 'Test Project', path: '/path/to/project', created_at: new Date().toISOString() },
	]),
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
	deleteProjectTask: vi.fn(),
	// Config stats for TopBar
	getConfigStats: vi.fn().mockResolvedValue({
		slashCommandsCount: 0,
		claudeMdSize: 0,
		mcpServersCount: 0,
		permissionsProfile: 'default',
	}),
}));

function renderApp(initialPath: string = '/') {
	return render(
		<MemoryRouter initialEntries={[initialPath]}>
			<App />
		</MemoryRouter>
	);
}

describe('App', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		// Reset stores before each test
		useProjectStore.setState({
			projects: [],
			currentProjectId: null,
			loading: false,
			error: null,
			_isHandlingPopState: false,
		});
		useTaskStore.setState({
			tasks: [],
			taskStates: new Map(),
			loading: false,
			error: null,
		});
		useInitiativeStore.setState({
			initiatives: new Map(),
			currentInitiativeId: null,
			loading: false,
			error: null,
			hasLoaded: false,
			_isHandlingPopState: false,
		});
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	it('renders App with router and EventProvider', async () => {
		renderApp('/');
		// App renders routes wrapped in EventProvider
		await waitFor(() => {
			// Should render the layout with sidebar and content
			expect(screen.getByRole('navigation', { name: 'Main navigation' })).toBeInTheDocument();
		});
	});

	it('redirects root route to /board', async () => {
		renderApp('/');
		await waitFor(() => {
			// Root route redirects to /board which renders BoardView
			const boardView = document.querySelector('.board-view');
			expect(boardView).toBeInTheDocument();
		});
	});

	it('renders Board page at /board route', async () => {
		renderApp('/board');
		await waitFor(() => {
			// Board page renders - check for the board-view container class
			const boardView = document.querySelector('.board-view');
			expect(boardView).toBeInTheDocument();
		});
	});

	it('redirects /dashboard to /stats and shows Statistics', async () => {
		renderApp('/dashboard');
		await waitFor(() => {
			// Stats page renders with "Statistics" title
			expect(screen.getByText('Statistics')).toBeInTheDocument();
		});
	});
});
