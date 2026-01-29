import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { AgentsView } from './AgentsView';
import { configClient } from '@/lib/client';
import type { Agent, Config } from '@/gen/orc/v1/config_pb';
import { SettingsScope } from '@/gen/orc/v1/config_pb';

// Mock the configClient
vi.mock('@/lib/client', () => ({
	configClient: {
		listAgents: vi.fn(),
		getConfig: vi.fn(),
		updateConfig: vi.fn(),
	},
}));

describe('AgentsView', () => {
	// Proto-compatible mock agents
	const mockAgents: Partial<Agent>[] = [
		{
			name: 'Primary Coder',
			description: 'Main coding agent',
			model: 'claude-sonnet-4-20250514',
			tools: { allow: ['File Read', 'File Write', 'Bash'], deny: [], $typeName: 'orc.v1.ToolPermissions' },
			skillRefs: [],
			scope: SettingsScope.PROJECT,
		},
		{
			name: 'Reviewer',
			description: 'Code review agent',
			model: 'claude-opus-4-20250514',
			tools: { allow: ['File Read', 'Git'], deny: [], $typeName: 'orc.v1.ToolPermissions' },
			skillRefs: [],
			scope: SettingsScope.PROJECT,
		},
		{
			name: 'Docs Agent',
			description: 'Documentation writer',
			model: 'claude-haiku-3-5-20241022',
			tools: { allow: [], deny: [], $typeName: 'orc.v1.ToolPermissions' },
			path: 'docs-tools.md',
			skillRefs: [],
			scope: SettingsScope.GLOBAL,
		},
	];

	// Proto-compatible mock config
	const mockConfig: Partial<Config> = {
		automation: {
			profile: 'auto',
			autoApprove: true,
			autoSkip: false,
			$typeName: 'orc.v1.AutomationConfig',
		},
		claude: {
			model: 'claude-sonnet-4-20250514',
			thinking: false,
			maxTurns: 100,
			temperature: 0,
			$typeName: 'orc.v1.ClaudeConfig',
		},
		completion: {
			action: 'finalize',
			autoMerge: false,
			targetBranch: 'main',
			$typeName: 'orc.v1.CompletionConfig',
		},
		export: {
			includeTranscripts: true,
			includeAttachments: true,
			format: 'tar.gz',
			$typeName: 'orc.v1.ExportConfig',
		},
	};

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(configClient.listAgents).mockResolvedValue({ agents: mockAgents as Agent[], $typeName: 'orc.v1.ListAgentsResponse' });
		vi.mocked(configClient.getConfig).mockResolvedValue({ config: mockConfig as Config, $typeName: 'orc.v1.GetConfigResponse' });
		vi.mocked(configClient.updateConfig).mockResolvedValue({ config: mockConfig as Config, $typeName: 'orc.v1.UpdateConfigResponse' });
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
			vi.mocked(configClient.listAgents).mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve({ agents: mockAgents as Agent[], $typeName: 'orc.v1.ListAgentsResponse' }), 100))
			);

			render(<AgentsView />);

			// Should show skeleton cards
			expect(document.querySelector('.agents-view-card-skeleton')).toBeInTheDocument();
		});

		it('shows skeleton grid with aria-busy', async () => {
			vi.mocked(configClient.listAgents).mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve({ agents: mockAgents as Agent[], $typeName: 'orc.v1.ListAgentsResponse' }), 100))
			);

			render(<AgentsView />);

			const grid = document.querySelector('.agents-view-grid');
			expect(grid).toHaveAttribute('aria-busy', 'true');
		});
	});

	describe('error state', () => {
		it('displays error state with retry button when fetch fails', async () => {
			vi.mocked(configClient.listAgents).mockRejectedValue(new Error('Network error'));

			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByText('Failed to load agents')).toBeInTheDocument();
				expect(screen.getByText('Network error')).toBeInTheDocument();
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});
		});

		it('retries loading when retry button is clicked', async () => {
			vi.mocked(configClient.listAgents)
				.mockRejectedValueOnce(new Error('Failed'))
				.mockResolvedValueOnce({ agents: mockAgents as Agent[], $typeName: 'orc.v1.ListAgentsResponse' });

			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /retry/i }));

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalledTimes(2);
			});
		});

		it('has alert role for accessibility', async () => {
			vi.mocked(configClient.listAgents).mockRejectedValue(new Error('Failed'));

			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByRole('alert')).toBeInTheDocument();
			});
		});
	});

	describe('empty state', () => {
		it('displays empty state when agents array is empty', async () => {
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [], $typeName: 'orc.v1.ListAgentsResponse' });

			render(<AgentsView />);

			await waitFor(() => {
				expect(screen.getByText('Create your first agent')).toBeInTheDocument();
			});
		});

		it('shows helpful description in empty state', async () => {
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [], $typeName: 'orc.v1.ListAgentsResponse' });

			render(<AgentsView />);

			await waitFor(() => {
				expect(
					screen.getByText(/agents are configured claude instances/i)
				).toBeInTheDocument();
			});
		});

		it('has status role for accessibility', async () => {
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [], $typeName: 'orc.v1.ListAgentsResponse' });

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
		it('is disabled until feature is implemented', async () => {
			render(<AgentsView />);

			await waitFor(() => {
				const button = screen.getByRole('button', { name: /add agent/i });
				expect(button).toBeDisabled();
			});
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
		vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [], $typeName: 'orc.v1.ListAgentsResponse' });
		vi.mocked(configClient.getConfig).mockResolvedValue({
			config: {
				automation: {
					profile: 'auto',
					autoApprove: true,
					autoSkip: false,
					$typeName: 'orc.v1.AutomationConfig',
				},
				claude: {
					model: 'claude-sonnet-4-20250514',
					thinking: false,
					maxTurns: 100,
					temperature: 0,
					$typeName: 'orc.v1.ClaudeConfig',
				},
				completion: {
					action: 'finalize',
					autoMerge: false,
					targetBranch: 'main',
					$typeName: 'orc.v1.CompletionConfig',
				},
				export: {
					includeTranscripts: true,
					includeAttachments: true,
					format: 'tar.gz',
					$typeName: 'orc.v1.ExportConfig',
				},
				$typeName: 'orc.v1.Config',
			} as Config,
			$typeName: 'orc.v1.GetConfigResponse',
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
