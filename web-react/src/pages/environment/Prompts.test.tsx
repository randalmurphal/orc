import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { Prompts } from './Prompts';
import * as api from '@/lib/api';

// Mock the API functions
vi.mock('@/lib/api', () => ({
	listPrompts: vi.fn(),
	getPrompt: vi.fn(),
	updatePrompt: vi.fn(),
	resetPrompt: vi.fn(),
}));

describe('Prompts', () => {
	const mockPromptsList: api.PromptInfo[] = [
		{ phase: 'spec', is_custom: false },
		{ phase: 'implement', is_custom: true },
		{ phase: 'test', is_custom: false },
		{ phase: 'docs', is_custom: false },
		{ phase: 'validate', is_custom: false },
	];

	const mockPrompt: api.Prompt = {
		phase: 'spec',
		content: '# Spec Phase\n\nAnalyze the task and create a specification.',
		is_custom: false,
	};

	const mockCustomPrompt: api.Prompt = {
		phase: 'implement',
		content: '# Custom Implementation\n\nCustom prompt content.',
		is_custom: true,
	};

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.listPrompts).mockResolvedValue(mockPromptsList);
		vi.mocked(api.getPrompt).mockResolvedValue(mockPrompt);
	});

	const renderPrompts = (initialPath: string = '/environment/prompts') => {
		return render(
			<MemoryRouter initialEntries={[initialPath]}>
				<Routes>
					<Route path="/environment/prompts" element={<Prompts />} />
				</Routes>
			</MemoryRouter>
		);
	};

	describe('loading state', () => {
		it('shows loading state initially', async () => {
			vi.mocked(api.listPrompts).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve(mockPromptsList), 100)
					)
			);

			renderPrompts();
			expect(screen.getByText('Loading prompts...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			vi.mocked(api.listPrompts).mockRejectedValue(
				new Error('Failed to load')
			);

			renderPrompts();

			await waitFor(() => {
				expect(screen.getByText('Failed to load')).toBeInTheDocument();
			});
		});
	});

	describe('header', () => {
		it('displays page title', async () => {
			renderPrompts();

			await waitFor(() => {
				expect(screen.getByText('Phase Prompts')).toBeInTheDocument();
			});
		});

		it('displays subtitle', async () => {
			renderPrompts();

			await waitFor(() => {
				expect(
					screen.getByText('Customize prompts for each execution phase')
				).toBeInTheDocument();
			});
		});
	});

	describe('phase list', () => {
		it('displays all phases', async () => {
			renderPrompts();

			await waitFor(() => {
				expect(screen.getByText('spec')).toBeInTheDocument();
				expect(screen.getByText('implement')).toBeInTheDocument();
				expect(screen.getByText('test')).toBeInTheDocument();
				expect(screen.getByText('docs')).toBeInTheDocument();
				expect(screen.getByText('validate')).toBeInTheDocument();
			});
		});

		it('shows phase descriptions', async () => {
			renderPrompts();

			await waitFor(() => {
				expect(screen.getByText('Define requirements and approach')).toBeInTheDocument();
				expect(screen.getByText('Write the implementation code')).toBeInTheDocument();
			});
		});

		it('shows Custom badge for custom prompts', async () => {
			renderPrompts();

			await waitFor(() => {
				expect(screen.getByText('Custom')).toBeInTheDocument();
			});
		});
	});

	describe('selecting a phase', () => {
		it('loads prompt content when clicked', async () => {
			renderPrompts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /spec/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /spec/i }));

			await waitFor(() => {
				expect(api.getPrompt).toHaveBeenCalledWith('spec');
			});
		});

		it('shows prompt content in textarea', async () => {
			renderPrompts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /spec/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /spec/i }));

			await waitFor(() => {
				const textarea = screen.getByRole('textbox');
				expect(textarea).toHaveValue(mockPrompt.content);
			});
		});

		it('shows Save button', async () => {
			renderPrompts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /spec/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /spec/i }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});
		});

		it('disables Save button when no changes', async () => {
			renderPrompts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /spec/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /spec/i }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
			});
		});

		it('enables Save button when content changes', async () => {
			renderPrompts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /spec/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /spec/i }));

			await waitFor(() => {
				expect(screen.getByRole('textbox')).toBeInTheDocument();
			});

			const textarea = screen.getByRole('textbox');
			fireEvent.change(textarea, { target: { value: 'Changed content' } });

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).not.toBeDisabled();
			});
		});
	});

	describe('reset to default', () => {
		it('shows Reset button for custom prompts', async () => {
			vi.mocked(api.getPrompt).mockResolvedValue(mockCustomPrompt);

			renderPrompts();

			await waitFor(() => {
				expect(screen.getByText('implement')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('implement'));

			await waitFor(() => {
				expect(
					screen.getByRole('button', { name: 'Reset to Default' })
				).toBeInTheDocument();
			});
		});

		it('does not show Reset button for default prompts', async () => {
			renderPrompts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /spec/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /spec/i }));

			await waitFor(() => {
				expect(screen.getByRole('textbox')).toBeInTheDocument();
			});

			expect(
				screen.queryByRole('button', { name: 'Reset to Default' })
			).not.toBeInTheDocument();
		});

		it('calls resetPrompt when Reset clicked and confirmed', async () => {
			vi.mocked(api.getPrompt).mockResolvedValue(mockCustomPrompt);
			vi.mocked(api.resetPrompt).mockResolvedValue({});
			vi.stubGlobal('confirm', vi.fn(() => true));

			renderPrompts();

			await waitFor(() => {
				expect(screen.getByText('implement')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('implement'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Reset to Default' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Reset to Default' }));

			await waitFor(() => {
				expect(api.resetPrompt).toHaveBeenCalledWith('implement');
			});

			vi.unstubAllGlobals();
		});

		it('does not reset when confirm cancelled', async () => {
			vi.mocked(api.getPrompt).mockResolvedValue(mockCustomPrompt);
			vi.stubGlobal('confirm', vi.fn(() => false));

			renderPrompts();

			await waitFor(() => {
				expect(screen.getByText('implement')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('implement'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Reset to Default' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Reset to Default' }));

			expect(api.resetPrompt).not.toHaveBeenCalled();

			vi.unstubAllGlobals();
		});

		it('shows success message after reset', async () => {
			vi.mocked(api.getPrompt).mockResolvedValue(mockCustomPrompt);
			vi.mocked(api.resetPrompt).mockResolvedValue({});
			vi.stubGlobal('confirm', vi.fn(() => true));

			renderPrompts();

			await waitFor(() => {
				expect(screen.getByText('implement')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('implement'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Reset to Default' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Reset to Default' }));

			await waitFor(() => {
				expect(screen.getByText('Prompt reset to default')).toBeInTheDocument();
			});

			vi.unstubAllGlobals();
		});
	});

	describe('saving prompts', () => {
		it('calls updatePrompt with form data', async () => {
			vi.mocked(api.updatePrompt).mockResolvedValue({});

			renderPrompts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /spec/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /spec/i }));

			await waitFor(() => {
				expect(screen.getByRole('textbox')).toBeInTheDocument();
			});

			const textarea = screen.getByRole('textbox');
			fireEvent.change(textarea, { target: { value: 'New content' } });
			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(api.updatePrompt).toHaveBeenCalledWith('spec', 'New content');
			});
		});

		it('shows Saving... text while saving', async () => {
			vi.mocked(api.updatePrompt).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve({}), 100)
					)
			);

			renderPrompts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /spec/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /spec/i }));

			await waitFor(() => {
				expect(screen.getByRole('textbox')).toBeInTheDocument();
			});

			const textarea = screen.getByRole('textbox');
			fireEvent.change(textarea, { target: { value: 'New content' } });
			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			expect(screen.getByText('Saving...')).toBeInTheDocument();
		});

		it('shows success message after save', async () => {
			vi.mocked(api.updatePrompt).mockResolvedValue({});

			renderPrompts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /spec/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /spec/i }));

			await waitFor(() => {
				expect(screen.getByRole('textbox')).toBeInTheDocument();
			});

			const textarea = screen.getByRole('textbox');
			fireEvent.change(textarea, { target: { value: 'New content' } });
			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(screen.getByText('Prompt saved successfully')).toBeInTheDocument();
			});
		});

		it('shows error message when save fails', async () => {
			vi.mocked(api.updatePrompt).mockRejectedValue(new Error('Save failed'));

			renderPrompts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /spec/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /spec/i }));

			await waitFor(() => {
				expect(screen.getByRole('textbox')).toBeInTheDocument();
			});

			const textarea = screen.getByRole('textbox');
			fireEvent.change(textarea, { target: { value: 'New content' } });
			fireEvent.click(screen.getByRole('button', { name: 'Save' }));

			await waitFor(() => {
				expect(screen.getByText('Save failed')).toBeInTheDocument();
			});
		});
	});

	describe('template variables hint', () => {
		it('shows template variables hint', async () => {
			renderPrompts();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /spec/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /spec/i }));

			await waitFor(() => {
				expect(screen.getByText('{{TASK_TITLE}}')).toBeInTheDocument();
				expect(screen.getByText('{{TASK_DESCRIPTION}}')).toBeInTheDocument();
				expect(screen.getByText('{{SPEC_CONTENT}}')).toBeInTheDocument();
			});
		});
	});

	describe('no selection state', () => {
		it('shows help message when no phase selected', async () => {
			renderPrompts();

			await waitFor(() => {
				expect(
					screen.getByText('Select a phase to view or edit its prompt')
				).toBeInTheDocument();
			});
		});
	});
});
