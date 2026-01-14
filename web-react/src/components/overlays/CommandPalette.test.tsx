import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { CommandPalette } from './CommandPalette';

// Mock useNavigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

describe('CommandPalette', () => {
	const mockOnClose = vi.fn();

	const defaultProps = {
		open: true,
		onClose: mockOnClose,
	};

	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
		const portalContent = document.querySelector('.palette-backdrop');
		if (portalContent) {
			portalContent.remove();
		}
	});

	const renderPalette = (props = {}) => {
		return render(
			<BrowserRouter>
				<CommandPalette {...defaultProps} {...props} />
			</BrowserRouter>
		);
	};

	it('renders nothing when open is false', () => {
		render(
			<BrowserRouter>
				<CommandPalette {...defaultProps} open={false} />
			</BrowserRouter>
		);
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('renders dialog when open is true', () => {
		renderPalette();
		expect(screen.getByRole('dialog')).toBeInTheDocument();
	});

	it('renders search input', () => {
		renderPalette();
		expect(screen.getByPlaceholderText(/type a command or search/i)).toBeInTheDocument();
	});

	it('renders command sections', () => {
		renderPalette();
		// Should have some section headings
		expect(screen.getByText('Tasks')).toBeInTheDocument();
		expect(screen.getByText('Navigation')).toBeInTheDocument();
	});

	it('filters commands when searching', async () => {
		renderPalette();
		const searchInput = screen.getByPlaceholderText(/type a command or search/i);
		fireEvent.change(searchInput, { target: { value: 'dashboard' } });

		await waitFor(() => {
			// The text may have <mark> tags from highlighting, so look for the item-label that contains the text
			const dashboardItem = document.querySelector('.item-label');
			expect(dashboardItem?.textContent).toContain('Dashboard');
		});
	});

	it('shows no results message when search has no matches', async () => {
		renderPalette();
		const searchInput = screen.getByPlaceholderText(/type a command or search/i);
		fireEvent.change(searchInput, { target: { value: 'xyznonexistent' } });

		await waitFor(() => {
			expect(screen.getByText(/no commands found/i)).toBeInTheDocument();
		});
	});

	it('calls onClose when Escape key is pressed on input', () => {
		renderPalette();
		const searchInput = screen.getByPlaceholderText(/type a command or search/i);
		fireEvent.keyDown(searchInput, { key: 'Escape' });
		expect(mockOnClose).toHaveBeenCalledTimes(1);
	});

	it('calls onClose when backdrop is clicked', () => {
		renderPalette();
		const backdrop = document.querySelector('.palette-backdrop');
		fireEvent.click(backdrop!);
		expect(mockOnClose).toHaveBeenCalledTimes(1);
	});

	it('navigates with keyboard arrows', async () => {
		renderPalette();
		const searchInput = screen.getByPlaceholderText(/type a command or search/i);

		// Press down arrow
		fireEvent.keyDown(searchInput, { key: 'ArrowDown' });

		// Second item should be selected (first starts selected at index 0)
		await waitFor(() => {
			const selectedItem = document.querySelector('.result-item.selected');
			expect(selectedItem).toBeInTheDocument();
		});
	});

	it('executes command on Enter', async () => {
		renderPalette();
		const searchInput = screen.getByPlaceholderText(/type a command or search/i);

		// First item is selected by default (New Task)
		// Press Enter
		fireEvent.keyDown(searchInput, { key: 'Enter' });

		// Should close palette
		await waitFor(() => {
			expect(mockOnClose).toHaveBeenCalled();
		});
	});

	it('dispatches new-task event when New Task command is executed', async () => {
		const eventHandler = vi.fn();
		window.addEventListener('orc:new-task', eventHandler);

		renderPalette();
		const searchInput = screen.getByPlaceholderText(/type a command or search/i);
		fireEvent.change(searchInput, { target: { value: 'new task' } });

		await waitFor(() => {
			expect(screen.getByText('New Task')).toBeInTheDocument();
		});

		// Click on the command
		fireEvent.click(screen.getByText('New Task').closest('.result-item')!);

		expect(eventHandler).toHaveBeenCalled();
		expect(mockOnClose).toHaveBeenCalledTimes(1);

		window.removeEventListener('orc:new-task', eventHandler);
	});

	it('navigates to dashboard when navigation command is executed', async () => {
		renderPalette();
		const searchInput = screen.getByPlaceholderText(/type a command or search/i);
		fireEvent.change(searchInput, { target: { value: 'dashboard' } });

		await waitFor(() => {
			// The text may have <mark> tags from highlighting, so look for the item-label
			const dashboardLabel = document.querySelector('.item-label');
			expect(dashboardLabel?.textContent).toContain('Dashboard');
		});

		// Click on the result item (first one after filtering)
		const resultItem = document.querySelector('.result-item');
		fireEvent.click(resultItem!);

		expect(mockNavigate).toHaveBeenCalledWith('/dashboard');
		expect(mockOnClose).toHaveBeenCalledTimes(1);
	});

	it('shows keyboard shortcuts in command items', () => {
		renderPalette();
		// Some commands should have keyboard shortcuts displayed
		// Look for kbd elements
		const kbdElements = document.querySelectorAll('kbd');
		expect(kbdElements.length).toBeGreaterThan(0);
	});

	it('has proper accessibility attributes', () => {
		renderPalette();
		const dialog = screen.getByRole('dialog');
		expect(dialog).toHaveAttribute('aria-modal', 'true');
	});

	it('focuses search input on open', async () => {
		renderPalette();
		await waitFor(() => {
			expect(screen.getByPlaceholderText(/type a command or search/i)).toHaveFocus();
		});
	});
});
