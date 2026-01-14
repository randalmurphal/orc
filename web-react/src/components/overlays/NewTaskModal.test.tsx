import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { NewTaskModal } from './NewTaskModal';

// Mock the API
vi.mock('@/lib/api', () => ({
	createProjectTask: vi.fn(),
}));

// Mock the stores
vi.mock('@/stores', () => ({
	useCurrentProjectId: vi.fn(() => 'proj-1'),
}));

vi.mock('@/stores/initiativeStore', () => ({
	useInitiatives: vi.fn(() => [
		{ id: 'INIT-001', title: 'Test Initiative', status: 'active' },
	]),
	useCurrentInitiativeId: vi.fn(() => null),
}));

vi.mock('@/stores/uiStore', () => ({
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

describe('NewTaskModal', () => {
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
				<NewTaskModal open={true} onClose={mockOnClose} {...props} />
			</BrowserRouter>
		);
	};

	it('renders nothing when open is false', () => {
		render(
			<BrowserRouter>
				<NewTaskModal open={false} onClose={mockOnClose} />
			</BrowserRouter>
		);
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('renders dialog when open is true', () => {
		renderModal();
		expect(screen.getByRole('dialog')).toBeInTheDocument();
	});

	it('renders all form fields', () => {
		renderModal();
		expect(screen.getByLabelText(/title/i)).toBeInTheDocument();
		expect(screen.getByLabelText(/description/i)).toBeInTheDocument();
		expect(screen.getByText(/weight/i)).toBeInTheDocument();
		expect(screen.getByText(/priority/i)).toBeInTheDocument();
		expect(screen.getByText(/category/i)).toBeInTheDocument();
	});

	it('renders weight options', () => {
		renderModal();
		expect(screen.getByText('Trivial')).toBeInTheDocument();
		expect(screen.getByText('Small')).toBeInTheDocument();
		expect(screen.getByText('Medium')).toBeInTheDocument();
		expect(screen.getByText('Large')).toBeInTheDocument();
		expect(screen.getByText('Greenfield')).toBeInTheDocument();
	});

	it('renders category options', () => {
		renderModal();
		expect(screen.getByText('Feature')).toBeInTheDocument();
		expect(screen.getByText('Bug')).toBeInTheDocument();
		expect(screen.getByText('Refactor')).toBeInTheDocument();
		expect(screen.getByText('Chore')).toBeInTheDocument();
		expect(screen.getByText('Docs')).toBeInTheDocument();
		expect(screen.getByText('Test')).toBeInTheDocument();
	});

	it('renders priority options', () => {
		renderModal();
		expect(screen.getByText('Critical')).toBeInTheDocument();
		expect(screen.getByText('High')).toBeInTheDocument();
		expect(screen.getByText('Normal')).toBeInTheDocument();
		expect(screen.getByText('Low')).toBeInTheDocument();
	});

	it('renders initiative selector when initiatives exist', () => {
		renderModal();
		expect(screen.getByLabelText(/initiative/i)).toBeInTheDocument();
		// Initiative options are rendered as "{id}: {title}"
		expect(screen.getByText('INIT-001: Test Initiative')).toBeInTheDocument();
	});

	it('renders create button disabled initially', () => {
		renderModal();
		const button = screen.getByRole('button', { name: /create task/i });
		expect(button).toBeDisabled();
	});

	it('enables create button when title is filled', () => {
		renderModal();
		const titleInput = screen.getByLabelText(/title/i);
		fireEvent.change(titleInput, { target: { value: 'New Task Title' } });
		const button = screen.getByRole('button', { name: /create task/i });
		expect(button).not.toBeDisabled();
	});

	it('calls onClose when cancel button is clicked', () => {
		renderModal();
		fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
		expect(mockOnClose).toHaveBeenCalledTimes(1);
	});

	it('renders attachment drop zone', () => {
		renderModal();
		expect(screen.getByText(/drop files here/i)).toBeInTheDocument();
	});

	it('shows keyboard hint', () => {
		renderModal();
		expect(screen.getByText(/enter/i)).toBeInTheDocument();
	});

	it('has required indicator on title field', () => {
		renderModal();
		expect(screen.getByText('*')).toBeInTheDocument();
	});

	it('defaults to medium weight', () => {
		renderModal();
		const mediumButton = screen.getByText('Medium').closest('button');
		expect(mediumButton).toHaveClass('selected');
	});

	it('defaults to feature category', () => {
		renderModal();
		const featureLabel = screen.getByText('Feature').closest('label');
		expect(featureLabel).toHaveClass('selected');
	});

	it('defaults to normal priority', () => {
		renderModal();
		const prioritySelect = screen.getByLabelText(/priority/i) as HTMLSelectElement;
		expect(prioritySelect.value).toBe('normal');
	});

	it('resets form when modal opens', async () => {
		const { rerender } = render(
			<BrowserRouter>
				<NewTaskModal open={true} onClose={mockOnClose} />
			</BrowserRouter>
		);

		// Fill in some data
		const titleInput = screen.getByLabelText(/title/i);
		fireEvent.change(titleInput, { target: { value: 'Test Task' } });
		expect(titleInput).toHaveValue('Test Task');

		// Close modal
		rerender(
			<BrowserRouter>
				<NewTaskModal open={false} onClose={mockOnClose} />
			</BrowserRouter>
		);

		// Reopen modal
		rerender(
			<BrowserRouter>
				<NewTaskModal open={true} onClose={mockOnClose} />
			</BrowserRouter>
		);

		// Should be reset
		await waitFor(() => {
			const newTitleInput = screen.getByLabelText(/title/i);
			expect(newTitleInput).toHaveValue('');
		});
	});

	it('has proper accessibility attributes', () => {
		renderModal();
		const dialog = screen.getByRole('dialog');
		expect(dialog).toHaveAttribute('aria-modal', 'true');
	});
});
