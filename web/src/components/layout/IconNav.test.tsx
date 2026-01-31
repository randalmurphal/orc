import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { IconNav } from './IconNav';
import { TooltipProvider } from '@/components/ui';

/**
 * Test wrapper providing required context (Router + TooltipProvider)
 */
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

describe('IconNav', () => {
	describe('rendering', () => {
		it('should render all nav items with correct icons', () => {
			renderWithProviders(<IconNav />);

			// Check all nav item labels are rendered
			expect(screen.getByText('Board')).toBeInTheDocument();
			expect(screen.getByText('Initiatives')).toBeInTheDocument();
			expect(screen.getByText('Stats')).toBeInTheDocument();
			expect(screen.getByText('Agents')).toBeInTheDocument();
			expect(screen.getByText('Settings')).toBeInTheDocument();
			expect(screen.getByText('Help')).toBeInTheDocument();
		});

		it('should render the logo mark with "O"', () => {
			renderWithProviders(<IconNav />);

			const logoMark = screen.getByText('O');
			expect(logoMark).toBeInTheDocument();
			expect(logoMark).toHaveClass('icon-nav__logo-mark');
		});

		it('should render divider between main and secondary nav', () => {
			const { container } = renderWithProviders(<IconNav />);

			const divider = container.querySelector('.icon-nav__divider');
			expect(divider).toBeInTheDocument();
		});
	});

	describe('active state', () => {
		it('should apply active state when route matches', () => {
			renderWithProviders(<IconNav />, { route: '/board' });

			const boardLink = screen.getByRole('link', { name: 'Task board view' });
			expect(boardLink).toHaveClass('icon-nav__item--active');
		});

		it('should apply active state for nested routes - settings/general', () => {
			renderWithProviders(<IconNav />, { route: '/settings/general' });

			const settingsLink = screen.getByRole('link', { name: 'Application settings' });
			expect(settingsLink).toHaveClass('icon-nav__item--active');
		});

		it('should apply active state for nested routes - settings/display', () => {
			renderWithProviders(<IconNav />, { route: '/settings/display' });

			const settingsLink = screen.getByRole('link', { name: 'Application settings' });
			expect(settingsLink).toHaveClass('icon-nav__item--active');
		});

		it('should not apply active state when route does not match', () => {
			renderWithProviders(<IconNav />, { route: '/other' });

			const boardLink = screen.getByRole('link', { name: 'Task board view' });
			expect(boardLink).not.toHaveClass('icon-nav__item--active');
		});
	});

	describe('accessibility', () => {
		it('should have role="navigation" on nav element', () => {
			renderWithProviders(<IconNav />);

			const nav = screen.getByRole('navigation', { name: 'Main navigation' });
			expect(nav).toBeInTheDocument();
		});

		it('should have aria-label="Main navigation" on nav element', () => {
			renderWithProviders(<IconNav />);

			const nav = screen.getByRole('navigation');
			expect(nav).toHaveAttribute('aria-label', 'Main navigation');
		});

		it('should have aria-label with full description on each nav item', () => {
			renderWithProviders(<IconNav />);

			// Check aria-labels for all nav items
			expect(screen.getByRole('link', { name: 'Task board view' })).toBeInTheDocument();
			expect(screen.getByRole('link', { name: 'View and manage initiatives' })).toBeInTheDocument();
			expect(screen.getByRole('link', { name: 'Statistics and metrics' })).toBeInTheDocument();
			expect(screen.getByRole('link', { name: 'Agent management' })).toBeInTheDocument();
			expect(screen.getByRole('link', { name: 'Application settings' })).toBeInTheDocument();
			expect(screen.getByRole('link', { name: 'Help and documentation' })).toBeInTheDocument();
		});

		it('should have aria-current="page" on active NavLink', () => {
			renderWithProviders(<IconNav />, { route: '/board' });

			const boardLink = screen.getByRole('link', { name: 'Task board view' });
			expect(boardLink).toHaveAttribute('aria-current', 'page');
		});

		it('should be keyboard navigable with Tab', () => {
			renderWithProviders(<IconNav />);

			const links = screen.getAllByRole('link');
			expect(links.length).toBe(9); // 9 nav items (Board, Initiatives, Timeline, Stats, Workflows, Agents, Environ, Settings, Help)

			// All links should be focusable (tabindex not -1)
			links.forEach((link) => {
				expect(link).not.toHaveAttribute('tabindex', '-1');
			});
		});
	});

	describe('tooltips', () => {
		/**
		 * Note: Complex hover/focus interactions are better tested via E2E tests
		 * as jsdom has limitations with pointer events. See Tooltip.test.tsx.
		 *
		 * We verify tooltips are configured by checking that:
		 * 1. The Tooltip wrapper exists (via data-state attribute from Radix)
		 * 2. The aria-label provides the description
		 */
		it('should have tooltip wrapper for Board nav item', () => {
			renderWithProviders(<IconNav />);

			const boardLink = screen.getByRole('link', { name: 'Task board view' });
			// Radix Tooltip adds data-state attribute to trigger
			expect(boardLink).toHaveAttribute('data-state');
		});

		it('should have tooltip wrapper for Settings nav item', () => {
			renderWithProviders(<IconNav />);

			const settingsLink = screen.getByRole('link', { name: 'Application settings' });
			expect(settingsLink).toHaveAttribute('data-state');
		});

		it('should have tooltip wrapper for Help nav item', () => {
			renderWithProviders(<IconNav />);

			const helpLink = screen.getByRole('link', { name: 'Help and documentation' });
			expect(helpLink).toHaveAttribute('data-state');
		});

		it('should have tooltip wrapper for all nav items', () => {
			renderWithProviders(<IconNav />);

			const links = screen.getAllByRole('link');
			// All 7 nav items should have data-state from Tooltip wrapper
			links.forEach((link) => {
				expect(link).toHaveAttribute('data-state');
			});
		});
	});

	describe('custom className', () => {
		it('should apply custom className when provided', () => {
			const { container } = renderWithProviders(<IconNav className="custom-class" />);

			const nav = container.querySelector('.icon-nav');
			expect(nav).toHaveClass('custom-class');
		});
	});
});
