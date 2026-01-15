import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Breadcrumbs } from './Breadcrumbs';

function renderBreadcrumbs(path: string) {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<Breadcrumbs />
		</MemoryRouter>
	);
}

describe('Breadcrumbs', () => {
	it('renders nothing for root path', () => {
		const { container } = renderBreadcrumbs('/');
		expect(container.querySelector('.breadcrumbs')).not.toBeInTheDocument();
	});

	it('renders nothing for dashboard path', () => {
		const { container } = renderBreadcrumbs('/dashboard');
		expect(container.querySelector('.breadcrumbs')).not.toBeInTheDocument();
	});

	it('renders nothing for board path', () => {
		const { container } = renderBreadcrumbs('/board');
		expect(container.querySelector('.breadcrumbs')).not.toBeInTheDocument();
	});

	it('renders breadcrumbs for environment path', () => {
		renderBreadcrumbs('/environment');
		expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toBeInTheDocument();
		expect(screen.getByText('Environment')).toBeInTheDocument();
	});

	it('renders breadcrumbs for preferences path', () => {
		renderBreadcrumbs('/preferences');
		expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toBeInTheDocument();
		expect(screen.getByText('Preferences')).toBeInTheDocument();
	});

	it('renders nested environment paths', () => {
		renderBreadcrumbs('/environment/claude/settings');
		expect(screen.getByText('Environment')).toBeInTheDocument();
		expect(screen.getByText('Claude Code')).toBeInTheDocument();
		expect(screen.getByText('Settings')).toBeInTheDocument();
	});

	it('renders environment/hooks path correctly', () => {
		renderBreadcrumbs('/environment/hooks');
		expect(screen.getByText('Environment')).toBeInTheDocument();
		expect(screen.getByText('Hooks')).toBeInTheDocument();
	});

	it('renders environment/skills path correctly', () => {
		renderBreadcrumbs('/environment/skills');
		expect(screen.getByText('Environment')).toBeInTheDocument();
		expect(screen.getByText('Skills')).toBeInTheDocument();
	});

	it('makes parent segments clickable links', () => {
		renderBreadcrumbs('/environment/claude/settings');
		// Environment should be a link
		const environmentLink = screen.getByRole('link', { name: 'Environment' });
		expect(environmentLink).toHaveAttribute('href', '/environment');
	});

	it('current segment is not a link', () => {
		renderBreadcrumbs('/environment/hooks');
		// Hooks should be current (not a link)
		const hooksText = screen.getByText('Hooks');
		expect(hooksText).toHaveClass('current');
		expect(hooksText.tagName).toBe('SPAN');
	});

	it('category segments link to /environment', () => {
		renderBreadcrumbs('/environment/claude/settings');
		// 'claude' is a category segment and should link to /environment
		const claudeLink = screen.getByRole('link', { name: 'Claude Code' });
		expect(claudeLink).toHaveAttribute('href', '/environment');
	});

	it('renders chevron separators between items', () => {
		const { container } = renderBreadcrumbs('/environment/skills');
		// Should have chevron icons between items
		const svgs = container.querySelectorAll('svg');
		// 2 items means 1 separator
		expect(svgs.length).toBe(1);
	});

	it('handles unknown path segments gracefully', () => {
		renderBreadcrumbs('/environment/unknown-path');
		expect(screen.getByText('Environment')).toBeInTheDocument();
		// Unknown path should just use the segment name
		expect(screen.getByText('unknown-path')).toBeInTheDocument();
	});
});
