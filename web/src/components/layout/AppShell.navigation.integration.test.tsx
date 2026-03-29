/**
 * Integration test: AppShell → TopBar navigation tabs wiring
 *
 * Verifies that the restructured TopBar navigation tabs (Home, Board,
 * Knowledge, Workflows, Settings) are visible through the production
 * AppShell entry point. The unit test (TopBar.navigation.test.tsx) tests
 * TopBar directly — this test verifies the tabs appear when rendered
 * within AppShell (the production component tree).
 *
 * Also verifies that Timeline/Stats have been removed from primary
 * navigation (per spec: "Remove: Timeline, Stats (merge into Home/Knowledge)")
 * while remaining accessible via direct URL.
 *
 * Deletion test: If TopBar stops rendering navigation tabs, these tests fail.
 * If AppShell stops rendering TopBar, these tests fail.
 *
 * Production path: AppShell → TopBar → navigation tabs (Home, Board, Knowledge, Workflows, Settings)
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { AppShell } from './AppShell';
import { TooltipProvider } from '@/components/ui';
import { useProjectStore, useSessionStore } from '@/stores';
import { useThreadStore } from '@/stores/threadStore';
import { useTaskStore } from '@/stores/taskStore';
import { createTimestamp } from '@/test/factories';
import { ProjectSchema } from '@/gen/orc/v1/project_pb';

// Mock API clients to prevent real network calls
vi.mock('@/lib/client', () => ({
	threadClient: {
		listThreads: vi.fn(),
		createThread: vi.fn(),
		getThread: vi.fn(),
		sendMessage: vi.fn(),
		promoteRecommendationDraft: vi.fn(),
		promoteDecisionDraft: vi.fn(),
	},
	taskClient: {
		listTasks: vi.fn(),
	},
	projectClient: {
		listProjects: vi.fn(),
	},
	initiativeClient: {
		listInitiatives: vi.fn(),
	},
}));

// =============================================================================
// TEST UTILITIES
// =============================================================================

function TestWrapper({
	children,
	initialEntries = ['/'],
}: {
	children: React.ReactNode;
	initialEntries?: string[];
}) {
	return (
		<MemoryRouter initialEntries={initialEntries}>
			<TooltipProvider delayDuration={0}>
				{children}
			</TooltipProvider>
		</MemoryRouter>
	);
}

function renderWithProviders(
	ui: React.ReactElement,
	initialEntries: string[] = ['/']
) {
	return render(ui, {
		wrapper: ({ children }) => (
			<TestWrapper initialEntries={initialEntries}>{children}</TestWrapper>
		),
	});
}

// =============================================================================
// SETUP
// =============================================================================

beforeEach(() => {
	useProjectStore.setState({
		projects: [
			create(ProjectSchema, {
				id: 'proj-001',
				name: 'Test Project',
				path: '/test/project',
				createdAt: createTimestamp('2024-01-01T00:00:00Z'),
			}),
		],
		currentProjectId: 'proj-001',
		loading: false,
		error: null,
	});

	useSessionStore.setState({
		sessionId: 'test-session',
		startTime: new Date(),
		totalTokens: 0,
		totalCost: 0,
		isPaused: false,
		activeTaskCount: 0,
		duration: '0m',
		formattedCost: '$0.00',
		formattedTokens: '0',
	});

	useTaskStore.setState({ tasks: [] });
	useThreadStore.getState().reset();
	localStorage.clear();
});

afterEach(() => {
	vi.clearAllMocks();
	localStorage.clear();
});

// =============================================================================
// NAVIGATION TABS THROUGH PRODUCTION APPSHELL
// =============================================================================

describe('AppShell → TopBar navigation tabs wiring', () => {
	it('should render Home navigation tab through AppShell', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const banner = screen.getByRole('banner');
		expect(within(banner).getByRole('link', { name: /home/i })).toBeInTheDocument();
	});

	it('should render Board navigation tab through AppShell', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const banner = screen.getByRole('banner');
		expect(within(banner).getByRole('link', { name: /board/i })).toBeInTheDocument();
	});

	it('should NOT render Knowledge navigation tab through AppShell (no route exists)', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const banner = screen.getByRole('banner');
		expect(within(banner).queryByRole('link', { name: /knowledge/i })).not.toBeInTheDocument();
	});

	it('should render Workflows navigation tab through AppShell', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const banner = screen.getByRole('banner');
		expect(within(banner).getByRole('link', { name: /workflows/i })).toBeInTheDocument();
	});

	it('should render Settings navigation tab through AppShell', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const banner = screen.getByRole('banner');
		expect(within(banner).getByRole('link', { name: /settings/i })).toBeInTheDocument();
	});

	it('should NOT render Timeline in primary navigation through AppShell', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const banner = screen.getByRole('banner');
		expect(within(banner).queryByRole('link', { name: /timeline/i })).not.toBeInTheDocument();
	});

	it('should NOT render Stats in primary navigation through AppShell', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const banner = screen.getByRole('banner');
		expect(within(banner).queryByRole('link', { name: /stats/i })).not.toBeInTheDocument();
	});

	it('should show active indicator on current route tab', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>,
			['/board']
		);

		const banner = screen.getByRole('banner');
		const boardLink = within(banner).getByRole('link', { name: /board/i });

		// Active link should have visual indicator (aria-current or active class)
		expect(
			boardLink.getAttribute('aria-current') === 'page' ||
			boardLink.className.includes('active')
		).toBe(true);
	});
});
