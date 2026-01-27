import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { NewCommandModal } from './NewCommandModal';
import { configClient } from '@/lib/client';
import { SettingsScope } from '@/gen/orc/v1/config_pb';
import type { Skill } from '@/gen/orc/v1/config_pb';

vi.mock('@/lib/client', () => ({
	configClient: {
		createSkill: vi.fn(),
	},
}));

vi.mock('@/stores/uiStore', () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn(),
	},
}));

const mockCreatedSkill: Partial<Skill> = {
	name: 'my-command',
	description: 'My custom command',
	content: '# My Command',
	scope: SettingsScope.GLOBAL,
	userInvocable: true,
};

describe('NewCommandModal', () => {
	const mockOnClose = vi.fn();
	const mockOnCreate = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(configClient.createSkill).mockResolvedValue({
			skill: mockCreatedSkill as Skill,
			$typeName: 'orc.v1.CreateSkillResponse',
		});
	});

	// SC-1: Modal renders when open
	describe('rendering', () => {
		it('renders modal with title when open', () => {
			render(<NewCommandModal open={true} onClose={mockOnClose} onCreate={mockOnCreate} />);
			expect(screen.getByRole('dialog')).toBeInTheDocument();
			expect(screen.getByText('New Command')).toBeInTheDocument();
		});

		it('does not render when closed', () => {
			render(<NewCommandModal open={false} onClose={mockOnClose} onCreate={mockOnCreate} />);
			expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		});
	});

	// SC-2: Form fields exist and work
	describe('form fields', () => {
		it('has name input field', () => {
			render(<NewCommandModal open={true} onClose={mockOnClose} onCreate={mockOnCreate} />);
			const nameInput = screen.getByLabelText(/name/i);
			expect(nameInput).toBeInTheDocument();
		});

		it('has description input field', () => {
			render(<NewCommandModal open={true} onClose={mockOnClose} onCreate={mockOnCreate} />);
			expect(screen.getByLabelText(/description/i)).toBeInTheDocument();
		});

		it('has scope selector', () => {
			render(<NewCommandModal open={true} onClose={mockOnClose} onCreate={mockOnCreate} />);
			expect(screen.getByLabelText(/scope/i)).toBeInTheDocument();
		});

		it('allows entering command name', () => {
			render(<NewCommandModal open={true} onClose={mockOnClose} onCreate={mockOnCreate} />);
			const nameInput = screen.getByLabelText(/name/i);
			fireEvent.change(nameInput, { target: { value: 'my-command' } });
			expect(nameInput).toHaveValue('my-command');
		});
	});

	// SC-3: Form submission creates skill via API
	describe('form submission', () => {
		it('calls createSkill API on submit', async () => {
			render(<NewCommandModal open={true} onClose={mockOnClose} onCreate={mockOnCreate} />);
			fireEvent.change(screen.getByLabelText(/name/i), { target: { value: 'my-command' } });
			fireEvent.change(screen.getByLabelText(/description/i), { target: { value: 'My desc' } });
			fireEvent.click(screen.getByRole('button', { name: /create/i }));

			await waitFor(() => {
				expect(configClient.createSkill).toHaveBeenCalledWith(
					expect.objectContaining({ name: 'my-command', description: 'My desc' })
				);
			});
		});

		it('calls onCreate callback with created skill', async () => {
			render(<NewCommandModal open={true} onClose={mockOnClose} onCreate={mockOnCreate} />);
			fireEvent.change(screen.getByLabelText(/name/i), { target: { value: 'my-command' } });
			fireEvent.click(screen.getByRole('button', { name: /create/i }));

			await waitFor(() => {
				expect(mockOnCreate).toHaveBeenCalledWith(expect.objectContaining({ name: 'my-command' }));
			});
		});

		it('closes modal after successful creation', async () => {
			render(<NewCommandModal open={true} onClose={mockOnClose} onCreate={mockOnCreate} />);
			fireEvent.change(screen.getByLabelText(/name/i), { target: { value: 'my-command' } });
			fireEvent.click(screen.getByRole('button', { name: /create/i }));

			await waitFor(() => {
				expect(mockOnClose).toHaveBeenCalled();
			});
		});

		it('disables create button when name is empty', () => {
			render(<NewCommandModal open={true} onClose={mockOnClose} onCreate={mockOnCreate} />);
			expect(screen.getByRole('button', { name: /create/i })).toBeDisabled();
		});
	});
});
