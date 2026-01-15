import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { Preferences } from './Preferences';
import * as api from '@/lib/api';

// Mock the API functions
vi.mock('@/lib/api', () => ({
	getSettings: vi.fn(),
	getGlobalSettings: vi.fn(),
	getProjectSettings: vi.fn(),
	updateSettings: vi.fn(),
	updateGlobalSettings: vi.fn(),
}));

describe('Preferences', () => {
	const mockGlobalSettings: api.Settings = {
		permissions: {
			allow: ['Read', 'Write'],
			deny: ['Bash'],
		},
		env: {
			GLOBAL_VAR: 'global_value',
		},
	};

	const mockProjectSettings: api.Settings = {
		permissions: {
			allow: ['Bash'],
		},
		env: {
			PROJECT_VAR: 'project_value',
		},
	};

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.getGlobalSettings).mockResolvedValue(mockGlobalSettings);
		vi.mocked(api.getProjectSettings).mockResolvedValue(mockProjectSettings);
	});

	const renderPreferences = (initialPath: string = '/preferences') => {
		return render(
			<MemoryRouter initialEntries={[initialPath]}>
				<Routes>
					<Route path="/preferences" element={<Preferences />} />
				</Routes>
			</MemoryRouter>
		);
	};

	describe('loading state', () => {
		it('shows loading state initially', async () => {
			vi.mocked(api.getGlobalSettings).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve(mockGlobalSettings), 100)
					)
			);

			renderPreferences();
			expect(screen.getByText('Loading settings...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			vi.mocked(api.getGlobalSettings).mockRejectedValue(
				new Error('Failed to load settings')
			);

			renderPreferences();

			await waitFor(() => {
				expect(screen.getByText('Failed to load settings')).toBeInTheDocument();
			});
		});
	});

	describe('header', () => {
		it('displays page title', async () => {
			renderPreferences();

			await waitFor(() => {
				expect(screen.getByText('Preferences')).toBeInTheDocument();
			});
		});

		it('displays subtitle', async () => {
			renderPreferences();

			await waitFor(() => {
				expect(
					screen.getByText('Claude Code settings and environment')
				).toBeInTheDocument();
			});
		});
	});

	describe('tab navigation', () => {
		it('shows all three tabs', async () => {
			renderPreferences();

			await waitFor(() => {
				expect(screen.getByText('Global Settings')).toBeInTheDocument();
				expect(screen.getByText('Project Settings')).toBeInTheDocument();
				expect(screen.getByText('Environment Variables')).toBeInTheDocument();
			});
		});

		it('defaults to global settings tab', async () => {
			renderPreferences();

			await waitFor(() => {
				const globalTab = screen.getByText('Global Settings');
				expect(globalTab.closest('button')).toHaveClass('active');
			});
		});

		it('switches to project settings tab when clicked', async () => {
			renderPreferences();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Project Settings' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Project Settings' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Project Settings' })).toHaveClass('active');
			});
		});

		it('switches to env tab when clicked', async () => {
			renderPreferences();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Environment Variables' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Environment Variables' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Environment Variables' })).toHaveClass('active');
			});
		});

		it('respects tab URL parameter', async () => {
			renderPreferences('/preferences?tab=project');

			await waitFor(() => {
				const projectTab = screen.getByText('Project Settings');
				expect(projectTab.closest('button')).toHaveClass('active');
			});
		});
	});

	describe('global settings tab', () => {
		it('displays global settings description', async () => {
			renderPreferences();

			await waitFor(() => {
				expect(
					screen.getByText(/settings stored in ~\/.claude\/settings.json/i)
				).toBeInTheDocument();
			});
		});

		it('displays global settings JSON', async () => {
			renderPreferences();

			await waitFor(() => {
				expect(screen.getByText(/GLOBAL_VAR/)).toBeInTheDocument();
			});
		});

		it('shows hint about editing settings', async () => {
			renderPreferences();

			await waitFor(() => {
				expect(screen.getByText(/to edit global settings/i)).toBeInTheDocument();
			});
		});

		it('shows no settings message when null', async () => {
			vi.mocked(api.getGlobalSettings).mockResolvedValue(null as unknown as api.Settings);

			renderPreferences();

			await waitFor(() => {
				expect(
					screen.getByText('No global settings configured')
				).toBeInTheDocument();
			});
		});
	});

	describe('project settings tab', () => {
		it('displays project settings description', async () => {
			renderPreferences('/preferences?tab=project');

			await waitFor(() => {
				expect(
					screen.getByText(/settings stored in .claude\/settings.json/i)
				).toBeInTheDocument();
			});
		});

		it('displays project settings JSON', async () => {
			renderPreferences('/preferences?tab=project');

			await waitFor(() => {
				expect(screen.getByText(/PROJECT_VAR/)).toBeInTheDocument();
			});
		});
	});

	describe('environment variables tab', () => {
		it('displays env vars description', async () => {
			renderPreferences('/preferences?tab=env');

			await waitFor(() => {
				expect(
					screen.getByText(
						/environment variables available to claude code and hooks/i
					)
				).toBeInTheDocument();
			});
		});

		it('shows existing env vars from project settings', async () => {
			renderPreferences('/preferences?tab=env');

			await waitFor(() => {
				expect(screen.getByDisplayValue('PROJECT_VAR')).toBeInTheDocument();
				expect(screen.getByDisplayValue('project_value')).toBeInTheDocument();
			});
		});

		it('shows save button', async () => {
			renderPreferences('/preferences?tab=env');

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});
		});

		it('shows Add button for new env vars', async () => {
			renderPreferences('/preferences?tab=env');

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Add' })).toBeInTheDocument();
			});
		});

		it('adds new env var when Add clicked', async () => {
			renderPreferences('/preferences?tab=env');

			await waitFor(() => {
				expect(screen.getByPlaceholderText('NEW_KEY')).toBeInTheDocument();
			});

			const newKeyInput = screen.getByPlaceholderText('NEW_KEY');
			// Get all "value" placeholders - the last one is the new entry row input
			const valueInputs = screen.getAllByPlaceholderText('value');
			const newValueInput = valueInputs[valueInputs.length - 1];

			fireEvent.change(newKeyInput, { target: { value: 'NEW_VAR' } });
			fireEvent.change(newValueInput, { target: { value: 'new_value' } });
			fireEvent.click(screen.getByRole('button', { name: 'Add' }));

			await waitFor(() => {
				expect(screen.getByDisplayValue('NEW_VAR')).toBeInTheDocument();
				expect(screen.getByDisplayValue('new_value')).toBeInTheDocument();
			});
		});

		it('disables Add button when key is empty', async () => {
			renderPreferences('/preferences?tab=env');

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Add' })).toBeDisabled();
			});
		});

		it('removes env var when delete clicked', async () => {
			renderPreferences('/preferences?tab=env');

			await waitFor(() => {
				expect(screen.getAllByTitle('Remove').length).toBeGreaterThan(0);
			});

			const removeButtons = screen.getAllByTitle('Remove');
			fireEvent.click(removeButtons[0]);

			await waitFor(() => {
				expect(
					screen.queryByDisplayValue('PROJECT_VAR')
				).not.toBeInTheDocument();
			});
		});

		it('saves env vars when Save clicked', async () => {
			vi.mocked(api.updateSettings).mockResolvedValue({} as api.Settings);

			renderPreferences('/preferences?tab=env');

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(api.updateSettings).toHaveBeenCalled();
			});
		});

		it('shows success message after save', async () => {
			vi.mocked(api.updateSettings).mockResolvedValue({} as api.Settings);

			renderPreferences('/preferences?tab=env');

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(
					screen.getByText('Environment variables saved')
				).toBeInTheDocument();
			});
		});

		it('shows error message when save fails', async () => {
			vi.mocked(api.updateSettings).mockRejectedValue(
				new Error('Save failed')
			);

			renderPreferences('/preferences?tab=env');

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(screen.getByText('Save failed')).toBeInTheDocument();
			});
		});

		it('shows no env vars message when empty', async () => {
			vi.mocked(api.getProjectSettings).mockResolvedValue({ permissions: {} });
			vi.mocked(api.getGlobalSettings).mockResolvedValue({ permissions: {} });

			renderPreferences('/preferences?tab=env');

			await waitFor(() => {
				expect(
					screen.getByText('No environment variables configured')
				).toBeInTheDocument();
			});
		});
	});
});
