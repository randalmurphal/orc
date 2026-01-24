/**
 * Timeline Navigation Tests for IconNav
 * 
 * Tests added as part of TASK-398 to verify Timeline nav item integration.
 * These tests cover:
 * - SC-1: Timeline nav item is visible in IconNav (label + icon + aria-label)
 * - SC-2: Timeline active state when at /timeline route
 * - SC-3: Timeline link href is correct
 */

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

describe('IconNav - Timeline Navigation (TASK-398)', () => {
	describe('rendering', () => {
		it('should render Timeline label in nav items', () => {
			renderWithProviders(<IconNav />);
			
			// Timeline label should be visible
			expect(screen.getByText('Timeline')).toBeInTheDocument();
		});

		it('should render Timeline link with correct aria-label', () => {
			renderWithProviders(<IconNav />);
			
			// Timeline link should have descriptive aria-label
			const timelineLink = screen.getByRole('link', { name: 'Activity timeline' });
			expect(timelineLink).toBeInTheDocument();
		});

		it('should render Timeline link with correct href', () => {
			renderWithProviders(<IconNav />);
			
			const timelineLink = screen.getByRole('link', { name: 'Activity timeline' });
			expect(timelineLink).toHaveAttribute('href', '/timeline');
		});

		it('should render Timeline with activity icon', () => {
			renderWithProviders(<IconNav />);

			// The Timeline link should contain an SVG icon
			const timelineLink = screen.getByRole('link', { name: 'Activity timeline' });
			const icon = timelineLink.querySelector('svg');
			expect(icon).toBeInTheDocument();
		});
	});

	describe('active state', () => {
		it('should apply active state when at /timeline route', () => {
			renderWithProviders(<IconNav />, { route: '/timeline' });
			
			const timelineLink = screen.getByRole('link', { name: 'Activity timeline' });
			expect(timelineLink).toHaveClass('icon-nav__item--active');
		});

		it('should apply active state for nested timeline routes', () => {
			// Test with a nested route like /timeline?types=phase_completed
			renderWithProviders(<IconNav />, { route: '/timeline?types=phase_completed' });
			
			const timelineLink = screen.getByRole('link', { name: 'Activity timeline' });
			expect(timelineLink).toHaveClass('icon-nav__item--active');
		});

		it('should have aria-current="page" when Timeline is active', () => {
			renderWithProviders(<IconNav />, { route: '/timeline' });
			
			const timelineLink = screen.getByRole('link', { name: 'Activity timeline' });
			expect(timelineLink).toHaveAttribute('aria-current', 'page');
		});

		it('should NOT apply active state when at different route', () => {
			renderWithProviders(<IconNav />, { route: '/board' });
			
			const timelineLink = screen.getByRole('link', { name: 'Activity timeline' });
			expect(timelineLink).not.toHaveClass('icon-nav__item--active');
		});
	});

	describe('positioning', () => {
		it('should render Timeline after Initiatives and before Stats', () => {
			renderWithProviders(<IconNav />);

			// Get all nav links
			const links = screen.getAllByRole('link');
			const labelTexts = links.map(link => link.textContent?.replace(/\s+/g, ' ').trim());
			
			// Find positions
			const initiativesIndex = labelTexts.findIndex(text => text?.includes('Initiatives'));
			const timelineIndex = labelTexts.findIndex(text => text?.includes('Timeline'));
			const statsIndex = labelTexts.findIndex(text => text?.includes('Stats'));
			
			// Timeline should be between Initiatives and Stats
			expect(timelineIndex).toBeGreaterThan(initiativesIndex);
			expect(timelineIndex).toBeLessThan(statsIndex);
		});
	});

	describe('tooltip', () => {
		it('should have tooltip wrapper for Timeline nav item', () => {
			renderWithProviders(<IconNav />);
			
			const timelineLink = screen.getByRole('link', { name: 'Activity timeline' });
			// Radix Tooltip adds data-state attribute to trigger
			expect(timelineLink).toHaveAttribute('data-state');
		});
	});
});
