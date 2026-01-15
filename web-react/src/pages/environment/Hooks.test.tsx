import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { Hooks } from './Hooks';
import * as api from '@/lib/api';

// Mock the API functions
vi.mock('@/lib/api', () => ({
	listHooks: vi.fn(),
	getHookTypes: vi.fn(),
	updateHook: vi.fn(),
	deleteHook: vi.fn(),
}));

describe('Hooks', () => {
	const mockHookTypes: api.HookEvent[] = ['PreToolUse', 'PostToolUse', 'PreCompact', 'PrePrompt', 'Stop'];

	const mockHooks: api.HooksMap = {
		PreToolUse: [
			{
				matcher: 'Bash',
				hooks: [{ type: 'command', command: 'echo "Pre bash"' }],
			},
		],
		PostToolUse: [
			{
				matcher: '*',
				hooks: [{ type: 'command', command: 'echo "Post tool"' }],
			},
		],
	};

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.getHookTypes).mockResolvedValue(mockHookTypes);
		vi.mocked(api.listHooks).mockResolvedValue(mockHooks);
	});

	const renderHooks = (initialPath: string = '/environment/hooks') => {
		return render(
			<MemoryRouter initialEntries={[initialPath]}>
				<Routes>
					<Route path="/environment/hooks" element={<Hooks />} />
				</Routes>
			</MemoryRouter>
		);
	};

	describe('loading state', () => {
		it('shows loading state initially', async () => {
			vi.mocked(api.listHooks).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve(mockHooks), 100)
					)
			);

			renderHooks();
			expect(screen.getByText('Loading hooks...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			vi.mocked(api.listHooks).mockRejectedValue(new Error('Failed to load hooks'));

			renderHooks();

			await waitFor(() => {
				expect(screen.getByText('Failed to load hooks')).toBeInTheDocument();
			});
		});
	});

	describe('header', () => {
		it('displays page title', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByText('Claude Code Hooks')).toBeInTheDocument();
			});
		});

		it('displays global title when scope is global', async () => {
			renderHooks('/environment/hooks?scope=global');

			await waitFor(() => {
				expect(screen.getByText('Global Claude Code Hooks')).toBeInTheDocument();
			});
		});

		it('displays subtitle', async () => {
			renderHooks();

			await waitFor(() => {
				expect(
					screen.getByText(/configure hook commands that run at specific events/i)
				).toBeInTheDocument();
			});
		});
	});

	describe('scope toggle', () => {
		it('shows Project and Global links', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('link', { name: 'Project' })).toBeInTheDocument();
				expect(screen.getByRole('link', { name: 'Global' })).toBeInTheDocument();
			});
		});

		it('highlights Project by default', async () => {
			renderHooks();

			await waitFor(() => {
				const projectLink = screen.getByRole('link', { name: 'Project' });
				expect(projectLink).toHaveClass('active');
			});
		});

		it('highlights Global when scope=global', async () => {
			renderHooks('/environment/hooks?scope=global');

			await waitFor(() => {
				const globalLink = screen.getByRole('link', { name: 'Global' });
				expect(globalLink).toHaveClass('active');
			});
		});
	});

	describe('event list', () => {
		it('displays all hook event types', async () => {
			renderHooks();

			await waitFor(() => {
				// Some events may appear twice (sidebar + editor), use getAllByText
				expect(screen.getAllByText('PreToolUse').length).toBeGreaterThanOrEqual(1);
				expect(screen.getAllByText('PostToolUse').length).toBeGreaterThanOrEqual(1);
				expect(screen.getAllByText('PreCompact').length).toBeGreaterThanOrEqual(1);
				expect(screen.getAllByText('PrePrompt').length).toBeGreaterThanOrEqual(1);
				expect(screen.getAllByText('Stop').length).toBeGreaterThanOrEqual(1);
			});
		});

		it('shows hook count for events with hooks', async () => {
			renderHooks();

			await waitFor(() => {
				// PreToolUse and PostToolUse have hooks (1 each)
				const counts = screen.getAllByText('1');
				expect(counts.length).toBe(2);
			});
		});

		it('selects first event with hooks by default', async () => {
			renderHooks();

			await waitFor(() => {
				const preToolUseButton = screen.getByRole('button', { name: /pretooluse/i });
				expect(preToolUseButton).toHaveClass('selected');
			});
		});
	});

	describe('selecting an event', () => {
		it('shows event description in editor', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByText('Runs before Claude uses any tool')).toBeInTheDocument();
			});
		});

		it('shows Edit button', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});
		});

		it('switches event when clicked', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /posttooluse/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /posttooluse/i }));

			await waitFor(() => {
				expect(screen.getByText('Runs after Claude uses any tool')).toBeInTheDocument();
			});
		});
	});

	describe('hook view mode', () => {
		it('shows hooks for selected event', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByText('Matcher:')).toBeInTheDocument();
				expect(screen.getByText('Bash')).toBeInTheDocument();
			});
		});

		it('shows commands for hooks', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByText('command:')).toBeInTheDocument();
				expect(screen.getByText('echo "Pre bash"')).toBeInTheDocument();
			});
		});

		it('shows no hooks message when event has no hooks', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /precompact/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /precompact/i }));

			await waitFor(() => {
				expect(screen.getByText('No hooks configured for this event')).toBeInTheDocument();
			});
		});

		it('shows Add Hook button for empty events', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /precompact/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /precompact/i }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Add Hook' })).toBeInTheDocument();
			});
		});
	});

	describe('editing mode', () => {
		it('enters edit mode when Edit clicked', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
				expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
			});
		});

		it('shows matcher input in edit mode', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Matcher (glob pattern)')).toBeInTheDocument();
			});
		});

		it('shows command input in edit mode', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

			await waitFor(() => {
				expect(screen.getByDisplayValue('echo "Pre bash"')).toBeInTheDocument();
			});
		});

		it('cancels editing and restores original state', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
				expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument();
			});
		});
	});

	describe('adding hooks', () => {
		it('shows Add Hook button in edit mode', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: '+ Add Hook' })).toBeInTheDocument();
			});
		});

		it('adds new hook card when Add Hook clicked', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: '+ Add Hook' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: '+ Add Hook' }));

			await waitFor(() => {
				// Should have 2 matcher inputs now (original + new)
				const matcherInputs = screen.getAllByLabelText('Matcher (glob pattern)');
				expect(matcherInputs.length).toBe(2);
			});
		});

		it('shows Add Command button for each hook', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: '+ Add Command' })).toBeInTheDocument();
			});
		});
	});

	describe('saving hooks', () => {
		it('calls updateHook when Save clicked', async () => {
			vi.mocked(api.updateHook).mockResolvedValue({});

			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(api.updateHook).toHaveBeenCalled();
			});
		});

		it('calls deleteHook when all hooks removed', async () => {
			vi.mocked(api.deleteHook).mockResolvedValue({});

			// Mock hooks with only one hook that has empty commands
			vi.mocked(api.listHooks).mockResolvedValue({
				PreToolUse: [
					{
						matcher: '',
						hooks: [{ type: 'command', command: '' }],
					},
				],
			});

			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(api.deleteHook).toHaveBeenCalled();
			});
		});

		it('shows success message after save', async () => {
			vi.mocked(api.updateHook).mockResolvedValue({});

			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(screen.getByText('Hooks saved successfully')).toBeInTheDocument();
			});
		});

		it('shows error message when save fails', async () => {
			vi.mocked(api.updateHook).mockRejectedValue(new Error('Save failed'));

			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(screen.getByText('Save failed')).toBeInTheDocument();
			});
		});

		it('shows Saving... text while saving', async () => {
			let resolvePromise: (value: object) => void;
			vi.mocked(api.updateHook).mockImplementation(
				() =>
					new Promise((resolve) => {
						resolvePromise = resolve;
					})
			);

			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			expect(screen.getByText('Saving...')).toBeInTheDocument();

			// Resolve the promise to allow proper cleanup
			resolvePromise!({});
			await waitFor(() => {
				expect(screen.queryByText('Saving...')).not.toBeInTheDocument();
			});
		});
	});

	describe('type selector', () => {
		it('shows command and url options', async () => {
			renderHooks();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

			await waitFor(() => {
				const selects = screen.getAllByRole('combobox');
				expect(selects[0]).toHaveValue('command');
			});
		});
	});
});
