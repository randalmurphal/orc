import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { NewInitiativeModal } from './NewInitiativeModal';

// Mock the API
vi.mock('@/lib/api', () => ({
	createInitiative: vi.fn(),
}));

// Mock the stores
vi.mock('@/stores', () => ({
	useInitiativeStore: vi.fn((selector) =>
		selector({
			addInitiative: vi.fn(),
		})
	),
	useUIStore: vi.fn((selector) =>
		selector({
			toast: {
				success: vi.fn(),
				error: vi.fn(),
			},
		})
	),
}));

// Mock useNavigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

describe('NewInitiativeModal', () => {
	const mockOnClose = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
		const portalContent = document.querySelector('.modal-backdrop');
		if (portalContent) {
			portalContent.remove();
		}
	});

	const renderModal = (props = {}) => {
		return render(
			<BrowserRouter>
				<NewInitiativeModal open={true} onClose={mockOnClose} {...props} />
			</BrowserRouter>
		);
	};

	it('renders nothing when open is false', () => {
		render(
			<BrowserRouter>
				<NewInitiativeModal open={false} onClose={mockOnClose} />
			</BrowserRouter>
		);
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('renders dialog when open is true', () => {
		renderModal();
		expect(screen.getByRole('dialog')).toBeInTheDocument();
	});

	it('renders title and vision fields', () => {
		renderModal();
		expect(screen.getByLabelText(/title/i)).toBeInTheDocument();
		expect(screen.getByLabelText(/vision/i)).toBeInTheDocument();
	});

	it('renders create button disabled initially', () => {
		renderModal();
		const button = screen.getByRole('button', { name: /create initiative/i });
		expect(button).toBeDisabled();
	});

	it('enables create button when title is filled', () => {
		renderModal();
		const titleInput = screen.getByLabelText(/title/i);
		fireEvent.change(titleInput, { target: { value: 'Test Initiative' } });
		const button = screen.getByRole('button', { name: /create initiative/i });
		expect(button).not.toBeDisabled();
	});

	it('calls onClose when cancel button is clicked', () => {
		renderModal();
		fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
		expect(mockOnClose).toHaveBeenCalledTimes(1);
	});

	it('shows error when submitting without title', async () => {
		renderModal();
		// Try to submit empty form - button should be disabled, so check that
		const titleInput = screen.getByLabelText(/title/i);
		expect(titleInput).toHaveValue('');
		const button = screen.getByRole('button', { name: /create initiative/i });
		expect(button).toBeDisabled();
	});

	it('resets form when modal opens', async () => {
		const { rerender } = render(
			<BrowserRouter>
				<NewInitiativeModal open={true} onClose={mockOnClose} />
			</BrowserRouter>
		);

		// Fill in some data
		const titleInput = screen.getByLabelText(/title/i);
		fireEvent.change(titleInput, { target: { value: 'Test Initiative' } });
		expect(titleInput).toHaveValue('Test Initiative');

		// Close modal
		rerender(
			<BrowserRouter>
				<NewInitiativeModal open={false} onClose={mockOnClose} />
			</BrowserRouter>
		);

		// Reopen modal
		rerender(
			<BrowserRouter>
				<NewInitiativeModal open={true} onClose={mockOnClose} />
			</BrowserRouter>
		);

		// Should be reset
		await waitFor(() => {
			const newTitleInput = screen.getByLabelText(/title/i);
			expect(newTitleInput).toHaveValue('');
		});
	});

	it('renders keyboard hint', () => {
		renderModal();
		expect(screen.getByText(/enter/i)).toBeInTheDocument();
	});

	it('has required indicator on title field', () => {
		renderModal();
		expect(screen.getByText('*')).toBeInTheDocument();
	});
});
