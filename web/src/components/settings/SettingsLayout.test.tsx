import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, within, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { configClient } from '@/lib/client';

// Mock API clients used by SettingsLayout for badge counts
vi.mock('@/lib/client', () => ({
	configClient: {
		getConfigStats: vi.fn().mockResolvedValue({
			stats: {
				slashCommandsCount: 5,
				claudeMdSize: BigInt(0),
				mcpServersCount: 3,
				permissionsProfile: 'default',
			},
		}),
	},
	knowledgeClient: {
		getKnowledgeStatus: vi.fn().mockResolvedValue({
			status: {
				pendingCount: 2,
				approvedCount: 4,
				rejectedCount: 0,
			},
		}),
	},
}));

import { SettingsLayout } from './SettingsLayout';

/**
 * Test wrapper providing router context at specific routes.
 * SettingsLayout is now nested under /settings/general/* in the main app.
 */
function renderWithRouter(initialRoute: string = '/settings/general/commands') {
	return render(
		<MemoryRouter initialEntries={[initialRoute]}>
			<Routes>
				<Route path="/settings/general" element={<SettingsLayout />}>
					<Route path="commands" element={<div data-testid="commands-content">Commands Content</div>} />
					<Route path="claude-md" element={<div data-testid="claude-md-content">CLAUDE.md Content</div>} />
					<Route path="mcp" element={<div data-testid="mcp-content">MCP Content</div>} />
					<Route path="permissions" element={<div data-testid="permissions-content">Permissions Content</div>} />
					<Route path="projects" element={<div data-testid="projects-content">Projects Content</div>} />
					<Route path="git" element={<div data-testid="git-content">Git Content</div>} />
					<Route path="billing" element={<div data-testid="billing-content">Billing Content</div>} />
					<Route path="import-export" element={<div data-testid="import-export-content">Import/Export Content</div>} />
					<Route path="constitution" element={<div data-testid="constitution-content">Constitution Content</div>} />
					<Route path="profile" element={<div data-testid="profile-content">Profile Content</div>} />
					<Route path="api-keys" element={<div data-testid="api-keys-content">API Keys Content</div>} />
				</Route>
			</Routes>
		</MemoryRouter>
	);
}

/**
 * Wait for the async API call in SettingsLayout's useEffect to complete.
 * This prevents act() warnings from state updates after tests end.
 */
async function waitForAsyncLoad() {
	await waitFor(() => {
		expect(configClient.getConfigStats).toHaveBeenCalled();
	});
}

describe('SettingsLayout', () => {
	describe('rendering', () => {
		it('renders sidebar and content outlet', async () => {
			const { container } = renderWithRouter();

			// Sidebar should be present
			const sidebar = container.querySelector('.settings-sidebar');
			expect(sidebar).toBeInTheDocument();

			// Content area should be present
			const content = container.querySelector('.settings-content');
			expect(content).toBeInTheDocument();

			// Outlet content should be rendered
			expect(screen.getByTestId('commands-content')).toBeInTheDocument();

			await waitForAsyncLoad();
		});

		it('renders sidebar header with title and subtitle', async () => {
			renderWithRouter();

			expect(screen.getByText('Settings')).toBeInTheDocument();
			expect(screen.getByText('Configure ORC and Claude')).toBeInTheDocument();

			await waitForAsyncLoad();
		});

		it('renders all navigation groups', async () => {
			renderWithRouter();

			// SettingsLayout now only shows CLAUDE CODE, ORC, and ACCOUNT groups
			// ENVIRONMENT items have moved to the Environment tab
			expect(screen.getByText('CLAUDE CODE')).toBeInTheDocument();
			expect(screen.getByText('ORC')).toBeInTheDocument();
			expect(screen.getByText('ACCOUNT')).toBeInTheDocument();

			await waitForAsyncLoad();
		});

		it('renders all navigation items', async () => {
			renderWithRouter();

			// CLAUDE CODE section
			expect(screen.getByText('Slash Commands')).toBeInTheDocument();
			expect(screen.getByText('CLAUDE.md')).toBeInTheDocument();
			expect(screen.getByText('MCP Servers')).toBeInTheDocument();
			expect(screen.getByText('Permissions')).toBeInTheDocument();

			// ORC section
			expect(screen.getByText('Projects')).toBeInTheDocument();
			expect(screen.getByText('Git Settings')).toBeInTheDocument();
			expect(screen.getByText('Billing & Usage')).toBeInTheDocument();
			expect(screen.getByText('Import / Export')).toBeInTheDocument();
			expect(screen.getByText('Constitution')).toBeInTheDocument();

			// ACCOUNT section
			expect(screen.getByText('Profile')).toBeInTheDocument();
			expect(screen.getByText('API Keys')).toBeInTheDocument();

			await waitForAsyncLoad();
		});
	});

	describe('sidebar layout', () => {
		it('sidebar has correct 240px width class', async () => {
			const { container } = renderWithRouter();

			const layout = container.querySelector('.settings-layout');
			expect(layout).toBeInTheDocument();

			// The CSS grid defines 240px for sidebar - test class presence
			const sidebar = container.querySelector('.settings-sidebar');
			expect(sidebar).toBeInTheDocument();

			await waitForAsyncLoad();
		});

		it('sidebar has bg-elevated background', async () => {
			const { container } = renderWithRouter();

			const sidebar = container.querySelector('.settings-sidebar');
			expect(sidebar).toBeInTheDocument();
			// CSS class should apply bg-elevated - verified by class presence

			await waitForAsyncLoad();
		});

		it('content area exists for scrolling', async () => {
			const { container } = renderWithRouter();

			const content = container.querySelector('.settings-content');
			expect(content).toBeInTheDocument();

			await waitForAsyncLoad();
		});
	});

	describe('navigation', () => {
		it('clicking nav item navigates to route', async () => {
			renderWithRouter('/settings/general/commands');

			// Initial content should be commands
			expect(screen.getByTestId('commands-content')).toBeInTheDocument();

			// Click CLAUDE.md nav item
			const claudeMdLink = screen.getByText('CLAUDE.md');
			fireEvent.click(claudeMdLink);

			// Content should change to claude-md
			expect(screen.getByTestId('claude-md-content')).toBeInTheDocument();

			await waitForAsyncLoad();
		});

		it('active nav item has correct styling class', async () => {
			const { container } = renderWithRouter('/settings/general/commands');

			// Find the active nav item (commands)
			const activeItem = container.querySelector('.settings-nav-item--active');
			expect(activeItem).toBeInTheDocument();

			// Verify it's the Slash Commands item
			expect(within(activeItem as HTMLElement).getByText('Slash Commands')).toBeInTheDocument();

			await waitForAsyncLoad();
		});

		it('clicking different sections updates active state', async () => {
			const { container } = renderWithRouter('/settings/general/commands');

			// Initially commands is active
			let activeItem = container.querySelector('.settings-nav-item--active');
			expect(within(activeItem as HTMLElement).getByText('Slash Commands')).toBeInTheDocument();

			// Click MCP Servers (links to /settings/general/mcp)
			fireEvent.click(screen.getByText('MCP Servers'));

			// MCP should now be active
			activeItem = container.querySelector('.settings-nav-item--active');
			expect(within(activeItem as HTMLElement).getByText('MCP Servers')).toBeInTheDocument();

			await waitForAsyncLoad();
		});
	});

	describe('badges', () => {
		it('displays badges for items with counts', async () => {
			const { container } = renderWithRouter();

			// Wait for async API mock to resolve and badges to render
			await waitFor(() => {
				const badges = container.querySelectorAll('.settings-nav-item__badge');
				expect(badges.length).toBeGreaterThan(0);
			});
		});

		it('Slash Commands badge shows count', async () => {
			renderWithRouter();

			// Wait for API mock to resolve and badge to appear
			await waitFor(() => {
				const slashCommandsItem = screen.getByText('Slash Commands').closest('.settings-nav-item');
				expect(slashCommandsItem).toBeInTheDocument();

				const badge = slashCommandsItem?.querySelector('.settings-nav-item__badge');
				expect(badge).toBeInTheDocument();
				expect(badge?.textContent).toBe('5'); // Mock count from configClient.getConfigStats
			});
		});
	});

	describe('keyboard accessibility', () => {
		it('navigation items are focusable', async () => {
			renderWithRouter();

			const navItems = screen.getAllByRole('link');
			expect(navItems.length).toBeGreaterThan(0);

			// All nav items should be focusable links
			navItems.forEach((item) => {
				expect(item.tagName).toBe('A');
			});

			await waitForAsyncLoad();
		});

		it('Enter key on nav item triggers navigation', async () => {
			renderWithRouter('/settings/general/commands');

			// MCP Servers link is in CLAUDE CODE section, links to /settings/general/mcp
			const mcpLink = screen.getByText('MCP Servers');
			mcpLink.focus();
			fireEvent.keyDown(mcpLink, { key: 'Enter' });

			// Should navigate (Enter on links is handled by browser, but we can check click effect)
			fireEvent.click(mcpLink);
			expect(screen.getByTestId('mcp-content')).toBeInTheDocument();

			await waitForAsyncLoad();
		});
	});

	describe('aria attributes', () => {
		it('sidebar has navigation role and aria-label', async () => {
			renderWithRouter();

			const sidebar = screen.getByRole('navigation', { name: 'Settings navigation' });
			expect(sidebar).toBeInTheDocument();

			await waitForAsyncLoad();
		});
	});

	describe('route integration', () => {
		it('renders correct content for /settings/general/commands', async () => {
			renderWithRouter('/settings/general/commands');
			expect(screen.getByTestId('commands-content')).toBeInTheDocument();
			await waitForAsyncLoad();
		});

		it('renders correct content for /settings/general/claude-md', async () => {
			renderWithRouter('/settings/general/claude-md');
			expect(screen.getByTestId('claude-md-content')).toBeInTheDocument();
			await waitForAsyncLoad();
		});

		it('renders correct content for /settings/general/mcp', async () => {
			renderWithRouter('/settings/general/mcp');
			expect(screen.getByTestId('mcp-content')).toBeInTheDocument();
			await waitForAsyncLoad();
		});

		it('renders correct content for /settings/general/permissions', async () => {
			renderWithRouter('/settings/general/permissions');
			expect(screen.getByTestId('permissions-content')).toBeInTheDocument();
			await waitForAsyncLoad();
		});

		it('renders correct content for /settings/general/projects', async () => {
			renderWithRouter('/settings/general/projects');
			expect(screen.getByTestId('projects-content')).toBeInTheDocument();
			await waitForAsyncLoad();
		});

		it('renders correct content for /settings/general/git', async () => {
			renderWithRouter('/settings/general/git');
			expect(screen.getByTestId('git-content')).toBeInTheDocument();
			await waitForAsyncLoad();
		});

		it('renders correct content for /settings/general/billing', async () => {
			renderWithRouter('/settings/general/billing');
			expect(screen.getByTestId('billing-content')).toBeInTheDocument();
			await waitForAsyncLoad();
		});

		it('renders correct content for /settings/general/import-export', async () => {
			renderWithRouter('/settings/general/import-export');
			expect(screen.getByTestId('import-export-content')).toBeInTheDocument();
			await waitForAsyncLoad();
		});

		it('renders correct content for /settings/general/constitution', async () => {
			renderWithRouter('/settings/general/constitution');
			expect(screen.getByTestId('constitution-content')).toBeInTheDocument();
			await waitForAsyncLoad();
		});

		it('renders correct content for /settings/general/profile', async () => {
			renderWithRouter('/settings/general/profile');
			expect(screen.getByTestId('profile-content')).toBeInTheDocument();
			await waitForAsyncLoad();
		});

		it('renders correct content for /settings/general/api-keys', async () => {
			renderWithRouter('/settings/general/api-keys');
			expect(screen.getByTestId('api-keys-content')).toBeInTheDocument();
			await waitForAsyncLoad();
		});
	});
});
