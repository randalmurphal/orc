/**
 * TDD Tests for SettingsTabs Component
 *
 * Tests for TASK-723: Create Settings page with Agents and Environment tabs
 *
 * These tests verify the SettingsTabs component which provides:
 * - Three tabs: General, Agents, Environment
 * - URL-driven tab state (syncs with React Router)
 * - Radix Tabs accessibility
 *
 * Coverage mapping:
 * - SC-1: Settings page renders three tabs
 * - SC-2: Clicking Agents tab displays AgentsView
 * - SC-3: Clicking Environment tab displays EnvironmentLayout
 * - SC-4: Direct navigation to /settings/agents shows Agents tab active
 * - SC-5: Direct navigation to /settings/environment shows Environment tab active
 * - SC-6: Navigation to /settings redirects to /settings/general
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route, Navigate } from 'react-router-dom';

// Mock API clients to prevent network calls during tests
vi.mock('@/lib/client', () => ({
	configClient: {
		listAgents: vi.fn().mockResolvedValue({ agents: [] }),
		getConfig: vi.fn().mockResolvedValue({
			config: {
				automation: { profile: 'auto' },
				claude: { model: 'claude-sonnet-4-20250514' },
			},
		}),
	},
}));

// Import after mocking
import { SettingsTabs } from './SettingsTabs';

/**
 * Test wrapper providing router context at specific settings routes.
 * Simulates the route structure after implementation.
 */
function renderWithRouter(initialRoute: string = '/settings/general') {
	return render(
		<MemoryRouter initialEntries={[initialRoute]}>
			<Routes>
				<Route path="/settings" element={<SettingsTabs />}>
					{/* Index redirect to general */}
					<Route index element={<Navigate to="general" replace />} />
					{/* Tab content routes */}
					<Route path="general/*" element={<div data-testid="general-content">General Settings Content</div>} />
					<Route path="agents" element={<div data-testid="agents-content">Agents Content</div>} />
					<Route path="environment/*" element={<div data-testid="environment-content">Environment Content</div>} />
				</Route>
			</Routes>
		</MemoryRouter>
	);
}

describe('SettingsTabs', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('SC-1: Settings page renders three tabs', () => {
		it('renders all three tab triggers with correct labels', () => {
			renderWithRouter('/settings/general');

			// All three tabs should be rendered
			expect(screen.getByRole('tab', { name: /general/i })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: /environment/i })).toBeInTheDocument();
		});

		it('renders tabs in a tablist container', () => {
			renderWithRouter('/settings/general');

			const tablist = screen.getByRole('tablist');
			expect(tablist).toBeInTheDocument();

			// All tabs should be within the tablist
			const tabs = screen.getAllByRole('tab');
			expect(tabs).toHaveLength(3);
		});

		it('has correct aria-label for the tablist', () => {
			renderWithRouter('/settings/general');

			const tablist = screen.getByRole('tablist');
			expect(tablist).toHaveAttribute('aria-label', 'Settings sections');
		});
	});

	describe('SC-2: Clicking Agents tab displays AgentsView', () => {
		it('clicking Agents tab navigates to /settings/agents', async () => {
			renderWithRouter('/settings/general');

			// Initially, general content is shown
			expect(screen.getByTestId('general-content')).toBeInTheDocument();

			// Click Agents tab
			const agentsTab = screen.getByRole('tab', { name: /agents/i });
			fireEvent.click(agentsTab);

			// Agents content should now be displayed
			await waitFor(() => {
				expect(screen.getByTestId('agents-content')).toBeInTheDocument();
			});
		});

		it('Agents tab becomes active after click', async () => {
			renderWithRouter('/settings/general');

			const agentsTab = screen.getByRole('tab', { name: /agents/i });
			fireEvent.click(agentsTab);

			await waitFor(() => {
				expect(agentsTab).toHaveAttribute('data-state', 'active');
			});
		});
	});

	describe('SC-3: Clicking Environment tab displays EnvironmentLayout', () => {
		it('clicking Environment tab navigates to /settings/environment', async () => {
			renderWithRouter('/settings/general');

			// Click Environment tab
			const envTab = screen.getByRole('tab', { name: /environment/i });
			fireEvent.click(envTab);

			// Environment content should be displayed
			await waitFor(() => {
				expect(screen.getByTestId('environment-content')).toBeInTheDocument();
			});
		});

		it('Environment tab becomes active after click', async () => {
			renderWithRouter('/settings/general');

			const envTab = screen.getByRole('tab', { name: /environment/i });
			fireEvent.click(envTab);

			await waitFor(() => {
				expect(envTab).toHaveAttribute('data-state', 'active');
			});
		});
	});

	describe('SC-4: Direct navigation to /settings/agents shows Agents tab active', () => {
		it('Agents tab is active when navigated directly to /settings/agents', () => {
			renderWithRouter('/settings/agents');

			const agentsTab = screen.getByRole('tab', { name: /agents/i });
			expect(agentsTab).toHaveAttribute('data-state', 'active');
		});

		it('Agents content is rendered when navigated directly', () => {
			renderWithRouter('/settings/agents');

			expect(screen.getByTestId('agents-content')).toBeInTheDocument();
		});

		it('General tab is not active when on /settings/agents', () => {
			renderWithRouter('/settings/agents');

			const generalTab = screen.getByRole('tab', { name: /general/i });
			expect(generalTab).toHaveAttribute('data-state', 'inactive');
		});
	});

	describe('SC-5: Direct navigation to /settings/environment shows Environment tab active', () => {
		it('Environment tab is active when navigated directly to /settings/environment', () => {
			renderWithRouter('/settings/environment');

			const envTab = screen.getByRole('tab', { name: /environment/i });
			expect(envTab).toHaveAttribute('data-state', 'active');
		});

		it('Environment content is rendered when navigated directly', () => {
			renderWithRouter('/settings/environment');

			expect(screen.getByTestId('environment-content')).toBeInTheDocument();
		});
	});

	describe('SC-6: Navigation to /settings redirects to /settings/general', () => {
		it('redirects /settings to /settings/general', async () => {
			renderWithRouter('/settings');

			// Should show general content after redirect
			await waitFor(() => {
				expect(screen.getByTestId('general-content')).toBeInTheDocument();
			});
		});

		it('General tab is active after redirect from /settings', async () => {
			renderWithRouter('/settings');

			await waitFor(() => {
				const generalTab = screen.getByRole('tab', { name: /general/i });
				expect(generalTab).toHaveAttribute('data-state', 'active');
			});
		});
	});

	describe('URL sync behavior', () => {
		it('switching tabs updates the URL path', async () => {
			renderWithRouter('/settings/general');

			// Find and click the Agents tab
			const agentsTab = screen.getByRole('tab', { name: /agents/i });
			fireEvent.click(agentsTab);

			// Verify agents content is shown (indicating URL changed)
			await waitFor(() => {
				expect(screen.getByTestId('agents-content')).toBeInTheDocument();
			});
		});

		it('tab state reflects current URL on initial render', () => {
			// Start at agents
			renderWithRouter('/settings/agents');

			const agentsTab = screen.getByRole('tab', { name: /agents/i });
			const generalTab = screen.getByRole('tab', { name: /general/i });
			const envTab = screen.getByRole('tab', { name: /environment/i });

			expect(agentsTab).toHaveAttribute('data-state', 'active');
			expect(generalTab).toHaveAttribute('data-state', 'inactive');
			expect(envTab).toHaveAttribute('data-state', 'inactive');
		});
	});

	describe('Edge cases', () => {
		it('environment sub-routes still show Environment tab as active', () => {
			// Navigate to a sub-route under environment
			renderWithRouter('/settings/environment/hooks');

			const envTab = screen.getByRole('tab', { name: /environment/i });
			expect(envTab).toHaveAttribute('data-state', 'active');
		});

		it('general sub-routes still show General tab as active', () => {
			// Navigate to a sub-route under general
			renderWithRouter('/settings/general/commands');

			const generalTab = screen.getByRole('tab', { name: /general/i });
			expect(generalTab).toHaveAttribute('data-state', 'active');
		});
	});

	describe('Keyboard accessibility', () => {
		it('tabs are focusable', () => {
			renderWithRouter('/settings/general');

			const tabs = screen.getAllByRole('tab');
			tabs.forEach((tab) => {
				expect(tab).not.toHaveAttribute('tabIndex', '-1');
			});
		});

		it('arrow keys navigate between tabs', async () => {
			renderWithRouter('/settings/general');

			const generalTab = screen.getByRole('tab', { name: /general/i });
			generalTab.focus();

			// Press right arrow to move to Agents
			fireEvent.keyDown(generalTab, { key: 'ArrowRight' });

			// Agents tab should now be focused
			await waitFor(() => {
				const agentsTab = screen.getByRole('tab', { name: /agents/i });
				expect(document.activeElement).toBe(agentsTab);
			});
		});
	});

	describe('Error boundary integration', () => {
		it('renders without crashing when mounted', () => {
			expect(() => renderWithRouter('/settings/general')).not.toThrow();
		});
	});
});
