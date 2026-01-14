import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ConfirmModal } from './ConfirmModal';

describe('ConfirmModal', () => {
	const mockOnCancel = vi.fn();
	const mockOnConfirm = vi.fn();

	const defaultProps = {
		open: true,
		onCancel: mockOnCancel,
		onConfirm: mockOnConfirm,
		title: 'Confirm Action',
		message: 'Are you sure you want to proceed?',
	};

	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
		const portalContent = document.querySelector('.confirm-backdrop');
		if (portalContent) {
			portalContent.remove();
		}
	});

	it('renders nothing when open is false', () => {
		render(<ConfirmModal {...defaultProps} open={false} />);
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('renders dialog when open is true', () => {
		render(<ConfirmModal {...defaultProps} />);
		expect(screen.getByRole('dialog')).toBeInTheDocument();
	});

	it('renders title and message', () => {
		render(<ConfirmModal {...defaultProps} />);
		expect(screen.getByRole('heading', { name: 'Confirm Action' })).toBeInTheDocument();
		expect(screen.getByText('Are you sure you want to proceed?')).toBeInTheDocument();
	});

	it('renders default button labels', () => {
		render(<ConfirmModal {...defaultProps} />);
		expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Confirm' })).toBeInTheDocument();
	});

	it('renders custom button labels', () => {
		render(
			<ConfirmModal
				{...defaultProps}
				cancelLabel="No, go back"
				confirmLabel="Yes, delete it"
			/>
		);
		expect(screen.getByRole('button', { name: 'No, go back' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Yes, delete it' })).toBeInTheDocument();
	});

	it('calls onCancel when cancel button is clicked', () => {
		render(<ConfirmModal {...defaultProps} />);
		fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
		expect(mockOnCancel).toHaveBeenCalledTimes(1);
	});

	it('calls onConfirm when confirm button is clicked', () => {
		render(<ConfirmModal {...defaultProps} />);
		fireEvent.click(screen.getByRole('button', { name: 'Confirm' }));
		expect(mockOnConfirm).toHaveBeenCalledTimes(1);
	});

	it('calls onCancel when Escape key is pressed', () => {
		render(<ConfirmModal {...defaultProps} />);
		fireEvent.keyDown(window, { key: 'Escape' });
		expect(mockOnCancel).toHaveBeenCalledTimes(1);
	});

	it('calls onConfirm when Enter key is pressed', () => {
		render(<ConfirmModal {...defaultProps} />);
		fireEvent.keyDown(window, { key: 'Enter' });
		expect(mockOnConfirm).toHaveBeenCalledTimes(1);
	});

	it('calls onCancel when backdrop is clicked', () => {
		render(<ConfirmModal {...defaultProps} />);
		const backdrop = document.querySelector('.confirm-backdrop');
		fireEvent.click(backdrop!);
		expect(mockOnCancel).toHaveBeenCalledTimes(1);
	});

	it('applies primary variant by default', () => {
		render(<ConfirmModal {...defaultProps} />);
		const confirmButton = screen.getByRole('button', { name: 'Confirm' });
		expect(confirmButton).toHaveClass('variant-primary');
	});

	it('applies danger variant when specified', () => {
		render(<ConfirmModal {...defaultProps} confirmVariant="danger" />);
		const confirmButton = screen.getByRole('button', { name: 'Confirm' });
		expect(confirmButton).toHaveClass('variant-danger');
	});

	it('applies warning variant when specified', () => {
		render(<ConfirmModal {...defaultProps} confirmVariant="warning" />);
		const confirmButton = screen.getByRole('button', { name: 'Confirm' });
		expect(confirmButton).toHaveClass('variant-warning');
	});

	it('renders icon based on action type', () => {
		const { rerender } = render(<ConfirmModal {...defaultProps} action="run" />);
		expect(document.querySelector('.confirm-icon')).toBeInTheDocument();

		rerender(<ConfirmModal {...defaultProps} action="delete" />);
		expect(document.querySelector('.confirm-icon')).toBeInTheDocument();

		rerender(<ConfirmModal {...defaultProps} action="pause" />);
		expect(document.querySelector('.confirm-icon')).toBeInTheDocument();
	});

	it('has proper accessibility attributes', () => {
		render(<ConfirmModal {...defaultProps} />);
		const dialog = screen.getByRole('dialog');
		expect(dialog).toHaveAttribute('aria-modal', 'true');
		expect(dialog).toHaveAttribute('aria-labelledby', 'confirm-title');
	});

	it('disables buttons when loading', () => {
		render(<ConfirmModal {...defaultProps} loading={true} />);
		expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled();
		// The confirm button text changes to "Processing..." when loading
		expect(screen.getByRole('button', { name: /processing/i })).toBeDisabled();
	});

	it('does not call onConfirm when Enter is pressed during loading', () => {
		render(<ConfirmModal {...defaultProps} loading={true} />);
		fireEvent.keyDown(window, { key: 'Enter' });
		expect(mockOnConfirm).not.toHaveBeenCalled();
	});

	it('renders keyboard hints', () => {
		render(<ConfirmModal {...defaultProps} />);
		expect(screen.getByText('Enter')).toBeInTheDocument();
		expect(screen.getByText('Esc')).toBeInTheDocument();
	});
});
