/**
 * Tests for TASK-722: Remove Agents and Environ from main navigation
 *
 * These tests verify that:
 * - SC-1: Navigation bar shows exactly 7 items (no Agents/Environ)
 * - SC-2: Agents and Environ links are not present
 */
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { IconNav } from './IconNav';
import { TooltipProvider } from '@/components/ui';

function renderWithProviders(ui: React.ReactElement, { route = '/' } = {}) {
	return render(
		<MemoryRouter initialEntries={[route]}>
			<TooltipProvider delayDuration={0}>{ui}</TooltipProvider>
		</MemoryRouter>
	);
}

describe('IconNav - Navigation Items (TASK-722)', () => {
	it('SC-1: should render exactly 8 nav items (My Work, Board, Initiatives, Timeline, Stats, Workflows, Settings, Help)', () => {
		renderWithProviders(<IconNav />);

		const links = screen.getAllByRole('link');
		// After removing Agents and Environ and adding My Work, should be 8 items
		expect(links.length).toBe(8);
	});

	it('SC-2a: should NOT render Agents nav item', () => {
		renderWithProviders(<IconNav />);

		// Agents link should not exist
		expect(screen.queryByText('Agents')).not.toBeInTheDocument();
		expect(screen.queryByRole('link', { name: 'Agent management' })).not.toBeInTheDocument();
	});

	it('SC-2b: should NOT render Environ nav item', () => {
		renderWithProviders(<IconNav />);

		// Environ link should not exist
		expect(screen.queryByText('Environ')).not.toBeInTheDocument();
		expect(screen.queryByRole('link', { name: /Environment/i })).not.toBeInTheDocument();
	});

	it('should still render all expected nav items', () => {
		renderWithProviders(<IconNav />);

		// These should still exist
		expect(screen.getByText('Board')).toBeInTheDocument();
		expect(screen.getByText('Initiatives')).toBeInTheDocument();
		expect(screen.getByText('Timeline')).toBeInTheDocument();
		expect(screen.getByText('Stats')).toBeInTheDocument();
		expect(screen.getByText('Workflows')).toBeInTheDocument();
		expect(screen.getByText('Settings')).toBeInTheDocument();
		expect(screen.getByText('Help')).toBeInTheDocument();
	});
});
