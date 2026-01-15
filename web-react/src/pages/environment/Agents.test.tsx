import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { Agents } from './Agents';
import * as api from '@/lib/api';

// Mock the API functions
vi.mock('@/lib/api', () => ({
	listAgents: vi.fn(),
	getAgent: vi.fn(),
	createAgent: vi.fn(),
	updateAgent: vi.fn(),
	deleteAgent: vi.fn(),
}));

describe('Agents', () => {
	const mockAgentsList: api.SubAgent[] = [
		{ name: 'code-reviewer', description: 'Reviews code changes' },
		{ name: 'doc-writer', description: 'Writes documentation' },
	];

	const mockAgent: api.SubAgent = {
		name: 'code-reviewer',
		description: 'Reviews code changes',
		model: 'claude-sonnet-4-20250514',
		tools: ['Read', 'Grep', 'Glob'],
	};

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.listAgents).mockResolvedValue(mockAgentsList);
		vi.mocked(api.getAgent).mockResolvedValue(mockAgent);
	});

	const renderAgents = (initialPath: string = '/environment/agents') => {
		return render(
			<MemoryRouter initialEntries={[initialPath]}>
				<Routes>
					<Route path="/environment/agents" element={<Agents />} />
				</Routes>
			</MemoryRouter>
		);
	};

	describe('loading state', () => {
		it('shows loading state initially', async () => {
			vi.mocked(api.listAgents).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve(mockAgentsList), 100)
					)
			);

			renderAgents();
			expect(screen.getByText('Loading agents...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			vi.mocked(api.listAgents).mockRejectedValue(new Error('Failed to load'));

			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('Failed to load')).toBeInTheDocument();
			});
		});
	});

	describe('header', () => {
		it('displays page title', async () => {
			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('Sub-Agents')).toBeInTheDocument();
			});
		});

		it('displays global title when scope is global', async () => {
			renderAgents('/environment/agents?scope=global');

			await waitFor(() => {
				expect(screen.getByText('Global Sub-Agents')).toBeInTheDocument();
			});
		});

		it('displays subtitle', async () => {
			renderAgents();

			await waitFor(() => {
				expect(
					screen.getByText('Configure Claude Code sub-agents')
				).toBeInTheDocument();
			});
		});

		it('shows New Agent button', async () => {
			renderAgents();

			await waitFor(() => {
				expect(
					screen.getByRole('button', { name: 'New Agent' })
				).toBeInTheDocument();
			});
		});
	});

	describe('scope toggle', () => {
		it('shows Project and Global links', async () => {
			renderAgents();

			await waitFor(() => {
				expect(screen.getByRole('link', { name: 'Project' })).toBeInTheDocument();
				expect(screen.getByRole('link', { name: 'Global' })).toBeInTheDocument();
			});
		});

		it('highlights Project by default', async () => {
			renderAgents();

			await waitFor(() => {
				const projectLink = screen.getByRole('link', { name: 'Project' });
				expect(projectLink).toHaveClass('active');
			});
		});

		it('highlights Global when scope=global', async () => {
			renderAgents('/environment/agents?scope=global');

			await waitFor(() => {
				const globalLink = screen.getByRole('link', { name: 'Global' });
				expect(globalLink).toHaveClass('active');
			});
		});
	});

	describe('agent list', () => {
		it('displays list of agents', async () => {
			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('code-reviewer')).toBeInTheDocument();
				expect(screen.getByText('doc-writer')).toBeInTheDocument();
			});
		});

		it('shows agent descriptions', async () => {
			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('Reviews code changes')).toBeInTheDocument();
				expect(screen.getByText('Writes documentation')).toBeInTheDocument();
			});
		});

		it('shows empty message when no agents', async () => {
			vi.mocked(api.listAgents).mockResolvedValue([]);

			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('No agents configured')).toBeInTheDocument();
			});
		});
	});

	describe('selecting an agent', () => {
		it('loads agent details when clicked', async () => {
			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('code-reviewer')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('code-reviewer'));

			await waitFor(() => {
				expect(api.getAgent).toHaveBeenCalledWith('code-reviewer');
			});
		});

		it('shows agent form with populated fields', async () => {
			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('code-reviewer')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('code-reviewer'));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toHaveValue('code-reviewer');
				expect(screen.getByLabelText('Description')).toHaveValue(
					'Reviews code changes'
				);
				expect(screen.getByLabelText('Model (optional)')).toHaveValue(
					'claude-sonnet-4-20250514'
				);
				expect(screen.getByLabelText('Tools (optional)')).toHaveValue(
					'Read, Grep, Glob'
				);
			});
		});

		it('shows Delete button for selected agent', async () => {
			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('code-reviewer')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('code-reviewer'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});
		});

		it('disables name field for existing agent', async () => {
			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('code-reviewer')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('code-reviewer'));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toBeDisabled();
			});
		});
	});

	describe('creating an agent', () => {
		it('shows empty form when New Agent clicked', async () => {
			renderAgents();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Agent' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Agent' }));

			await waitFor(() => {
				expect(
					screen.getByRole('heading', { name: 'New Agent' })
				).toBeInTheDocument();
				expect(screen.getByLabelText('Name')).toHaveValue('');
			});
		});

		it('enables name field for new agent', async () => {
			renderAgents();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Agent' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Agent' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).not.toBeDisabled();
			});
		});

		it('shows Create button for new agent', async () => {
			renderAgents();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Agent' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Agent' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument();
			});
		});

		it('calls createAgent with form data', async () => {
			vi.mocked(api.createAgent).mockResolvedValue({});
			vi.mocked(api.getAgent).mockResolvedValue({
				name: 'new-agent',
				description: 'A new agent',
			});

			renderAgents();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Agent' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Agent' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Name'), {
				target: { value: 'new-agent' },
			});
			fireEvent.change(screen.getByLabelText('Description'), {
				target: { value: 'A new agent' },
			});
			fireEvent.click(screen.getByRole('button', { name: 'Create' }));

			await waitFor(() => {
				expect(api.createAgent).toHaveBeenCalledWith(
					expect.objectContaining({
						name: 'new-agent',
						description: 'A new agent',
					})
				);
			});
		});

		it('shows validation error when name is empty', async () => {
			renderAgents();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Agent' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Agent' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Create' }));

			await waitFor(() => {
				expect(screen.getByText('Name is required')).toBeInTheDocument();
			});
		});

		it('shows success message after create', async () => {
			vi.mocked(api.createAgent).mockResolvedValue({});
			vi.mocked(api.getAgent).mockResolvedValue({
				name: 'new-agent',
			});

			renderAgents();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Agent' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Agent' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Name'), {
				target: { value: 'new-agent' },
			});
			fireEvent.click(screen.getByRole('button', { name: 'Create' }));

			await waitFor(() => {
				expect(screen.getByText('Agent created')).toBeInTheDocument();
			});
		});
	});

	describe('updating an agent', () => {
		it('shows Update button for existing agent', async () => {
			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('code-reviewer')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('code-reviewer'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Update' })).toBeInTheDocument();
			});
		});

		it('calls updateAgent with form data', async () => {
			vi.mocked(api.updateAgent).mockResolvedValue({});

			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('code-reviewer')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('code-reviewer'));

			await waitFor(() => {
				expect(screen.getByLabelText('Description')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Description'), {
				target: { value: 'Updated description' },
			});
			fireEvent.click(screen.getByRole('button', { name: 'Update' }));

			await waitFor(() => {
				expect(api.updateAgent).toHaveBeenCalledWith(
					'code-reviewer',
					expect.objectContaining({ description: 'Updated description' })
				);
			});
		});

		it('shows success message after update', async () => {
			vi.mocked(api.updateAgent).mockResolvedValue({});

			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('code-reviewer')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('code-reviewer'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Update' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Update' }));

			await waitFor(() => {
				expect(screen.getByText('Agent updated')).toBeInTheDocument();
			});
		});
	});

	describe('deleting an agent', () => {
		it('calls deleteAgent when delete clicked and confirmed', async () => {
			vi.mocked(api.deleteAgent).mockResolvedValue({});
			vi.stubGlobal('confirm', vi.fn(() => true));

			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('code-reviewer')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('code-reviewer'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

			await waitFor(() => {
				expect(api.deleteAgent).toHaveBeenCalledWith('code-reviewer');
			});

			vi.unstubAllGlobals();
		});

		it('does not delete when confirm cancelled', async () => {
			vi.stubGlobal('confirm', vi.fn(() => false));

			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('code-reviewer')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('code-reviewer'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

			expect(api.deleteAgent).not.toHaveBeenCalled();

			vi.unstubAllGlobals();
		});

		it('shows success message after delete', async () => {
			vi.mocked(api.deleteAgent).mockResolvedValue({});
			vi.stubGlobal('confirm', vi.fn(() => true));

			renderAgents();

			await waitFor(() => {
				expect(screen.getByText('code-reviewer')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('code-reviewer'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

			await waitFor(() => {
				expect(screen.getByText('Agent deleted')).toBeInTheDocument();
			});

			vi.unstubAllGlobals();
		});
	});

	describe('no selection state', () => {
		it('shows help message when no agent selected', async () => {
			renderAgents();

			await waitFor(() => {
				expect(
					screen.getByText('Select an agent from the list or create a new one.')
				).toBeInTheDocument();
			});
		});
	});
});
