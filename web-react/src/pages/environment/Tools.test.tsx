import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { Tools } from './Tools';
import * as api from '@/lib/api';

// Mock the API functions
vi.mock('@/lib/api', () => ({
	listToolsByCategory: vi.fn(),
	getToolPermissions: vi.fn(),
	updateToolPermissions: vi.fn(),
}));

describe('Tools', () => {
	const mockToolsByCategory: api.ToolsByCategory = {
		'File System': [
			{ name: 'Read', description: 'Read files from disk' },
			{ name: 'Write', description: 'Write files to disk' },
			{ name: 'Edit', description: 'Edit files' },
		],
		'Code Execution': [
			{ name: 'Bash', description: 'Execute shell commands' },
		],
	};

	const mockPermissions: api.ToolPermissions = {
		Read: 'allow',
		Bash: 'deny',
	};

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.listToolsByCategory).mockResolvedValue(mockToolsByCategory);
		vi.mocked(api.getToolPermissions).mockResolvedValue(mockPermissions);
	});

	const renderTools = (initialPath: string = '/environment/tools') => {
		return render(
			<MemoryRouter initialEntries={[initialPath]}>
				<Routes>
					<Route path="/environment/tools" element={<Tools />} />
				</Routes>
			</MemoryRouter>
		);
	};

	describe('loading state', () => {
		it('shows loading state initially', async () => {
			vi.mocked(api.listToolsByCategory).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve(mockToolsByCategory), 100)
					)
			);

			renderTools();
			expect(screen.getByText('Loading tools...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			vi.mocked(api.listToolsByCategory).mockRejectedValue(
				new Error('Failed to load')
			);

			renderTools();

			await waitFor(() => {
				expect(screen.getByText('Failed to load')).toBeInTheDocument();
			});
		});
	});

	describe('header', () => {
		it('displays page title', async () => {
			renderTools();

			await waitFor(() => {
				expect(screen.getByText('Tool Permissions')).toBeInTheDocument();
			});
		});

		it('displays subtitle', async () => {
			renderTools();

			await waitFor(() => {
				expect(
					screen.getByText('Configure which tools Claude Code can use')
				).toBeInTheDocument();
			});
		});

		it('shows Save button', async () => {
			renderTools();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});
		});

		it('disables Save button when no changes', async () => {
			renderTools();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
			});
		});
	});

	describe('tool categories', () => {
		it('displays category headers', async () => {
			renderTools();

			await waitFor(() => {
				expect(screen.getByText('File System')).toBeInTheDocument();
				expect(screen.getByText('Code Execution')).toBeInTheDocument();
			});
		});

		it('displays tools in each category', async () => {
			renderTools();

			await waitFor(() => {
				expect(screen.getByText('Read')).toBeInTheDocument();
				expect(screen.getByText('Write')).toBeInTheDocument();
				expect(screen.getByText('Edit')).toBeInTheDocument();
				expect(screen.getByText('Bash')).toBeInTheDocument();
			});
		});

		it('displays tool descriptions', async () => {
			renderTools();

			await waitFor(() => {
				expect(screen.getByText('Read files from disk')).toBeInTheDocument();
				expect(screen.getByText('Execute shell commands')).toBeInTheDocument();
			});
		});

		it('shows empty message when no tools', async () => {
			vi.mocked(api.listToolsByCategory).mockResolvedValue({});

			renderTools();

			await waitFor(() => {
				expect(screen.getByText('No tools available')).toBeInTheDocument();
			});
		});
	});

	describe('permission toggles', () => {
		it('shows Allow and Deny buttons for each tool', async () => {
			renderTools();

			await waitFor(() => {
				const allowButtons = screen.getAllByRole('button', { name: 'Allow' });
				const denyButtons = screen.getAllByRole('button', { name: 'Deny' });
				expect(allowButtons.length).toBe(4); // 4 tools total
				expect(denyButtons.length).toBe(4);
			});
		});

		it('highlights Allow button when tool is allowed', async () => {
			renderTools();

			await waitFor(() => {
				const readCard = screen.getByText('Read').closest('.tool-card');
				const allowButton = readCard?.querySelector('.toggle-btn.allow');
				expect(allowButton).toHaveClass('active');
			});
		});

		it('highlights Deny button when tool is denied', async () => {
			renderTools();

			await waitFor(() => {
				const bashCard = screen.getByText('Bash').closest('.tool-card');
				const denyButton = bashCard?.querySelector('.toggle-btn.deny');
				expect(denyButton).toHaveClass('active');
			});
		});

		it('toggles permission when clicked', async () => {
			renderTools();

			await waitFor(() => {
				expect(screen.getByText('Write')).toBeInTheDocument();
			});

			const writeCard = screen.getByText('Write').closest('.tool-card');
			const allowButton = writeCard?.querySelector('.toggle-btn.allow') as HTMLElement;
			fireEvent.click(allowButton);

			await waitFor(() => {
				const updatedWriteCard = screen.getByText('Write').closest('.tool-card');
				const updatedAllowButton = updatedWriteCard?.querySelector('.toggle-btn.allow');
				expect(updatedAllowButton).toHaveClass('active');
			});
		});

		it('enables Save button when permission changed', async () => {
			renderTools();

			await waitFor(() => {
				expect(screen.getByText('Write')).toBeInTheDocument();
			});

			const writeCard = screen.getByText('Write').closest('.tool-card');
			const allowButton = writeCard?.querySelector('.toggle-btn.allow') as HTMLElement;
			fireEvent.click(allowButton);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).not.toBeDisabled();
			});
		});

		it('shows Reset button when changes made', async () => {
			renderTools();

			await waitFor(() => {
				expect(screen.queryByRole('button', { name: 'Reset' })).not.toBeInTheDocument();
			});

			await waitFor(() => {
				expect(screen.getByText('Write')).toBeInTheDocument();
			});

			const writeCard = screen.getByText('Write').closest('.tool-card');
			const allowButton = writeCard?.querySelector('.toggle-btn.allow') as HTMLElement;
			fireEvent.click(allowButton);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Reset' })).toBeInTheDocument();
			});
		});

		it('resets permissions when Reset clicked', async () => {
			renderTools();

			await waitFor(() => {
				expect(screen.getByText('Write')).toBeInTheDocument();
			});

			const writeCard = screen.getByText('Write').closest('.tool-card');
			const allowButton = writeCard?.querySelector('.toggle-btn.allow') as HTMLElement;
			fireEvent.click(allowButton);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Reset' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Reset' }));

			await waitFor(() => {
				expect(screen.queryByRole('button', { name: 'Reset' })).not.toBeInTheDocument();
				expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
			});
		});

		it('toggles off when clicking same permission again', async () => {
			renderTools();

			await waitFor(() => {
				expect(screen.getByText('Read')).toBeInTheDocument();
			});

			const readCard = screen.getByText('Read').closest('.tool-card');
			const allowButton = readCard?.querySelector('.toggle-btn.allow') as HTMLElement;
			// Read is already allowed, clicking again should toggle off
			fireEvent.click(allowButton);

			await waitFor(() => {
				const updatedReadCard = screen.getByText('Read').closest('.tool-card');
				const updatedAllowButton = updatedReadCard?.querySelector('.toggle-btn.allow');
				expect(updatedAllowButton).not.toHaveClass('active');
			});
		});
	});

	describe('saving permissions', () => {
		it('calls updateToolPermissions with current permissions', async () => {
			vi.mocked(api.updateToolPermissions).mockResolvedValue({});

			renderTools();

			await waitFor(() => {
				expect(screen.getByText('Write')).toBeInTheDocument();
			});

			const writeCard = screen.getByText('Write').closest('.tool-card');
			const allowButton = writeCard?.querySelector('.toggle-btn.allow') as HTMLElement;
			fireEvent.click(allowButton);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).not.toBeDisabled();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(api.updateToolPermissions).toHaveBeenCalledWith(
					expect.objectContaining({
						Read: 'allow',
						Bash: 'deny',
						Write: 'allow',
					})
				);
			});
		});

		it('shows Saving... text while saving', async () => {
			vi.mocked(api.updateToolPermissions).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve({}), 100)
					)
			);

			renderTools();

			await waitFor(() => {
				expect(screen.getByText('Write')).toBeInTheDocument();
			});

			const writeCard = screen.getByText('Write').closest('.tool-card');
			const allowButton = writeCard?.querySelector('.toggle-btn.allow') as HTMLElement;
			fireEvent.click(allowButton);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).not.toBeDisabled();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			expect(screen.getByText('Saving...')).toBeInTheDocument();
		});

		it('shows success message after save', async () => {
			vi.mocked(api.updateToolPermissions).mockResolvedValue({});

			renderTools();

			await waitFor(() => {
				expect(screen.getByText('Write')).toBeInTheDocument();
			});

			const writeCard = screen.getByText('Write').closest('.tool-card');
			const allowButton = writeCard?.querySelector('.toggle-btn.allow') as HTMLElement;
			fireEvent.click(allowButton);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).not.toBeDisabled();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(screen.getByText('Permissions saved')).toBeInTheDocument();
			});
		});

		it('shows error message when save fails', async () => {
			vi.mocked(api.updateToolPermissions).mockRejectedValue(
				new Error('Save failed')
			);

			renderTools();

			await waitFor(() => {
				expect(screen.getByText('Write')).toBeInTheDocument();
			});

			const writeCard = screen.getByText('Write').closest('.tool-card');
			const allowButton = writeCard?.querySelector('.toggle-btn.allow') as HTMLElement;
			fireEvent.click(allowButton);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).not.toBeDisabled();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(screen.getByText('Save failed')).toBeInTheDocument();
			});
		});

		it('hides Reset button and disables Save after successful save', async () => {
			vi.mocked(api.updateToolPermissions).mockResolvedValue({});

			renderTools();

			await waitFor(() => {
				expect(screen.getByText('Write')).toBeInTheDocument();
			});

			const writeCard = screen.getByText('Write').closest('.tool-card');
			const allowButton = writeCard?.querySelector('.toggle-btn.allow') as HTMLElement;
			fireEvent.click(allowButton);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).not.toBeDisabled();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(screen.queryByRole('button', { name: 'Reset' })).not.toBeInTheDocument();
				expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
			});
		});
	});
});
