import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { TopBar } from './TopBar';
import { useProjectStore, useSessionStore } from '@/stores';
import { createTimestamp } from '@/test/factories';
import { ProjectSchema } from '@/gen/orc/v1/project_pb';

/**
 * SC-5: Top navigation shows Home, Project, Board, Inbox, Workflows, Settings tabs
 * replacing the icon-based navigation
 *
 * These tests verify the navigation restructure in TopBar.
 * Timeline, Stats, and Knowledge links are removed from primary navigation;
 * pages still accessible via direct URL (Knowledge has no route yet).
 */

function renderWithRouter(ui: React.ReactElement, initialEntries: string[] = ['/']) {
	return render(ui, {
		wrapper: ({ children }) => (
			<MemoryRouter initialEntries={initialEntries}>{children}</MemoryRouter>
		),
	});
}

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
});

// =============================================================================
// SC-5: Navigation tabs
// =============================================================================

describe('TopBar navigation tabs (SC-5)', () => {
	it('should render Home navigation tab', () => {
		renderWithRouter(<TopBar />);

		const homeLink = screen.getByRole('link', { name: /home/i });
		expect(homeLink).toBeInTheDocument();
		expect(homeLink).toHaveAttribute('href', '/');
	});

	it('should render Project navigation tab', () => {
		renderWithRouter(<TopBar />);

		const projectLink = screen.getByRole('link', { name: /project/i });
		expect(projectLink).toBeInTheDocument();
		expect(projectLink).toHaveAttribute('href', '/project');
	});

	it('should render Board navigation tab', () => {
		renderWithRouter(<TopBar />);

		const boardLink = screen.getByRole('link', { name: /board/i });
		expect(boardLink).toBeInTheDocument();
		expect(boardLink).toHaveAttribute('href', '/board');
	});

	it('should NOT render Knowledge in primary navigation (no route exists)', () => {
		renderWithRouter(<TopBar />);

		const knowledgeLink = screen.queryByRole('link', { name: /knowledge/i });
		expect(knowledgeLink).not.toBeInTheDocument();
	});

	it('should render Workflows navigation tab', () => {
		renderWithRouter(<TopBar />);

		const workflowsLink = screen.getByRole('link', { name: /workflows/i });
		expect(workflowsLink).toBeInTheDocument();
		expect(workflowsLink).toHaveAttribute('href', '/workflows');
	});

	it('should render Settings navigation tab', () => {
		renderWithRouter(<TopBar />);

		const settingsLink = screen.getByRole('link', { name: /settings/i });
		expect(settingsLink).toBeInTheDocument();
		expect(settingsLink).toHaveAttribute('href', '/settings');
	});

	it('should have exactly 6 navigation tabs', () => {
		renderWithRouter(<TopBar />);

		// Get all navigation links within the nav tab area
		const navTabs = screen.getAllByRole('link').filter(link => {
			const href = link.getAttribute('href');
			return href === '/' ||
				href === '/project' ||
				href === '/board' ||
				href === '/recommendations' ||
				href === '/workflows' ||
				href === '/settings';
		});
		expect(navTabs).toHaveLength(6);
	});

	it('should NOT render Timeline in primary navigation', () => {
		renderWithRouter(<TopBar />);

		// Timeline should not appear as a nav tab
		const timelineLink = screen.queryByRole('link', { name: /timeline/i });
		expect(timelineLink).not.toBeInTheDocument();
	});

	it('should NOT render Stats in primary navigation', () => {
		renderWithRouter(<TopBar />);

		// Stats should not appear as a nav tab
		const statsLink = screen.queryByRole('link', { name: /stats/i });
		expect(statsLink).not.toBeInTheDocument();
	});

	it('should show active indicator on current route', () => {
		renderWithRouter(<TopBar />, ['/project']);

		const projectLink = screen.getByRole('link', { name: /project/i });
		expect(projectLink.className).toMatch(/active/);
	});

	it('should show active indicator on Home for root route', () => {
		renderWithRouter(<TopBar />, ['/']);

		const homeLink = screen.getByRole('link', { name: /home/i });
		expect(homeLink.className).toMatch(/active/);
	});
});
