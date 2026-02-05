/**
 * Integration Test for IconNav - "My Work" navigation entry
 *
 * Success Criteria Coverage:
 * - SC-7: IconNav includes "My Work" as the first item in main navigation
 *
 * INTEGRATION TEST:
 * This tests that IconNav (an existing component) has been modified to include
 * the new "My Work" nav item. If the implementation doesn't add the nav item
 * to mainNavItems, this test fails.
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { IconNav } from './IconNav';
import { TooltipProvider } from '@/components/ui';

function renderWithProviders(
	ui: React.ReactElement,
	{ route = '/' } = {}
) {
	return render(
		<MemoryRouter initialEntries={[route]}>
			<TooltipProvider delayDuration={0}>
				{ui}
			</TooltipProvider>
		</MemoryRouter>
	);
}

describe('IconNav - My Work integration', () => {
	it('should include "My Work" nav item', () => {
		renderWithProviders(<IconNav />);

		expect(screen.getByText('My Work')).toBeInTheDocument();
	});

	it('should render "My Work" as the first main nav item (before Board)', () => {
		renderWithProviders(<IconNav />);

		const links = screen.getAllByRole('link');
		const myWorkIndex = links.findIndex(
			(link) => link.textContent?.includes('My Work')
		);
		const boardIndex = links.findIndex(
			(link) => link.textContent?.includes('Board')
		);

		expect(myWorkIndex).toBeGreaterThanOrEqual(0);
		expect(boardIndex).toBeGreaterThanOrEqual(0);
		expect(myWorkIndex).toBeLessThan(boardIndex);
	});

	it('should link "My Work" to /', () => {
		renderWithProviders(<IconNav />);

		const myWorkLink = screen.getByText('My Work').closest('a');
		expect(myWorkLink).toHaveAttribute('href', '/');
	});

	it('should show active indicator when pathname is /', () => {
		renderWithProviders(<IconNav />, { route: '/' });

		const myWorkLink = screen.getByText('My Work').closest('a');
		expect(myWorkLink).toHaveClass('icon-nav__item--active');
	});

	it('should NOT show active indicator when on /board', () => {
		renderWithProviders(<IconNav />, { route: '/board' });

		const myWorkLink = screen.getByText('My Work').closest('a');
		expect(myWorkLink).not.toHaveClass('icon-nav__item--active');
	});

	it('should have correct total nav items count (8: My Work + original 7)', () => {
		renderWithProviders(<IconNav />);

		const links = screen.getAllByRole('link');
		expect(links.length).toBe(8);
	});
});
