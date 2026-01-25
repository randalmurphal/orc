import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { SettingsView } from './SettingsView';
import type { SkillInfo, Skill } from '@/lib/api';

// Mock the api module
vi.mock('@/lib/api', () => ({
	listSkills: vi.fn(),
	getSkill: vi.fn(),
	updateSkill: vi.fn(),
}));

// Import after mock to get mocked version
import * as api from '@/lib/api';

const mockSkillInfos: SkillInfo[] = [
	{
		name: 'commit',
		description: 'Create a commit with staged changes',
		path: '/home/user/.claude/commands/commit.md', // global (contains /.claude/)
	},
	{
		name: 'review',
		description: 'Review code changes',
		path: '/project/commands/review.md', // project (no /.claude/)
	},
	{
		name: 'test',
		description: 'Run tests',
		path: '/home/user/.claude/commands/test.md', // global
	},
];

const mockSkillContent = '# Mock Command Content\n\nThis is the command template.';

const mockSkill: Skill = {
	name: 'commit',
	description: 'Create a commit',
	content: mockSkillContent,
	path: '/home/user/.claude/commands/commit.md',
};

describe('SettingsView', () => {
	beforeEach(() => {
		// Set up default mocks
		vi.mocked(api.listSkills).mockResolvedValue(mockSkillInfos);
		vi.mocked(api.getSkill).mockResolvedValue(mockSkill);
		vi.mocked(api.updateSkill).mockResolvedValue(mockSkill);
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('rendering', () => {
		it('renders page header with title and subtitle', async () => {
			render(<SettingsView />);

			expect(screen.getByText('Slash Commands')).toBeInTheDocument();
			expect(screen.getByText('Custom commands for Claude Code (~/.claude/commands)')).toBeInTheDocument();
		});

		it('renders New Command button', async () => {
			render(<SettingsView />);

			const newButton = screen.getByRole('button', { name: /new command/i });
			expect(newButton).toBeInTheDocument();
		});

		it('renders CommandList component', async () => {
			render(<SettingsView />);

			// Wait for skills to load
			await waitFor(() => {
				expect(screen.getByText('/commit')).toBeInTheDocument();
			});

			// CommandList renders section headers
			expect(screen.getByText('Project Commands')).toBeInTheDocument();
			expect(screen.getByText('Global Commands')).toBeInTheDocument();
		});

		it('renders ConfigEditor component after data loads', async () => {
			render(<SettingsView />);

			// Wait for editor to render with selected command
			await waitFor(() => {
				expect(screen.getByTestId('config-editor')).toBeInTheDocument();
			});
		});

		it('renders commands from API', async () => {
			render(<SettingsView />);

			// Wait for commands to load
			await waitFor(() => {
				expect(screen.getByText('/commit')).toBeInTheDocument();
			});

			expect(screen.getByText('/review')).toBeInTheDocument();
			expect(screen.getByText('/test')).toBeInTheDocument();
		});
	});

	describe('layout', () => {
		it('has header with correct structure', async () => {
			const { container } = render(<SettingsView />);

			const header = container.querySelector('.settings-view__header');
			expect(header).toBeInTheDocument();

			const headerContent = container.querySelector('.settings-view__header-content');
			expect(headerContent).toBeInTheDocument();
		});

		it('has content area with list and editor', async () => {
			const { container } = render(<SettingsView />);

			const content = container.querySelector('.settings-view__content');
			expect(content).toBeInTheDocument();

			const list = container.querySelector('.settings-view__list');
			expect(list).toBeInTheDocument();

			const editor = container.querySelector('.settings-view__editor');
			expect(editor).toBeInTheDocument();
		});
	});

	describe('command selection', () => {
		it('first command is selected by default', async () => {
			const { container } = render(<SettingsView />);

			// Wait for commands to load
			await waitFor(() => {
				expect(screen.getByText('/commit')).toBeInTheDocument();
			});

			// First command should be selected
			const selectedItem = container.querySelector('.command-item.selected');
			expect(selectedItem).toBeInTheDocument();
		});

		it('clicking command updates selection', async () => {
			render(<SettingsView />);

			// Wait for commands to load
			await waitFor(() => {
				expect(screen.getByText('/commit')).toBeInTheDocument();
			});

			// Click on /review command
			const reviewItem = screen.getByText('/review').closest('.command-item');
			fireEvent.click(reviewItem!);

			// /review should now be selected
			expect(reviewItem).toHaveClass('selected');
		});

		it('editor shows content for selected command', async () => {
			render(<SettingsView />);

			// Wait for editor to appear
			await waitFor(() => {
				expect(screen.getByTestId('config-editor-textarea')).toBeInTheDocument();
			});

			const editor = screen.getByTestId('config-editor-textarea');
			expect(editor).toBeInTheDocument();
		});
	});

	describe('command deletion', () => {
		it('delete action removes command from list', async () => {
			render(<SettingsView />);

			// Wait for commands to load
			await waitFor(() => {
				expect(screen.getByText('/commit')).toBeInTheDocument();
			});

			// Get initial command count
			const initialItems = screen.getAllByText(/^\//).filter(el =>
				el.classList.contains('command-name')
			);
			const initialCount = initialItems.length;

			// Click delete on first project command
			const deleteButton = screen.getByRole('button', { name: /delete \/commit/i });
			fireEvent.click(deleteButton);

			// Confirm delete
			const confirmButton = screen.getByRole('button', { name: 'Confirm delete' });
			fireEvent.click(confirmButton);

			// Command should be removed
			await waitFor(() => {
				const remainingItems = screen.queryAllByText(/^\//).filter(el =>
					el.classList.contains('command-name')
				);
				expect(remainingItems.length).toBe(initialCount - 1);
			});
		});
	});

	describe('editor functionality', () => {
		it('editor shows file path from selected command', async () => {
			render(<SettingsView />);

			// Wait for editor to load
			await waitFor(() => {
				expect(screen.getByTestId('config-editor-path')).toBeInTheDocument();
			});

			const pathDisplay = screen.getByTestId('config-editor-path');
			expect(pathDisplay).toBeInTheDocument();
		});

		it('editor content is editable', async () => {
			render(<SettingsView />);

			// Wait for editor to load
			await waitFor(() => {
				expect(screen.getByTestId('config-editor-textarea')).toBeInTheDocument();
			});

			const textarea = screen.getByTestId('config-editor-textarea');
			fireEvent.change(textarea, { target: { value: '# Updated content' } });

			expect(textarea).toHaveValue('# Updated content');
		});

		it('save button triggers save action', async () => {
			render(<SettingsView />);

			// Wait for editor to load
			await waitFor(() => {
				expect(screen.getByTestId('config-editor-save')).toBeInTheDocument();
			});

			const saveButton = screen.getByTestId('config-editor-save');
			fireEvent.click(saveButton);

			// Verify updateSkill was called
			await waitFor(() => {
				expect(api.updateSkill).toHaveBeenCalled();
			});
		});
	});

	describe('empty state', () => {
		it('shows empty state when no commands available', async () => {
			vi.mocked(api.listSkills).mockResolvedValue([]);

			const { container } = render(<SettingsView />);

			// Wait for fetch to complete
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

			// Editor should show empty state
			const emptyState = container.querySelector('.settings-view__empty');
			expect(emptyState).toBeInTheDocument();
		});
	});

	describe('New Command button', () => {
		it('clicking New Command is clickable', async () => {
			render(<SettingsView />);

			const newButton = screen.getByRole('button', { name: /new command/i });
			// Button should be clickable
			fireEvent.click(newButton);
			expect(newButton).toBeInTheDocument();
		});
	});

	describe('accessibility', () => {
		it('header is properly structured with h2', async () => {
			render(<SettingsView />);

			const heading = screen.getByRole('heading', { level: 2, name: 'Slash Commands' });
			expect(heading).toBeInTheDocument();
		});

		it('command list items are keyboard navigable', async () => {
			render(<SettingsView />);

			// Wait for commands to load
			await waitFor(() => {
				expect(screen.getByText('/commit')).toBeInTheDocument();
			});

			const commandItem = screen.getByText('/commit').closest('.command-item');
			expect(commandItem).toHaveAttribute('tabindex', '0');
		});

		it('editor textarea has aria-label', async () => {
			render(<SettingsView />);

			// Wait for editor to load
			await waitFor(() => {
				expect(screen.getByTestId('config-editor-textarea')).toBeInTheDocument();
			});

			const textarea = screen.getByTestId('config-editor-textarea');
			expect(textarea).toHaveAttribute('aria-label');
		});
	});

	describe('error handling', () => {
		it('handles API error gracefully', async () => {
			vi.mocked(api.listSkills).mockRejectedValue(new Error('Network error'));

			const { container } = render(<SettingsView />);

			// Should still render the view structure
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

			// View should be rendered (empty but functional)
			expect(container.querySelector('.settings-view')).toBeInTheDocument();
		});
	});
});
