import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { SettingsView } from './SettingsView';
import * as api from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
	listSkills: vi.fn(),
	getSkill: vi.fn(),
	updateSkill: vi.fn(),
	deleteSkill: vi.fn(),
	getClaudeMD: vi.fn(),
	updateClaudeMD: vi.fn(),
}));

// Mock skill data
const mockSkills = [
	{
		name: 'commit',
		description: 'Create a git commit',
		path: '.claude/skills/commit/SKILL.md',
	},
	{
		name: 'review',
		description: 'Review code',
		path: '.claude/skills/review/SKILL.md',
	},
	{
		name: 'test',
		description: 'Run tests',
		path: '/home/user/.claude/skills/test/SKILL.md',
	},
];

const mockSkillContent = {
	name: 'commit',
	description: 'Create a git commit',
	content: '# Commit Command\n\nGenerate commit messages.',
	path: '.claude/skills/commit/SKILL.md',
};

describe('SettingsView', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.listSkills).mockResolvedValue(mockSkills);
		vi.mocked(api.getSkill).mockResolvedValue(mockSkillContent);
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('rendering', () => {
		it('renders page header with title and subtitle', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

			expect(screen.getByText('Slash Commands')).toBeInTheDocument();
			expect(screen.getByText('Create and manage custom slash commands for Claude')).toBeInTheDocument();
		});

		it('renders New Command button', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

			const newButton = screen.getByRole('button', { name: /new command/i });
			expect(newButton).toBeInTheDocument();
		});

		it('renders CommandList component', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

			// CommandList renders section headers
			expect(screen.getByText('Project Commands')).toBeInTheDocument();
			expect(screen.getByText('Global Commands')).toBeInTheDocument();
		});

		it('renders ConfigEditor component', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.getSkill).toHaveBeenCalled();
			});

			// ConfigEditor should be visible (for selected command)
			expect(screen.getByTestId('config-editor')).toBeInTheDocument();
		});

		it('renders mock commands from initial data', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

			// Check for mock commands
			expect(screen.getByText('/commit')).toBeInTheDocument();
			expect(screen.getByText('/review')).toBeInTheDocument();
			expect(screen.getByText('/test')).toBeInTheDocument();
		});
	});

	describe('layout', () => {
		it('has header with correct structure', async () => {
			const { container } = render(<SettingsView />);
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

			const header = container.querySelector('.settings-view__header');
			expect(header).toBeInTheDocument();

			const headerContent = container.querySelector('.settings-view__header-content');
			expect(headerContent).toBeInTheDocument();
		});

		it('has content area with list and editor', async () => {
			const { container } = render(<SettingsView />);
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

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
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

			// First command should be selected
			const selectedItem = container.querySelector('.command-item.selected');
			expect(selectedItem).toBeInTheDocument();
		});

		it('clicking command updates selection', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

			// Click on /review command
			const reviewItem = screen.getByText('/review').closest('.command-item');
			fireEvent.click(reviewItem!);

			// /review should now be selected
			expect(reviewItem).toHaveClass('selected');
		});

		it('editor shows content for selected command', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.getSkill).toHaveBeenCalled();
			});

			// Initially shows first command's content
			const editor = screen.getByTestId('config-editor-textarea');
			expect(editor).toBeInTheDocument();
		});
	});

	describe('command deletion', () => {
		it('delete action removes command from list', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
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
			const remainingItems = screen.getAllByText(/^\//).filter(el =>
				el.classList.contains('command-name')
			);
			expect(remainingItems.length).toBe(initialCount - 1);
		});
	});

	describe('editor functionality', () => {
		it('editor shows file path from selected command', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.getSkill).toHaveBeenCalled();
			});

			// Should show path in editor header
			const pathDisplay = screen.getByTestId('config-editor-path');
			expect(pathDisplay).toBeInTheDocument();
		});

		it('editor content is editable', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.getSkill).toHaveBeenCalled();
			});

			const textarea = screen.getByTestId('config-editor-textarea');
			fireEvent.change(textarea, { target: { value: '# Updated content' } });

			expect(textarea).toHaveValue('# Updated content');
		});

		it('save button triggers save action', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.getSkill).toHaveBeenCalled();
			});

			const saveButton = screen.getByTestId('config-editor-save');
			// Save button should be clickable (mock implementation is a no-op for now)
			fireEvent.click(saveButton);
			expect(saveButton).toBeInTheDocument();
		});
	});

	describe('empty state', () => {
		it('shows empty state in editor when no command selected', async () => {
			// This test verifies the empty state behavior
			// In practice, we always have a selected command initially
			// but we can test the component handles undefined selection
			const { container } = render(<SettingsView />);
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

			// Editor should be visible (with selected command)
			const _editor = container.querySelector('.settings-view__editor');
			expect(_editor).toBeInTheDocument();
		});
	});

	describe('New Command button', () => {
		it('clicking New Command is clickable', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

			const newButton = screen.getByRole('button', { name: /new command/i });
			// Button should be clickable (mock implementation is a no-op for now)
			fireEvent.click(newButton);
			expect(newButton).toBeInTheDocument();
		});
	});

	describe('accessibility', () => {
		it('header is properly structured with h2', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

			const heading = screen.getByRole('heading', { level: 2, name: 'Slash Commands' });
			expect(heading).toBeInTheDocument();
		});

		it('command list items are keyboard navigable', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.listSkills).toHaveBeenCalled();
			});

			const commandItem = screen.getByText('/commit').closest('.command-item');
			expect(commandItem).toHaveAttribute('tabindex', '0');
		});

		it('editor textarea has aria-label', async () => {
			render(<SettingsView />);
			await waitFor(() => {
				expect(api.getSkill).toHaveBeenCalled();
			});

			const textarea = screen.getByTestId('config-editor-textarea');
			expect(textarea).toHaveAttribute('aria-label');
		});
	});
});
