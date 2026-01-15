import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { Mcp } from './Mcp';
import * as api from '@/lib/api';

// Mock the API functions
vi.mock('@/lib/api', () => ({
	listMCPServers: vi.fn(),
	getMCPServer: vi.fn(),
	createMCPServer: vi.fn(),
	updateMCPServer: vi.fn(),
	deleteMCPServer: vi.fn(),
}));

describe('Mcp', () => {
	const mockServersList: api.MCPServerInfo[] = [
		{ name: 'filesystem', type: 'stdio', disabled: false },
		{ name: 'remote-api', type: 'sse', disabled: true },
	];

	const mockServer: api.MCPServer = {
		name: 'filesystem',
		type: 'stdio',
		command: 'npx',
		args: ['-y', '@modelcontextprotocol/server-filesystem'],
		disabled: false,
		env: { HOME: '/home/user' },
	};

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.listMCPServers).mockResolvedValue(mockServersList);
		vi.mocked(api.getMCPServer).mockResolvedValue(mockServer);
	});

	const renderMcp = (initialPath: string = '/environment/mcp') => {
		return render(
			<MemoryRouter initialEntries={[initialPath]}>
				<Routes>
					<Route path="/environment/mcp" element={<Mcp />} />
				</Routes>
			</MemoryRouter>
		);
	};

	describe('loading state', () => {
		it('shows loading state initially', async () => {
			vi.mocked(api.listMCPServers).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve(mockServersList), 100)
					)
			);

			renderMcp();
			expect(screen.getByText('Loading MCP servers...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			vi.mocked(api.listMCPServers).mockRejectedValue(
				new Error('Failed to load')
			);

			renderMcp();

			await waitFor(() => {
				expect(screen.getByText('Failed to load')).toBeInTheDocument();
			});
		});
	});

	describe('header', () => {
		it('displays page title', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByText('MCP Servers')).toBeInTheDocument();
			});
		});

		it('displays global title when scope is global', async () => {
			renderMcp('/environment/mcp?scope=global');

			await waitFor(() => {
				expect(screen.getByText('Global MCP Servers')).toBeInTheDocument();
			});
		});

		it('displays subtitle', async () => {
			renderMcp();

			await waitFor(() => {
				expect(
					screen.getByText('Configure Model Context Protocol servers')
				).toBeInTheDocument();
			});
		});

		it('shows Add Server button', async () => {
			renderMcp();

			await waitFor(() => {
				expect(
					screen.getByRole('button', { name: 'Add Server' })
				).toBeInTheDocument();
			});
		});
	});

	describe('scope toggle', () => {
		it('shows Project and Global links', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByRole('link', { name: 'Project' })).toBeInTheDocument();
				expect(screen.getByRole('link', { name: 'Global' })).toBeInTheDocument();
			});
		});

		it('highlights Project by default', async () => {
			renderMcp();

			await waitFor(() => {
				const projectLink = screen.getByRole('link', { name: 'Project' });
				expect(projectLink).toHaveClass('active');
			});
		});

		it('highlights Global when scope=global', async () => {
			renderMcp('/environment/mcp?scope=global');

			await waitFor(() => {
				const globalLink = screen.getByRole('link', { name: 'Global' });
				expect(globalLink).toHaveClass('active');
			});
		});
	});

	describe('server list', () => {
		it('displays list of servers', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByText('filesystem')).toBeInTheDocument();
				expect(screen.getByText('remote-api')).toBeInTheDocument();
			});
		});

		it('shows server types', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByText('stdio')).toBeInTheDocument();
				expect(screen.getByText('sse')).toBeInTheDocument();
			});
		});

		it('shows Disabled badge for disabled servers', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByText('Disabled')).toBeInTheDocument();
			});
		});

		it('shows empty message when no servers', async () => {
			vi.mocked(api.listMCPServers).mockResolvedValue([]);

			renderMcp();

			await waitFor(() => {
				expect(
					screen.getByText('No MCP servers configured')
				).toBeInTheDocument();
			});
		});
	});

	describe('selecting a server', () => {
		it('loads server details when clicked', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByText('filesystem')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('filesystem'));

			await waitFor(() => {
				expect(api.getMCPServer).toHaveBeenCalledWith('filesystem');
			});
		});

		it('shows server form with populated fields', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByText('filesystem')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('filesystem'));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toHaveValue('filesystem');
				expect(screen.getByLabelText('Command')).toHaveValue('npx');
				expect(screen.getByLabelText('Arguments')).toHaveValue(
					'-y @modelcontextprotocol/server-filesystem'
				);
			});
		});

		it('shows Delete button for selected server', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByText('filesystem')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('filesystem'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});
		});

		it('disables name field for existing server', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByText('filesystem')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('filesystem'));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toBeDisabled();
			});
		});

		it('shows env vars from server', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByText('filesystem')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('filesystem'));

			await waitFor(() => {
				expect(screen.getByDisplayValue('HOME')).toBeInTheDocument();
				expect(screen.getByDisplayValue('/home/user')).toBeInTheDocument();
			});
		});
	});

	describe('creating a server', () => {
		it('shows empty form when Add Server clicked', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Add Server' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Add Server' }));

			await waitFor(() => {
				expect(
					screen.getByRole('heading', { name: 'New MCP Server' })
				).toBeInTheDocument();
				expect(screen.getByLabelText('Name')).toHaveValue('');
			});
		});

		it('enables name field for new server', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Add Server' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Add Server' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).not.toBeDisabled();
			});
		});

		it('shows Create button for new server', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Add Server' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Add Server' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument();
			});
		});

		it('calls createMCPServer with form data', async () => {
			vi.mocked(api.createMCPServer).mockResolvedValue({});
			vi.mocked(api.getMCPServer).mockResolvedValue({
				name: 'new-server',
				type: 'stdio',
				command: 'node',
				disabled: false,
			});

			renderMcp();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Add Server' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Add Server' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Name'), {
				target: { value: 'new-server' },
			});
			fireEvent.change(screen.getByLabelText('Command'), {
				target: { value: 'node' },
			});
			fireEvent.click(screen.getByRole('button', { name: 'Create' }));

			await waitFor(() => {
				expect(api.createMCPServer).toHaveBeenCalledWith(
					expect.objectContaining({
						name: 'new-server',
						type: 'stdio',
						command: 'node',
					})
				);
			});
		});

		it('shows validation error when name is empty', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Add Server' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Add Server' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Create' }));

			await waitFor(() => {
				expect(screen.getByText('Name is required')).toBeInTheDocument();
			});
		});

		it('shows success message after create', async () => {
			vi.mocked(api.createMCPServer).mockResolvedValue({});
			vi.mocked(api.getMCPServer).mockResolvedValue({
				name: 'new-server',
				type: 'stdio',
				disabled: false,
			});

			renderMcp();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Add Server' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Add Server' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Name'), {
				target: { value: 'new-server' },
			});
			fireEvent.click(screen.getByRole('button', { name: 'Create' }));

			await waitFor(() => {
				expect(screen.getByText('MCP server created')).toBeInTheDocument();
			});
		});
	});

	describe('server type', () => {
		it('shows stdio fields by default', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Add Server' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Add Server' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Command')).toBeInTheDocument();
				expect(screen.getByLabelText('Arguments')).toBeInTheDocument();
			});
		});

		it('shows URL field when SSE selected', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Add Server' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Add Server' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Type')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Type'), {
				target: { value: 'sse' },
			});

			await waitFor(() => {
				expect(screen.getByLabelText('URL')).toBeInTheDocument();
				expect(screen.queryByLabelText('Command')).not.toBeInTheDocument();
			});
		});
	});

	describe('environment variables', () => {
		it('shows Add Variable button', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Add Server' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Add Server' }));

			await waitFor(() => {
				expect(
					screen.getByRole('button', { name: '+ Add Variable' })
				).toBeInTheDocument();
			});
		});

		it('adds new env var row when Add Variable clicked', async () => {
			renderMcp();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Add Server' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Add Server' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: '+ Add Variable' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: '+ Add Variable' }));

			await waitFor(() => {
				const keyInputs = screen.getAllByPlaceholderText('KEY');
				expect(keyInputs.length).toBe(1);
			});
		});
	});

	describe('deleting a server', () => {
		it('calls deleteMCPServer when delete clicked and confirmed', async () => {
			vi.mocked(api.deleteMCPServer).mockResolvedValue({});
			vi.stubGlobal('confirm', vi.fn(() => true));

			renderMcp();

			await waitFor(() => {
				expect(screen.getByText('filesystem')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('filesystem'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

			await waitFor(() => {
				expect(api.deleteMCPServer).toHaveBeenCalledWith('filesystem');
			});

			vi.unstubAllGlobals();
		});

		it('does not delete when confirm cancelled', async () => {
			vi.stubGlobal('confirm', vi.fn(() => false));

			renderMcp();

			await waitFor(() => {
				expect(screen.getByText('filesystem')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('filesystem'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

			expect(api.deleteMCPServer).not.toHaveBeenCalled();

			vi.unstubAllGlobals();
		});

		it('shows success message after delete', async () => {
			vi.mocked(api.deleteMCPServer).mockResolvedValue({});
			vi.stubGlobal('confirm', vi.fn(() => true));

			renderMcp();

			await waitFor(() => {
				expect(screen.getByText('filesystem')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('filesystem'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

			await waitFor(() => {
				expect(screen.getByText('MCP server deleted')).toBeInTheDocument();
			});

			vi.unstubAllGlobals();
		});
	});

	describe('no selection state', () => {
		it('shows help message when no server selected', async () => {
			renderMcp();

			await waitFor(() => {
				expect(
					screen.getByText('Select an MCP server from the list or add a new one.')
				).toBeInTheDocument();
			});
		});
	});
});
