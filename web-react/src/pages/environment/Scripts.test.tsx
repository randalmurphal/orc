import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { Scripts } from './Scripts';
import * as api from '@/lib/api';

// Mock the API functions
vi.mock('@/lib/api', () => ({
	listScripts: vi.fn(),
	getScript: vi.fn(),
	createScript: vi.fn(),
	updateScript: vi.fn(),
	deleteScript: vi.fn(),
	discoverScripts: vi.fn(),
}));

describe('Scripts', () => {
	const mockScriptsList: api.ProjectScript[] = [
		{ name: 'build', description: 'Build the project', command: 'npm run build' },
		{ name: 'test', description: 'Run tests', path: './scripts/test.sh' },
	];

	const mockScript: api.ProjectScript = {
		name: 'build',
		description: 'Build the project',
		command: 'npm',
		args: ['run', 'build'],
	};

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.listScripts).mockResolvedValue(mockScriptsList);
		vi.mocked(api.getScript).mockResolvedValue(mockScript);
	});

	const renderScripts = (initialPath: string = '/environment/scripts') => {
		return render(
			<MemoryRouter initialEntries={[initialPath]}>
				<Routes>
					<Route path="/environment/scripts" element={<Scripts />} />
				</Routes>
			</MemoryRouter>
		);
	};

	describe('loading state', () => {
		it('shows loading state initially', async () => {
			vi.mocked(api.listScripts).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve(mockScriptsList), 100)
					)
			);

			renderScripts();
			expect(screen.getByText('Loading scripts...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			vi.mocked(api.listScripts).mockRejectedValue(new Error('Failed to load'));

			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('Failed to load')).toBeInTheDocument();
			});
		});
	});

	describe('header', () => {
		it('displays page title', async () => {
			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('Project Scripts')).toBeInTheDocument();
			});
		});

		it('displays subtitle', async () => {
			renderScripts();

			await waitFor(() => {
				expect(
					screen.getByText('Configure scripts for task execution')
				).toBeInTheDocument();
			});
		});

		it('shows Discover Scripts button', async () => {
			renderScripts();

			await waitFor(() => {
				expect(
					screen.getByRole('button', { name: 'Discover Scripts' })
				).toBeInTheDocument();
			});
		});

		it('shows New Script button', async () => {
			renderScripts();

			await waitFor(() => {
				expect(
					screen.getByRole('button', { name: 'New Script' })
				).toBeInTheDocument();
			});
		});
	});

	describe('script list', () => {
		it('displays list of scripts', async () => {
			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('build')).toBeInTheDocument();
				expect(screen.getByText('test')).toBeInTheDocument();
			});
		});

		it('shows script descriptions', async () => {
			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('Build the project')).toBeInTheDocument();
				expect(screen.getByText('Run tests')).toBeInTheDocument();
			});
		});

		it('shows empty message with discover hint when no scripts', async () => {
			vi.mocked(api.listScripts).mockResolvedValue([]);

			renderScripts();

			await waitFor(() => {
				expect(
					screen.getByText(
						'No scripts configured. Click "Discover Scripts" to find executables.'
					)
				).toBeInTheDocument();
			});
		});
	});

	describe('discover scripts', () => {
		it('calls discoverScripts when button clicked', async () => {
			vi.mocked(api.discoverScripts).mockResolvedValue([
				{ name: 'lint', path: './scripts/lint.sh' },
			]);

			renderScripts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Discover Scripts' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Discover Scripts' }));

			await waitFor(() => {
				expect(api.discoverScripts).toHaveBeenCalled();
			});
		});

		it('shows Discovering... text while discovering', async () => {
			vi.mocked(api.discoverScripts).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve([]), 100)
					)
			);

			renderScripts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Discover Scripts' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Discover Scripts' }));

			expect(screen.getByText('Discovering...')).toBeInTheDocument();
		});

		it('shows success message with count after discovery', async () => {
			vi.mocked(api.discoverScripts).mockResolvedValue([
				{ name: 'lint' },
				{ name: 'format' },
			]);

			renderScripts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Discover Scripts' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Discover Scripts' }));

			await waitFor(() => {
				expect(screen.getByText('Discovered 2 scripts')).toBeInTheDocument();
			});
		});

		it('shows error message when discovery fails', async () => {
			vi.mocked(api.discoverScripts).mockRejectedValue(
				new Error('Discovery failed')
			);

			renderScripts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Discover Scripts' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Discover Scripts' }));

			await waitFor(() => {
				expect(screen.getByText('Discovery failed')).toBeInTheDocument();
			});
		});
	});

	describe('selecting a script', () => {
		it('loads script details when clicked', async () => {
			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('build')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('build'));

			await waitFor(() => {
				expect(api.getScript).toHaveBeenCalledWith('build');
			});
		});

		it('shows script form with populated fields', async () => {
			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('build')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('build'));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toHaveValue('build');
				expect(screen.getByLabelText('Description (optional)')).toHaveValue(
					'Build the project'
				);
				expect(screen.getByLabelText('Command (alternative to path)')).toHaveValue(
					'npm'
				);
				expect(screen.getByLabelText('Arguments (optional)')).toHaveValue(
					'run build'
				);
			});
		});

		it('shows Delete button for selected script', async () => {
			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('build')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('build'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});
		});

		it('disables name field for existing script', async () => {
			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('build')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('build'));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toBeDisabled();
			});
		});
	});

	describe('creating a script', () => {
		it('shows empty form when New Script clicked', async () => {
			renderScripts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Script' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Script' }));

			await waitFor(() => {
				expect(
					screen.getByRole('heading', { name: 'New Script' })
				).toBeInTheDocument();
				expect(screen.getByLabelText('Name')).toHaveValue('');
			});
		});

		it('enables name field for new script', async () => {
			renderScripts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Script' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Script' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).not.toBeDisabled();
			});
		});

		it('shows Create button for new script', async () => {
			renderScripts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Script' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Script' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument();
			});
		});

		it('calls createScript with form data', async () => {
			vi.mocked(api.createScript).mockResolvedValue({});
			vi.mocked(api.getScript).mockResolvedValue({
				name: 'lint',
				command: 'npm run lint',
			});

			renderScripts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Script' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Script' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Name'), {
				target: { value: 'lint' },
			});
			fireEvent.change(
				screen.getByLabelText('Command (alternative to path)'),
				{ target: { value: 'npm run lint' } }
			);
			fireEvent.click(screen.getByRole('button', { name: 'Create' }));

			await waitFor(() => {
				expect(api.createScript).toHaveBeenCalledWith(
					expect.objectContaining({
						name: 'lint',
						command: 'npm run lint',
					})
				);
			});
		});

		it('shows validation error when name is empty', async () => {
			renderScripts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Script' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Script' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Create' }));

			await waitFor(() => {
				expect(screen.getByText('Name is required')).toBeInTheDocument();
			});
		});

		it('shows validation error when neither path nor command provided', async () => {
			renderScripts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Script' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Script' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Name'), {
				target: { value: 'lint' },
			});
			fireEvent.click(screen.getByRole('button', { name: 'Create' }));

			await waitFor(() => {
				expect(
					screen.getByText('Either path or command is required')
				).toBeInTheDocument();
			});
		});

		it('shows success message after create', async () => {
			vi.mocked(api.createScript).mockResolvedValue({});
			vi.mocked(api.getScript).mockResolvedValue({
				name: 'lint',
				command: 'npm run lint',
			});

			renderScripts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Script' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Script' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Name'), {
				target: { value: 'lint' },
			});
			fireEvent.change(
				screen.getByLabelText('Command (alternative to path)'),
				{ target: { value: 'npm run lint' } }
			);
			fireEvent.click(screen.getByRole('button', { name: 'Create' }));

			await waitFor(() => {
				expect(screen.getByText('Script created')).toBeInTheDocument();
			});
		});
	});

	describe('updating a script', () => {
		it('shows Update button for existing script', async () => {
			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('build')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('build'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Update' })).toBeInTheDocument();
			});
		});

		it('calls updateScript with form data', async () => {
			vi.mocked(api.updateScript).mockResolvedValue({});

			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('build')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('build'));

			await waitFor(() => {
				expect(screen.getByLabelText('Description (optional)')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Description (optional)'), {
				target: { value: 'Updated description' },
			});
			fireEvent.click(screen.getByRole('button', { name: 'Update' }));

			await waitFor(() => {
				expect(api.updateScript).toHaveBeenCalledWith(
					'build',
					expect.objectContaining({ description: 'Updated description' })
				);
			});
		});

		it('shows success message after update', async () => {
			vi.mocked(api.updateScript).mockResolvedValue({});

			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('build')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('build'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Update' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Update' }));

			await waitFor(() => {
				expect(screen.getByText('Script updated')).toBeInTheDocument();
			});
		});
	});

	describe('deleting a script', () => {
		it('calls deleteScript when delete clicked and confirmed', async () => {
			vi.mocked(api.deleteScript).mockResolvedValue({});
			vi.stubGlobal('confirm', vi.fn(() => true));

			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('build')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('build'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

			await waitFor(() => {
				expect(api.deleteScript).toHaveBeenCalledWith('build');
			});

			vi.unstubAllGlobals();
		});

		it('does not delete when confirm cancelled', async () => {
			vi.stubGlobal('confirm', vi.fn(() => false));

			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('build')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('build'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

			expect(api.deleteScript).not.toHaveBeenCalled();

			vi.unstubAllGlobals();
		});

		it('shows success message after delete', async () => {
			vi.mocked(api.deleteScript).mockResolvedValue({});
			vi.stubGlobal('confirm', vi.fn(() => true));

			renderScripts();

			await waitFor(() => {
				expect(screen.getByText('build')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('build'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

			await waitFor(() => {
				expect(screen.getByText('Script deleted')).toBeInTheDocument();
			});

			vi.unstubAllGlobals();
		});
	});

	describe('no selection state', () => {
		it('shows help message when no script selected', async () => {
			renderScripts();

			await waitFor(() => {
				expect(
					screen.getByText('Select a script from the list or create a new one.')
				).toBeInTheDocument();
			});
		});
	});
});
