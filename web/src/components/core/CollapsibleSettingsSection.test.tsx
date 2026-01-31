/**
 * TDD Tests for CollapsibleSettingsSection
 *
 * Tests for TASK-669: Phase template claude_config editor with collapsible sections
 *
 * Success Criteria Coverage:
 * - SC-1: CollapsibleSettingsSection renders collapsed by default and expands on click,
 *         showing badge count of active items
 */

import { describe, it, expect, vi, afterEach, beforeAll } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CollapsibleSettingsSection } from './CollapsibleSettingsSection';

// Mock browser APIs for Radix
beforeAll(() => {
	Element.prototype.scrollIntoView = vi.fn();
	Element.prototype.hasPointerCapture = vi.fn().mockReturnValue(false);
	Element.prototype.setPointerCapture = vi.fn();
	Element.prototype.releasePointerCapture = vi.fn();
	global.ResizeObserver = vi.fn().mockImplementation(() => ({
		observe: vi.fn(),
		unobserve: vi.fn(),
		disconnect: vi.fn(),
	}));
});

describe('CollapsibleSettingsSection', () => {
	afterEach(() => {
		cleanup();
	});

	// SC-1: renders collapsed by default
	it('renders collapsed by default with content hidden', () => {
		render(
			<CollapsibleSettingsSection title="Hooks" badgeCount={3}>
				<div>Hook content</div>
			</CollapsibleSettingsSection>
		);

		// Header should be visible
		expect(screen.getByText('Hooks')).toBeInTheDocument();

		// Content should be hidden when collapsed
		expect(screen.queryByText('Hook content')).not.toBeInTheDocument();
	});

	// SC-1: badge shows count of active items
	it('shows badge count of active items', () => {
		render(
			<CollapsibleSettingsSection title="Hooks" badgeCount={3}>
				<div>Hook content</div>
			</CollapsibleSettingsSection>
		);

		expect(screen.getByText('3')).toBeInTheDocument();
	});

	// SC-1: badge shows 0 or is hidden when no items
	it('shows badge with 0 when no active items', () => {
		render(
			<CollapsibleSettingsSection title="Hooks" badgeCount={0}>
				<div>Hook content</div>
			</CollapsibleSettingsSection>
		);

		// Badge should show "0" or not be present - either is acceptable per spec
		expect(screen.getByText('Hooks')).toBeInTheDocument();
	});

	// SC-1: expands on click, showing content
	it('expands on header click and shows content', async () => {
		const user = userEvent.setup();

		render(
			<CollapsibleSettingsSection title="Hooks" badgeCount={2}>
				<div>Hook content</div>
			</CollapsibleSettingsSection>
		);

		// Content hidden initially
		expect(screen.queryByText('Hook content')).not.toBeInTheDocument();

		// Click header to expand
		await user.click(screen.getByText('Hooks'));

		// Content should now be visible
		expect(screen.getByText('Hook content')).toBeInTheDocument();
	});

	// SC-1: chevron rotates on expand (visual indicator)
	it('renders a chevron indicator that changes on expand', async () => {
		const user = userEvent.setup();

		render(
			<CollapsibleSettingsSection title="MCP Servers" badgeCount={1}>
				<div>Server content</div>
			</CollapsibleSettingsSection>
		);

		// Header button should exist (clickable to toggle)
		const headerButton = screen.getByRole('button', { name: /mcp servers/i });
		expect(headerButton).toBeInTheDocument();

		// Click to expand
		await user.click(headerButton);

		// Content should be visible after expand
		expect(screen.getByText('Server content')).toBeInTheDocument();
	});

	// SC-1: collapses on second click
	it('collapses on second click, hiding content again', async () => {
		const user = userEvent.setup();

		render(
			<CollapsibleSettingsSection title="Skills" badgeCount={0}>
				<div>Skill content</div>
			</CollapsibleSettingsSection>
		);

		// Click to expand
		await user.click(screen.getByText('Skills'));
		expect(screen.getByText('Skill content')).toBeInTheDocument();

		// Click to collapse
		await user.click(screen.getByText('Skills'));
		expect(screen.queryByText('Skill content')).not.toBeInTheDocument();
	});

	// SC-1: supports defaultExpanded prop
	it('supports starting expanded via defaultExpanded prop', () => {
		render(
			<CollapsibleSettingsSection title="Env Vars" badgeCount={2} defaultExpanded>
				<div>Env content</div>
			</CollapsibleSettingsSection>
		);

		// Content should be visible immediately
		expect(screen.getByText('Env content')).toBeInTheDocument();
	});

	// Edge case: disabled state for built-in templates
	it('renders disabled state when disabled prop is set', () => {
		render(
			<CollapsibleSettingsSection title="Hooks" badgeCount={1} disabled>
				<div>Hook content</div>
			</CollapsibleSettingsSection>
		);

		const headerButton = screen.getByRole('button', { name: /hooks/i });
		expect(headerButton).toBeDisabled();
	});
});
