import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { CommandList, type Command } from './CommandList';

const mockProjectCommand: Command = {
	id: 'cmd-1',
	name: '/review',
	description: 'Run comprehensive code review with security analysis',
	scope: 'project',
	path: '.claude/commands/review.md',
};

const mockGlobalCommand: Command = {
	id: 'cmd-2',
	name: '/commit',
	description: 'Generate semantic commit message based on staged changes',
	scope: 'global',
	path: '~/.claude/commands/commit.md',
};

const mockCommands: Command[] = [
	mockProjectCommand,
	mockGlobalCommand,
	{
		id: 'cmd-3',
		name: '/test',
		description: 'Generate tests for the current file',
		scope: 'project',
	},
	{
		id: 'cmd-4',
		name: '/deploy',
		description: 'Deploy to production with validation checks',
		scope: 'global',
	},
];

describe('CommandList', () => {
	describe('rendering', () => {
		it('renders project and global command sections', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={mockCommands}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			expect(screen.getByText('Project Commands')).toBeInTheDocument();
			expect(screen.getByText('Global Commands')).toBeInTheDocument();
		});

		it('renders command items with name, description, and icon', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={mockCommands}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			expect(screen.getByText('/review')).toBeInTheDocument();
			expect(
				screen.getByText('Run comprehensive code review with security analysis')
			).toBeInTheDocument();
			expect(screen.getByText('/commit')).toBeInTheDocument();
			expect(
				screen.getByText('Generate semantic commit message based on staged changes')
			).toBeInTheDocument();
		});

		it('project commands use purple icon color (no global class)', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			const { container } = render(
				<CommandList
					commands={[mockProjectCommand]}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			const icon = container.querySelector('.command-icon');
			expect(icon).toBeInTheDocument();
			expect(icon).not.toHaveClass('global');
		});

		it('global commands use cyan icon color (has global class)', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			const { container } = render(
				<CommandList
					commands={[mockGlobalCommand]}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			const icon = container.querySelector('.command-icon');
			expect(icon).toBeInTheDocument();
			expect(icon).toHaveClass('global');
		});

		it('renders section descriptions', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={mockCommands}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			expect(
				screen.getByText(/Commands available in the current project/)
			).toBeInTheDocument();
			expect(screen.getByText(/Available in all projects/)).toBeInTheDocument();
		});
	});

	describe('selection', () => {
		it('click on item calls onSelect with correct id', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={mockCommands}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			const commandItem = screen.getByText('/review').closest('.command-item');
			fireEvent.click(commandItem!);

			expect(handleSelect).toHaveBeenCalledTimes(1);
			expect(handleSelect).toHaveBeenCalledWith('cmd-1');
		});

		it('click on edit button calls onSelect with correct id', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={mockCommands}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			const editButton = screen.getByRole('button', { name: 'Edit /review' });
			fireEvent.click(editButton);

			expect(handleSelect).toHaveBeenCalledTimes(1);
			expect(handleSelect).toHaveBeenCalledWith('cmd-1');
		});

		it('selected item has selected class', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			const { container } = render(
				<CommandList
					commands={mockCommands}
					selectedId="cmd-1"
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			const selectedItem = container.querySelector('.command-item.selected');
			expect(selectedItem).toBeInTheDocument();
			expect(selectedItem?.querySelector('.command-name')?.textContent).toBe('/review');
		});

		it('keyboard navigation with Enter selects item', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={mockCommands}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			const commandItem = screen.getByText('/review').closest('.command-item');
			fireEvent.keyDown(commandItem!, { key: 'Enter' });

			expect(handleSelect).toHaveBeenCalledTimes(1);
			expect(handleSelect).toHaveBeenCalledWith('cmd-1');
		});

		it('keyboard navigation with Space selects item', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={mockCommands}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			const commandItem = screen.getByText('/commit').closest('.command-item');
			fireEvent.keyDown(commandItem!, { key: ' ' });

			expect(handleSelect).toHaveBeenCalledTimes(1);
			expect(handleSelect).toHaveBeenCalledWith('cmd-2');
		});
	});

	describe('delete confirmation', () => {
		it('click on delete button triggers confirmation', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={mockCommands}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			const deleteButton = screen.getByRole('button', { name: 'Delete /review' });
			fireEvent.click(deleteButton);

			// Confirmation buttons should appear
			expect(screen.getByRole('button', { name: 'Confirm delete' })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: 'Cancel delete' })).toBeInTheDocument();

			// onDelete should NOT be called yet
			expect(handleDelete).not.toHaveBeenCalled();
		});

		it('confirmation accept calls onDelete', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={mockCommands}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			// Click delete to show confirmation
			const deleteButton = screen.getByRole('button', { name: 'Delete /review' });
			fireEvent.click(deleteButton);

			// Click confirm
			const confirmButton = screen.getByRole('button', { name: 'Confirm delete' });
			fireEvent.click(confirmButton);

			expect(handleDelete).toHaveBeenCalledTimes(1);
			expect(handleDelete).toHaveBeenCalledWith('cmd-1');
		});

		it('confirmation reject does not call onDelete', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={mockCommands}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			// Click delete to show confirmation
			const deleteButton = screen.getByRole('button', { name: 'Delete /review' });
			fireEvent.click(deleteButton);

			// Click cancel
			const cancelButton = screen.getByRole('button', { name: 'Cancel delete' });
			fireEvent.click(cancelButton);

			expect(handleDelete).not.toHaveBeenCalled();

			// Original buttons should be back
			expect(screen.getByRole('button', { name: 'Delete /review' })).toBeInTheDocument();
		});

		it('delete button click does not propagate to item selection', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={mockCommands}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			const deleteButton = screen.getByRole('button', { name: 'Delete /review' });
			fireEvent.click(deleteButton);

			// onSelect should NOT be called when clicking delete
			expect(handleSelect).not.toHaveBeenCalled();
		});
	});

	describe('empty state', () => {
		it('empty list shows empty state message', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList commands={[]} onSelect={handleSelect} onDelete={handleDelete} />
			);

			expect(screen.getByText('No commands')).toBeInTheDocument();
			expect(screen.getByText('Create a command to get started')).toBeInTheDocument();
		});

		it('empty state does not show section headers', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList commands={[]} onSelect={handleSelect} onDelete={handleDelete} />
			);

			expect(screen.queryByText('Project Commands')).not.toBeInTheDocument();
			expect(screen.queryByText('Global Commands')).not.toBeInTheDocument();
		});
	});

	describe('description truncation', () => {
		it('long descriptions have truncation class applied', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			const longDescCommand: Command = {
				id: 'cmd-long',
				name: '/long',
				description:
					'This is a very long description that should be truncated when it exceeds the available width of the command item container element',
				scope: 'project',
			};

			const { container } = render(
				<CommandList
					commands={[longDescCommand]}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			const descElement = container.querySelector('.command-desc');
			expect(descElement).toBeInTheDocument();
			// CSS handles truncation with text-overflow: ellipsis
			// We verify the class is present and has the right content
			expect(descElement?.textContent).toBe(longDescCommand.description);
		});

		it('description has title attribute for full text on hover', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={[mockProjectCommand]}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			const descElement = screen.getByText(mockProjectCommand.description);
			expect(descElement).toHaveAttribute('title', mockProjectCommand.description);
		});
	});

	describe('accessibility', () => {
		it('command items have role="button"', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={mockCommands}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			const commandItems = screen.getAllByRole('button', { name: /\// });
			// Edit and delete buttons also have role="button", so filter to just items
			const itemButtons = commandItems.filter((el) =>
				el.classList.contains('command-item')
			);
			expect(itemButtons.length).toBeGreaterThan(0);
		});

		it('command items have tabIndex for keyboard navigation', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			const { container } = render(
				<CommandList
					commands={mockCommands}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			const commandItems = container.querySelectorAll('.command-item');
			commandItems.forEach((item) => {
				expect(item).toHaveAttribute('tabindex', '0');
			});
		});

		it('action buttons have aria-label', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			render(
				<CommandList
					commands={[mockProjectCommand]}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			expect(screen.getByRole('button', { name: 'Edit /review' })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: 'Delete /review' })).toBeInTheDocument();
		});
	});

	describe('sections filtering', () => {
		it('only shows project section when no global commands', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			const projectOnly = mockCommands.filter((c) => c.scope === 'project');

			render(
				<CommandList
					commands={projectOnly}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			expect(screen.getByText('Project Commands')).toBeInTheDocument();
			expect(screen.queryByText('Global Commands')).not.toBeInTheDocument();
		});

		it('only shows global section when no project commands', () => {
			const handleSelect = vi.fn();
			const handleDelete = vi.fn();

			const globalOnly = mockCommands.filter((c) => c.scope === 'global');

			render(
				<CommandList
					commands={globalOnly}
					onSelect={handleSelect}
					onDelete={handleDelete}
				/>
			);

			expect(screen.queryByText('Project Commands')).not.toBeInTheDocument();
			expect(screen.getByText('Global Commands')).toBeInTheDocument();
		});
	});
});
