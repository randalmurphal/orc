/**
 * SettingsLayout Badge Tests - TDD for TASK-533
 *
 * Tests that badges show API-fetched counts instead of hard-coded mock values.
 * These tests will FAIL until the implementation replaces mock values with API calls.
 *
 * Success Criteria covered:
 * - SC-1: Slash Commands badge shows actual command count from API
 * - SC-2: MCP Servers badge shows actual server count from API
 * - SC-3: Memory badge is removed until memory API exists
 * - SC-4: Badges show loading skeleton while fetching
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { SettingsLayout } from './SettingsLayout';
import * as api from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
	getConfigStats: vi.fn(),
}));

const mockGetConfigStats = api.getConfigStats as ReturnType<typeof vi.fn>;

/**
 * Test wrapper providing router context
 */
function renderWithRouter(initialRoute: string = '/settings/commands') {
	return render(
		<MemoryRouter initialEntries={[initialRoute]}>
			<Routes>
				<Route path="/settings" element={<SettingsLayout />}>
					<Route path="commands" element={<div data-testid="commands-content">Commands</div>} />
					<Route path="mcp" element={<div data-testid="mcp-content">MCP</div>} />
					<Route path="memory" element={<div data-testid="memory-content">Memory</div>} />
				</Route>
			</Routes>
		</MemoryRouter>
	);
}

/**
 * Helper to find badge by nav item label
 */
function getBadgeForNavItem(label: string): HTMLElement | null {
	const navItem = screen.getByText(label).closest('.settings-nav-item');
	return navItem?.querySelector('.settings-nav-item__badge') ?? null;
}

describe('SettingsLayout Badge API Integration', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('SC-1: Slash Commands badge shows actual count from API', () => {
		it('displays badge matching API-returned slash commands count', async () => {
			mockGetConfigStats.mockResolvedValue({
				slashCommandsCount: 3,
				claudeMdSize: 1024,
				mcpServersCount: 1,
				permissionsProfile: 'default',
			});

			renderWithRouter();

			// Wait for API call to complete and badge to update
			await waitFor(() => {
				const badge = getBadgeForNavItem('Slash Commands');
				expect(badge).toBeInTheDocument();
				expect(badge?.textContent).toBe('3');
			});
		});

		it('shows no badge when API returns 0 slash commands (BDD-2)', async () => {
			mockGetConfigStats.mockResolvedValue({
				slashCommandsCount: 0,
				claudeMdSize: 0,
				mcpServersCount: 0,
				permissionsProfile: 'default',
			});

			renderWithRouter();

			// Wait for API call to complete
			await waitFor(() => {
				expect(mockGetConfigStats).toHaveBeenCalled();
			});

			// Badge should not be present when count is 0
			const badge = getBadgeForNavItem('Slash Commands');
			expect(badge).not.toBeInTheDocument();
		});

		it('handles large counts correctly (1000+)', async () => {
			mockGetConfigStats.mockResolvedValue({
				slashCommandsCount: 1234,
				claudeMdSize: 0,
				mcpServersCount: 0,
				permissionsProfile: 'default',
			});

			renderWithRouter();

			await waitFor(() => {
				const badge = getBadgeForNavItem('Slash Commands');
				expect(badge).toBeInTheDocument();
				expect(badge?.textContent).toBe('1234');
			});
		});
	});

	describe('SC-2: MCP Servers badge shows actual count from API', () => {
		it('displays badge matching API-returned MCP servers count', async () => {
			mockGetConfigStats.mockResolvedValue({
				slashCommandsCount: 0,
				claudeMdSize: 0,
				mcpServersCount: 7,
				permissionsProfile: 'default',
			});

			renderWithRouter();

			await waitFor(() => {
				const badge = getBadgeForNavItem('MCP Servers');
				expect(badge).toBeInTheDocument();
				expect(badge?.textContent).toBe('7');
			});
		});

		it('shows no badge when API returns 0 MCP servers', async () => {
			mockGetConfigStats.mockResolvedValue({
				slashCommandsCount: 5,
				claudeMdSize: 0,
				mcpServersCount: 0,
				permissionsProfile: 'default',
			});

			renderWithRouter();

			await waitFor(() => {
				expect(mockGetConfigStats).toHaveBeenCalled();
			});

			const badge = getBadgeForNavItem('MCP Servers');
			expect(badge).not.toBeInTheDocument();
		});
	});

	describe('SC-3: Memory badge is removed', () => {
		it('does not show Memory badge regardless of API response', async () => {
			mockGetConfigStats.mockResolvedValue({
				slashCommandsCount: 5,
				claudeMdSize: 1024,
				mcpServersCount: 3,
				permissionsProfile: 'default',
			});

			renderWithRouter();

			// Wait for API call to complete
			await waitFor(() => {
				expect(mockGetConfigStats).toHaveBeenCalled();
			});

			// Memory nav item should exist but have no badge
			const memoryNavItem = screen.getByText('Memory').closest('.settings-nav-item');
			expect(memoryNavItem).toBeInTheDocument();

			const badge = memoryNavItem?.querySelector('.settings-nav-item__badge');
			expect(badge).not.toBeInTheDocument();
		});
	});

	describe('SC-4: Loading state for badges', () => {
		it('shows no badges during loading (before API response)', async () => {
			// Create a promise that won't resolve immediately
			let resolvePromise: (value: api.ConfigStats) => void;
			const pendingPromise = new Promise<api.ConfigStats>((resolve) => {
				resolvePromise = resolve;
			});
			mockGetConfigStats.mockReturnValue(pendingPromise);

			const { container } = renderWithRouter();

			// During loading, badges should not show hard-coded values
			const badges = container.querySelectorAll('.settings-nav-item__badge');

			// Either no badges, or badges should not show hard-coded "5", "2", "12"
			const badgeTexts = Array.from(badges).map((b) => b.textContent);
			expect(badgeTexts).not.toContain('5');
			expect(badgeTexts).not.toContain('2');
			expect(badgeTexts).not.toContain('12');

			// Clean up by resolving the promise
			resolvePromise!({
				slashCommandsCount: 0,
				claudeMdSize: 0,
				mcpServersCount: 0,
				permissionsProfile: 'default',
			});
		});
	});

	describe('Error handling - API failure graceful degradation (BDD-3)', () => {
		it('shows no badges when API fails', async () => {
			mockGetConfigStats.mockRejectedValue(new Error('Network error'));

			const { container } = renderWithRouter();

			// Wait for the error to be caught
			await waitFor(() => {
				expect(mockGetConfigStats).toHaveBeenCalled();
			});

			// Give time for error handling
			await new Promise((resolve) => setTimeout(resolve, 50));

			// No badges should be shown on error
			const badges = container.querySelectorAll('.settings-nav-item__badge');
			expect(badges.length).toBe(0);
		});

		it('does not show error UI in navigation on API failure', async () => {
			mockGetConfigStats.mockRejectedValue(new Error('Server error'));

			renderWithRouter();

			await waitFor(() => {
				expect(mockGetConfigStats).toHaveBeenCalled();
			});

			// Navigation should still work, just without badges
			expect(screen.getByText('Slash Commands')).toBeInTheDocument();
			expect(screen.getByText('MCP Servers')).toBeInTheDocument();
			expect(screen.getByText('Memory')).toBeInTheDocument();
		});
	});

	describe('API call behavior', () => {
		it('calls getConfigStats on component mount', async () => {
			mockGetConfigStats.mockResolvedValue({
				slashCommandsCount: 1,
				claudeMdSize: 100,
				mcpServersCount: 1,
				permissionsProfile: 'default',
			});

			renderWithRouter();

			await waitFor(() => {
				expect(mockGetConfigStats).toHaveBeenCalledTimes(1);
			});
		});
	});

	describe('Both badges show correct counts simultaneously', () => {
		it('displays both Slash Commands and MCP Servers badges with correct counts', async () => {
			mockGetConfigStats.mockResolvedValue({
				slashCommandsCount: 8,
				claudeMdSize: 2048,
				mcpServersCount: 4,
				permissionsProfile: 'default',
			});

			renderWithRouter();

			await waitFor(() => {
				const commandsBadge = getBadgeForNavItem('Slash Commands');
				const mcpBadge = getBadgeForNavItem('MCP Servers');

				expect(commandsBadge).toBeInTheDocument();
				expect(commandsBadge?.textContent).toBe('8');

				expect(mcpBadge).toBeInTheDocument();
				expect(mcpBadge?.textContent).toBe('4');
			});
		});
	});

	describe('SC-5: No hard-coded mock values in source', () => {
		it('badges reflect API response, not hard-coded values', async () => {
			// This test verifies the implementation uses API data, not hard-coded values
			// Uses unique values (99, 77) that can't be mistaken for the original mocks (5, 2, 12)
			const mockStats = {
				slashCommandsCount: 99,
				claudeMdSize: 0,
				mcpServersCount: 77,
				permissionsProfile: 'default',
			};

			mockGetConfigStats.mockResolvedValue(mockStats);

			renderWithRouter();

			// If implementation still uses hard-coded values, these will show 5 and 2
			// If implementation correctly uses API, these will show 99 and 77
			await waitFor(() => {
				const commandsBadge = getBadgeForNavItem('Slash Commands');
				const mcpBadge = getBadgeForNavItem('MCP Servers');

				// These assertions prove the implementation uses API data
				expect(commandsBadge?.textContent).toBe('99');
				expect(mcpBadge?.textContent).toBe('77');
			});
		});
	});
});
