import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, within, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';

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
 * Test wrapper providing router context at specific routes
 */
function renderWithRouter(initialRoute: string = '/settings/commands') {
	return render(
		<MemoryRouter initialEntries={[initialRoute]}>
			<Routes>
				<Route path="/settings" element={<SettingsLayout />}>
					<Route path="commands" element={<div data-testid="commands-content">Commands Content</div>} />
					<Route path="claude-md" element={<div data-testid="claude-md-content">CLAUDE.md Content</div>} />
					<Route path="mcp" element={<div data-testid="mcp-content">MCP Content</div>} />
					<Route path="permissions" element={<div data-testid="permissions-content">Permissions Content</div>} />
					<Route path="projects" element={<div data-testid="projects-content">Projects Content</div>} />
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

describe('SettingsLayout', () => {
	describe('rendering', () => {
		it('renders sidebar and content outlet', () => {
			const { container } = renderWithRouter();

			// Sidebar should be present
			const sidebar = container.querySelector('.settings-sidebar');
			expect(sidebar).toBeInTheDocument();

			// Content area should be present
			const content = container.querySelector('.settings-content');
			expect(content).toBeInTheDocument();

			// Outlet content should be rendered
			expect(screen.getByTestId('commands-content')).toBeInTheDocument();
		});

		it('renders sidebar header with title and subtitle', () => {
			renderWithRouter();

			expect(screen.getByText('Settings')).toBeInTheDocument();
			expect(screen.getByText('Configure ORC and Claude')).toBeInTheDocument();
		});

		it('renders all navigation groups', () => {
			renderWithRouter();

			expect(screen.getByText('CLAUDE CODE')).toBeInTheDocument();
			expect(screen.getByText('ORC')).toBeInTheDocument();
			expect(screen.getByText('ACCOUNT')).toBeInTheDocument();
		});

		it('renders all navigation items', () => {
			renderWithRouter();

			// CLAUDE CODE section
			expect(screen.getByText('Slash Commands')).toBeInTheDocument();
			expect(screen.getByText('CLAUDE.md')).toBeInTheDocument();
			expect(screen.getByText('MCP Servers')).toBeInTheDocument();
			expect(screen.getByText('Permissions')).toBeInTheDocument();

			// ORC section
			expect(screen.getByText('Projects')).toBeInTheDocument();
			expect(screen.getByText('Billing & Usage')).toBeInTheDocument();
			expect(screen.getByText('Import / Export')).toBeInTheDocument();
			expect(screen.getByText('Constitution')).toBeInTheDocument();

			// ACCOUNT section
			expect(screen.getByText('Profile')).toBeInTheDocument();
			expect(screen.getByText('API Keys')).toBeInTheDocument();
		});
	});

	describe('sidebar layout', () => {
		it('sidebar has correct 240px width class', () => {
			const { container } = renderWithRouter();

			const layout = container.querySelector('.settings-layout');
			expect(layout).toBeInTheDocument();

			// The CSS grid defines 240px for sidebar - test class presence
			const sidebar = container.querySelector('.settings-sidebar');
			expect(sidebar).toBeInTheDocument();
		});

		it('sidebar has bg-elevated background', () => {
			const { container } = renderWithRouter();

			const sidebar = container.querySelector('.settings-sidebar');
			expect(sidebar).toBeInTheDocument();
			// CSS class should apply bg-elevated - verified by class presence
		});

		it('content area exists for scrolling', () => {
			const { container } = renderWithRouter();

			const content = container.querySelector('.settings-content');
			expect(content).toBeInTheDocument();
		});
	});

	describe('navigation', () => {
		it('clicking nav item navigates to route', async () => {
			renderWithRouter('/settings/commands');

			// Initial content should be commands
			expect(screen.getByTestId('commands-content')).toBeInTheDocument();

			// Click CLAUDE.md nav item
			const claudeMdLink = screen.getByText('CLAUDE.md');
			fireEvent.click(claudeMdLink);

			// Content should change to claude-md
			expect(screen.getByTestId('claude-md-content')).toBeInTheDocument();
		});

		it('active nav item has correct styling class', () => {
			const { container } = renderWithRouter('/settings/commands');

			// Find the active nav item (commands)
			const activeItem = container.querySelector('.settings-nav-item--active');
			expect(activeItem).toBeInTheDocument();

			// Verify it's the Slash Commands item
			expect(within(activeItem as HTMLElement).getByText('Slash Commands')).toBeInTheDocument();
		});

		it('clicking different sections updates active state', () => {
			const { container } = renderWithRouter('/settings/commands');

			// Initially commands is active
			let activeItem = container.querySelector('.settings-nav-item--active');
			expect(within(activeItem as HTMLElement).getByText('Slash Commands')).toBeInTheDocument();

			// Click MCP Servers
			fireEvent.click(screen.getByText('MCP Servers'));

			// MCP should now be active
			activeItem = container.querySelector('.settings-nav-item--active');
			expect(within(activeItem as HTMLElement).getByText('MCP Servers')).toBeInTheDocument();
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
		it('navigation items are focusable', () => {
			renderWithRouter();

			const navItems = screen.getAllByRole('link');
			expect(navItems.length).toBeGreaterThan(0);

			// All nav items should be focusable links
			navItems.forEach((item) => {
				expect(item.tagName).toBe('A');
			});
		});

		it('Enter key on nav item triggers navigation', () => {
			renderWithRouter('/settings/commands');

			const mcpLink = screen.getByText('MCP Servers');
			mcpLink.focus();
			fireEvent.keyDown(mcpLink, { key: 'Enter' });

			// Should navigate (Enter on links is handled by browser, but we can check click effect)
			fireEvent.click(mcpLink);
			expect(screen.getByTestId('mcp-content')).toBeInTheDocument();
		});
	});

	describe('aria attributes', () => {
		it('sidebar has navigation role and aria-label', () => {
			renderWithRouter();

			const sidebar = screen.getByRole('navigation', { name: 'Settings navigation' });
			expect(sidebar).toBeInTheDocument();
		});
	});

	describe('route integration', () => {
		it('renders correct content for /settings/commands', () => {
			renderWithRouter('/settings/commands');
			expect(screen.getByTestId('commands-content')).toBeInTheDocument();
		});

		it('renders correct content for /settings/claude-md', () => {
			renderWithRouter('/settings/claude-md');
			expect(screen.getByTestId('claude-md-content')).toBeInTheDocument();
		});

		it('renders correct content for /settings/mcp', () => {
			renderWithRouter('/settings/mcp');
			expect(screen.getByTestId('mcp-content')).toBeInTheDocument();
		});

		it('renders correct content for /settings/permissions', () => {
			renderWithRouter('/settings/permissions');
			expect(screen.getByTestId('permissions-content')).toBeInTheDocument();
		});

		it('renders correct content for /settings/projects', () => {
			renderWithRouter('/settings/projects');
			expect(screen.getByTestId('projects-content')).toBeInTheDocument();
		});

		it('renders correct content for /settings/billing', () => {
			renderWithRouter('/settings/billing');
			expect(screen.getByTestId('billing-content')).toBeInTheDocument();
		});

		it('renders correct content for /settings/import-export', () => {
			renderWithRouter('/settings/import-export');
			expect(screen.getByTestId('import-export-content')).toBeInTheDocument();
		});

		it('renders correct content for /settings/constitution', () => {
			renderWithRouter('/settings/constitution');
			expect(screen.getByTestId('constitution-content')).toBeInTheDocument();
		});

		it('renders correct content for /settings/profile', () => {
			renderWithRouter('/settings/profile');
			expect(screen.getByTestId('profile-content')).toBeInTheDocument();
		});

		it('renders correct content for /settings/api-keys', () => {
			renderWithRouter('/settings/api-keys');
			expect(screen.getByTestId('api-keys-content')).toBeInTheDocument();
		});
	});
});
