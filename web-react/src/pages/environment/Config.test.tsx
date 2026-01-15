import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { Config } from './Config';
import * as api from '@/lib/api';

// Mock the API functions
vi.mock('@/lib/api', () => ({
	getConfig: vi.fn(),
	updateConfig: vi.fn(),
}));

describe('Config', () => {
	const mockConfig: api.Config = {
		profile: 'auto',
		automation: {
			gates_default: 'default',
			retry_enabled: true,
			retry_max: 3,
		},
		execution: {
			model: 'claude-sonnet-4-20250514',
			max_iterations: 10,
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
			action: 'pr',
			target_branch: 'main',
			delete_branch: true,
		},
		timeouts: {
			phase_max: '30m',
			turn_max: '5m',
			idle_warning: '2m',
			heartbeat_interval: '10s',
			idle_timeout: '5m',
		},
	};

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.getConfig).mockResolvedValue(mockConfig);
	});

	const renderConfig = (initialPath: string = '/environment/config') => {
		return render(
			<MemoryRouter initialEntries={[initialPath]}>
				<Routes>
					<Route path="/environment/config" element={<Config />} />
				</Routes>
			</MemoryRouter>
		);
	};

	describe('loading state', () => {
		it('shows loading state initially', async () => {
			vi.mocked(api.getConfig).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve(mockConfig), 100)
					)
			);

			renderConfig();
			expect(screen.getByText('Loading configuration...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			vi.mocked(api.getConfig).mockRejectedValue(new Error('Failed to load config'));

			renderConfig();

			await waitFor(() => {
				expect(screen.getByText('Failed to load config')).toBeInTheDocument();
			});
		});
	});

	describe('header', () => {
		it('displays page title', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByText('Orc Configuration')).toBeInTheDocument();
			});
		});

		it('displays subtitle', async () => {
			renderConfig();

			await waitFor(() => {
				expect(
					screen.getByText(/manage orchestrator settings in .orc\/config.yaml/i)
				).toBeInTheDocument();
			});
		});

		it('shows Save button', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});
		});
	});

	describe('automation profile section', () => {
		it('displays section title', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByText('Automation Profile')).toBeInTheDocument();
			});
		});

		it('shows all profile options', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByText('Auto')).toBeInTheDocument();
				expect(screen.getByText('Fast')).toBeInTheDocument();
				expect(screen.getByText('Safe')).toBeInTheDocument();
				expect(screen.getByText('Strict')).toBeInTheDocument();
			});
		});

		it('shows profile descriptions', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByText('Fully automated')).toBeInTheDocument();
				expect(screen.getByText('Minimal gates, speed over safety')).toBeInTheDocument();
				expect(screen.getByText('AI reviews, human for merge')).toBeInTheDocument();
				expect(screen.getByText('Human gates on spec/review/merge')).toBeInTheDocument();
			});
		});

		it('selects current profile from config', async () => {
			renderConfig();

			await waitFor(() => {
				// Auto profile has accessible name "Auto Fully automated"
				const autoRadio = screen.getByRole('radio', { name: /^auto/i });
				expect(autoRadio).toBeChecked();
			});
		});

		it('changes profile when clicked', async () => {
			renderConfig();

			await waitFor(() => {
				// Use exact match to avoid matching "Fast" which contains "safety"
				expect(screen.getByRole('radio', { name: /^safe/i })).toBeInTheDocument();
			});

			const safeRadio = screen.getByRole('radio', { name: /^safe/i });
			fireEvent.click(safeRadio);
			expect(safeRadio).toBeChecked();
		});
	});

	describe('gates & retry section', () => {
		it('displays section title', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByText('Gates & Retry')).toBeInTheDocument();
			});
		});

		it('shows default gates dropdown', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Default Gates')).toBeInTheDocument();
			});
		});

		it('shows max retries input', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Max Retries')).toHaveValue(3);
			});
		});

		it('shows retry enabled checkbox', async () => {
			renderConfig();

			await waitFor(() => {
				expect(
					screen.getByRole('checkbox', { name: /enable automatic retry on failure/i })
				).toBeChecked();
			});
		});
	});

	describe('execution section', () => {
		it('displays section title', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByText('Execution')).toBeInTheDocument();
			});
		});

		it('shows model input with value', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Model')).toHaveValue('claude-sonnet-4-20250514');
			});
		});

		it('shows max iterations input', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Max Iterations')).toHaveValue(10);
			});
		});

		it('shows timeout input', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Timeout')).toHaveValue('30m');
			});
		});
	});

	describe('git settings section', () => {
		it('displays section title', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByText('Git Settings')).toBeInTheDocument();
			});
		});

		it('shows branch prefix input', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Branch Prefix')).toHaveValue('orc/');
			});
		});

		it('shows commit prefix input', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Commit Prefix')).toHaveValue('[orc]');
			});
		});
	});

	describe('worktree settings section', () => {
		it('displays section title', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByText('Worktree Settings')).toBeInTheDocument();
			});
		});

		it('shows worktree enabled checkbox', async () => {
			renderConfig();

			await waitFor(() => {
				expect(
					screen.getByRole('checkbox', { name: /enable git worktrees for task isolation/i })
				).toBeChecked();
			});
		});

		it('shows worktree directory input', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Worktree Directory')).toHaveValue('.orc/worktrees');
			});
		});

		it('shows cleanup on complete checkbox', async () => {
			renderConfig();

			await waitFor(() => {
				expect(
					screen.getByRole('checkbox', { name: /cleanup worktree on completion/i })
				).toBeChecked();
			});
		});

		it('shows cleanup on fail checkbox', async () => {
			renderConfig();

			await waitFor(() => {
				expect(
					screen.getByRole('checkbox', { name: /cleanup worktree on failure/i })
				).not.toBeChecked();
			});
		});

		it('disables worktree fields when worktree disabled', async () => {
			vi.mocked(api.getConfig).mockResolvedValue({
				...mockConfig,
				worktree: { ...mockConfig.worktree!, enabled: false },
			});

			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Worktree Directory')).toBeDisabled();
			});
		});
	});

	describe('completion settings section', () => {
		it('displays section title', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByText('Completion Settings')).toBeInTheDocument();
			});
		});

		it('shows completion action dropdown', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Completion Action')).toHaveValue('pr');
			});
		});

		it('shows target branch input', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Target Branch')).toHaveValue('main');
			});
		});

		it('shows delete branch checkbox', async () => {
			renderConfig();

			await waitFor(() => {
				expect(
					screen.getByRole('checkbox', { name: /delete branch after merge/i })
				).toBeChecked();
			});
		});
	});

	describe('timeouts section', () => {
		it('displays section title', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByText('Timeouts')).toBeInTheDocument();
			});
		});

		it('shows phase max input', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Phase Max')).toHaveValue('30m');
			});
		});

		it('shows turn max input', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Turn Max')).toHaveValue('5m');
			});
		});

		it('shows idle warning input', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Idle Warning')).toHaveValue('2m');
			});
		});

		it('shows heartbeat interval input', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Heartbeat Interval')).toHaveValue('10s');
			});
		});

		it('shows idle timeout input', async () => {
			renderConfig();

			await waitFor(() => {
				expect(screen.getByLabelText('Idle Timeout')).toHaveValue('5m');
			});
		});
	});

	describe('saving configuration', () => {
		it('calls updateConfig with form data', async () => {
			vi.mocked(api.updateConfig).mockResolvedValue({});

			renderConfig();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(api.updateConfig).toHaveBeenCalledWith(
					expect.objectContaining({
						profile: 'auto',
						automation: expect.objectContaining({
							retry_enabled: true,
							retry_max: 3,
						}),
						git: expect.objectContaining({
							branch_prefix: 'orc/',
						}),
					})
				);
			});
		});

		it('shows Saving... text while saving', async () => {
			vi.mocked(api.updateConfig).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve({}), 100)
					)
			);

			renderConfig();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			expect(screen.getByText('Saving...')).toBeInTheDocument();
		});

		it('shows success message after save', async () => {
			vi.mocked(api.updateConfig).mockResolvedValue({});

			renderConfig();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(screen.getByText('Configuration saved successfully')).toBeInTheDocument();
			});
		});

		it('shows error message when save fails', async () => {
			vi.mocked(api.updateConfig).mockRejectedValue(new Error('Save failed'));

			renderConfig();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(screen.getByText('Save failed')).toBeInTheDocument();
			});
		});

		it('reloads config after successful save', async () => {
			vi.mocked(api.updateConfig).mockResolvedValue({});

			renderConfig();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				// getConfig called once on load and once after save
				expect(api.getConfig).toHaveBeenCalledTimes(2);
			});
		});
	});
});
