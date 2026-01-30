/**
 * Timeline Route Tests
 *
 * Tests for the /timeline route configuration.
 *
 * Success Criteria covered:
 * - SC-1: Timeline page renders at /timeline route
 *
 * TDD Note: These tests are written BEFORE the implementation exists.
 * The /timeline route does not yet exist in routes.tsx.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, useRoutes } from 'react-router-dom';
import { routes } from './routes';
import { EventProvider } from '@/hooks';
import { TooltipProvider } from '@/components/ui/Tooltip';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useProjectStore, useInitiativeStore, useUIStore, useTaskStore } from '@/stores';

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

// Mock API for timeline page
vi.mock('@/lib/api', () => ({
	getEvents: vi.fn().mockResolvedValue({
		events: [],
		total: 0,
		limit: 50,
		offset: 0,
		has_more: false,
	}),
	listProjectTasks: vi.fn().mockResolvedValue([]),
	listInitiatives: vi.fn().mockResolvedValue([]),
	getDashboardStats: vi.fn().mockResolvedValue({
		running: 0,
		paused: 0,
		blocked: 0,
		completed: 0,
		failed: 0,
		today: 0,
		total: 0,
		tokens: 0,
		cost: 0,
	}),
	getConfigStats: vi.fn().mockResolvedValue({
		slashCommandsCount: 0,
		claudeMdSize: 0,
		mcpServersCount: 0,
		permissionsProfile: 'default',
	}),
}));

// Test wrapper
function TestApp() {
	const routeElements = useRoutes(routes);
	return (
		<TooltipProvider delayDuration={0}>
			<EventProvider>{routeElements}</EventProvider>
		</TooltipProvider>
	);
}

function renderWithRouter(initialPath: string = '/') {
	const queryClient = new QueryClient({
		defaultOptions: {
			queries: {
				retry: false,
			},
		},
	});

	return render(
		<QueryClientProvider client={queryClient}>
			<MemoryRouter initialEntries={[initialPath]}>
				<TestApp />
			</MemoryRouter>
		</QueryClientProvider>
	);
}

describe('Timeline Route', () => {
	beforeEach(() => {
		// Reset stores
		// RootLayout requires currentProjectId to render app content;
		// without it, ProjectPickerPage is shown instead.
		useProjectStore.setState({
			projects: [{ id: 'test-project', path: '/test', name: 'Test' } as never],
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

	describe('/timeline route (SC-1)', () => {
		it('renders TimelinePage at /timeline route', async () => {
			renderWithRouter('/timeline');

			// Should render the timeline page
			await waitFor(() => {
				const timelinePage = document.querySelector('.timeline-page');
				expect(timelinePage).toBeInTheDocument();
			});
		});

		it('shows Timeline heading', async () => {
			renderWithRouter('/timeline');

			await waitFor(() => {
				expect(screen.getByRole('heading', { name: /timeline/i })).toBeInTheDocument();
			});
		});

		it('does not show 404 for /timeline route', async () => {
			renderWithRouter('/timeline');

			// Wait for page to load
			await waitFor(() => {
				// Should NOT see 404 page content
				expect(screen.queryByText('Page not found')).not.toBeInTheDocument();
				expect(screen.queryByText('404')).not.toBeInTheDocument();
			});
		});

		it('is accessible from navigation sidebar', async () => {
			renderWithRouter('/timeline');

			await waitFor(() => {
				// Sidebar should have timeline link
				const nav = screen.getByRole('navigation', { name: 'Main navigation' });
				expect(nav.querySelector('a[href="/timeline"]')).toBeInTheDocument();
			});
		});
	});

	describe('timeline URL parameters', () => {
		it('accepts types filter parameter', async () => {
			renderWithRouter('/timeline?types=phase_completed,error_occurred');

			await waitFor(() => {
				// Should not throw or show error
				expect(screen.queryByText('Page not found')).not.toBeInTheDocument();
			});
		});

		it('accepts since/until date parameters', async () => {
			const since = new Date(Date.now() - 86400000).toISOString();
			const until = new Date().toISOString();

			renderWithRouter(`/timeline?since=${since}&until=${until}`);

			await waitFor(() => {
				expect(screen.queryByText('Page not found')).not.toBeInTheDocument();
			});
		});

		it('accepts task_id filter parameter', async () => {
			renderWithRouter('/timeline?task_id=TASK-001');

			await waitFor(() => {
				expect(screen.queryByText('Page not found')).not.toBeInTheDocument();
			});
		});

		it('accepts initiative_id filter parameter', async () => {
			renderWithRouter('/timeline?initiative_id=INIT-001');

			await waitFor(() => {
				expect(screen.queryByText('Page not found')).not.toBeInTheDocument();
			});
		});
	});
});
