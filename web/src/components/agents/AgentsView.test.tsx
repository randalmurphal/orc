import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { AgentsView } from './AgentsView';
import * as api from '@/lib/api';
import type { SubAgent, Config } from '@/lib/api';

// Mock the API functions
vi.mock('@/lib/api', () => ({
	listAgents: vi.fn(),
	getConfig: vi.fn(),
	updateConfig: vi.fn(),
}));

describe('AgentsView', () => {
	const mockAgents: SubAgent[] = [
		{
			name: 'Primary Coder',
			description: 'Main coding agent',
			model: 'claude-sonnet-4-20250514',
			tools: { allow: ['File Read', 'File Write', 'Bash'] },
		},
		{
			name: 'Reviewer',
			description: 'Code review agent',
			model: 'claude-opus-4-20250514',
			tools: { allow: ['File Read', 'Git'] },
		},
		{
			name: 'Docs Agent',
			description: 'Documentation writer',
			model: 'claude-haiku-3-5-20241022',
			tools: 'docs-tools.md',
		},
	];

	const mockConfig: Config = {
		version: '1.0',
		profile: 'auto',
		automation: {
			profile: 'auto',
			gates_default: 'skip',
			retry_enabled: true,
			retry_max: 3,
		},
		execution: {
			model: 'claude-sonnet-4-20250514',
			max_iterations: 100,
			timeout: '30m',
		},
		git: {
			branch_prefix: 'orc/',
			commit_prefix: '[orc]',
		},
		worktree: {
			enabled: true,
			dir: '.orc/worktrees',
			cleanup_on_complete: true,
			cleanup_on_fail: false,
		},
		completion: {
			action: 'finalize',
			target_branch: 'main',
			delete_branch: true,
		},
		timeouts: {
			phase_max: '30m',
			turn_max: '5m',
			idle_warning: '2m',
			heartbeat_interval: '30s',
			idle_timeout: '10m',
		},
	};

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.listAgents).mockResolvedValue(mockAgents);
		vi.mocked(api.getConfig).mockResolvedValue(mockConfig);
		vi.mocked(api.updateConfig).mockResolvedValue(mockConfig);
	});

	describe('page header', () => {
		it('renders with title', async () => {
			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByText('Agents')).toBeInTheDocument();
			});
		});

		it('renders with subtitle', async () => {
			render(<AgentsView />);

			await waitFor(() => {
				expect(
					screen.getByText('Configure Claude models and execution settings')
				).toBeInTheDocument();
			});
		});

		it('renders Add Agent button', async () => {
			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /add agent/i })).toBeInTheDocument();
			});
		});
	});

	describe('loading state', () => {
		it('displays loading skeletons during fetch', async () => {
			// Delay the API response
			vi.mocked(api.listAgents).mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve(mockAgents), 100))
			);

			render(<AgentsView />);

			// Should show skeleton cards
			expect(document.querySelector('.agents-view-card-skeleton')).toBeInTheDocument();
		});

		it('shows skeleton grid with aria-busy', async () => {
			vi.mocked(api.listAgents).mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve(mockAgents), 100))
			);

			render(<AgentsView />);

			const grid = document.querySelector('.agents-view-grid');
			expect(grid).toHaveAttribute('aria-busy', 'true');
		});
	});

	describe('error state', () => {
		it('displays error state with retry button when fetch fails', async () => {
			vi.mocked(api.listAgents).mockRejectedValue(new Error('Network error'));

			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByText('Failed to load agents')).toBeInTheDocument();
				expect(screen.getByText('Network error')).toBeInTheDocument();
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});
		});

		it('retries loading when retry button is clicked', async () => {
			vi.mocked(api.listAgents)
				.mockRejectedValueOnce(new Error('Failed'))
				.mockResolvedValueOnce(mockAgents);

			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /retry/i }));

			await waitFor(() => {
				expect(api.listAgents).toHaveBeenCalledTimes(2);
			});
		});

		it('has alert role for accessibility', async () => {
			vi.mocked(api.listAgents).mockRejectedValue(new Error('Failed'));

			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByRole('alert')).toBeInTheDocument();
			});
		});
	});

	describe('empty state', () => {
		it('displays empty state when agents array is empty', async () => {
			vi.mocked(api.listAgents).mockResolvedValue([]);

			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByText('Create your first agent')).toBeInTheDocument();
			});
		});

		it('shows helpful description in empty state', async () => {
			vi.mocked(api.listAgents).mockResolvedValue([]);

			render(<AgentsView />);

			await waitFor(() => {
				expect(
					screen.getByText(/agents are configured claude instances/i)
				).toBeInTheDocument();
			});
		});

		it('has status role for accessibility', async () => {
			vi.mocked(api.listAgents).mockResolvedValue([]);

			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByRole('status')).toBeInTheDocument();
			});
		});
	});

	describe('AgentCard grid', () => {
		it('renders AgentCard grid when agents exist', async () => {
			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByText('Primary Coder')).toBeInTheDocument();
				expect(screen.getByText('Reviewer')).toBeInTheDocument();
				expect(screen.getByText('Docs Agent')).toBeInTheDocument();
			});
		});

		it('displays agent model information', async () => {
			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByText('claude-sonnet-4-20250514')).toBeInTheDocument();
				expect(screen.getByText('claude-opus-4-20250514')).toBeInTheDocument();
			});
		});
	});

	describe('ExecutionSettings section', () => {
		it('renders ExecutionSettings section', async () => {
			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByText('Execution Settings')).toBeInTheDocument();
				expect(
					screen.getByText('Global configuration for all agents')
				).toBeInTheDocument();
			});
		});

		it('displays execution settings controls', async () => {
			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByText('Parallel Tasks')).toBeInTheDocument();
				expect(screen.getByText('Auto-Approve')).toBeInTheDocument();
				expect(screen.getByText('Default Model')).toBeInTheDocument();
				expect(screen.getByText('Cost Limit')).toBeInTheDocument();
			});
		});
	});

	describe('ToolPermissions section', () => {
		it('renders ToolPermissions section', async () => {
			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByText('Tool Permissions')).toBeInTheDocument();
				expect(
					screen.getByText('Control what actions agents can perform')
				).toBeInTheDocument();
			});
		});

		it('displays tool permission toggles', async () => {
			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByText('File Read')).toBeInTheDocument();
				expect(screen.getByText('File Write')).toBeInTheDocument();
				expect(screen.getByText('Bash Commands')).toBeInTheDocument();
			});
		});
	});

	describe('Add Agent button', () => {
		it('dispatches orc:add-agent event when clicked', async () => {
			const dispatchSpy = vi.spyOn(window, 'dispatchEvent');

			render(<AgentsView />);

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: /add agent/i }));
			});

			expect(dispatchSpy).toHaveBeenCalledWith(
				expect.objectContaining({
					type: 'orc:add-agent',
				})
			);

			dispatchSpy.mockRestore();
		});
	});

	describe('accessibility', () => {
		it('has proper heading hierarchy', async () => {
			render(<AgentsView />);

			await waitFor(() => {
				const heading = screen.getByRole('heading', { level: 1 });
				expect(heading).toHaveTextContent('Agents');
			});
		});

		it('has section headings at level 2', async () => {
			render(<AgentsView />);

			await waitFor(() => {
				const h2s = screen.getAllByRole('heading', { level: 2 });
				expect(h2s.length).toBeGreaterThanOrEqual(3);
			});
		});
	});
});

describe('AgentsPage', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.listAgents).mockResolvedValue([]);
		vi.mocked(api.getConfig).mockResolvedValue({
			version: '1.0',
			profile: 'auto',
			automation: {
				profile: 'auto',
				gates_default: 'skip',
				retry_enabled: true,
				retry_max: 3,
			},
			execution: {
				model: 'claude-sonnet-4-20250514',
				max_iterations: 100,
				timeout: '30m',
			},
			git: {
				branch_prefix: 'orc/',
				commit_prefix: '[orc]',
			},
			worktree: {
				enabled: true,
				dir: '.orc/worktrees',
				cleanup_on_complete: true,
				cleanup_on_fail: false,
			},
			completion: {
				action: 'finalize',
				target_branch: 'main',
				delete_branch: true,
			},
			timeouts: {
				phase_max: '30m',
				turn_max: '5m',
				idle_warning: '2m',
				heartbeat_interval: '30s',
				idle_timeout: '10m',
			},
		});
	});

	it('wrapper renders AgentsView', async () => {
		// Import dynamically to avoid hoisting issues
		const { AgentsPage } = await import('@/pages/AgentsPage');
		render(<AgentsPage />);

		await waitFor(() => {
			expect(screen.getByText('Agents')).toBeInTheDocument();
		});
	});
});
