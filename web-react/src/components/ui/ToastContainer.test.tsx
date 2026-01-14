import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { ToastContainer } from './ToastContainer';
import { useUIStore, toast } from '@/stores';

describe('ToastContainer', () => {
	beforeEach(() => {
		// Reset store state before each test
		useUIStore.setState({ toasts: [] });
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('renders nothing when no toasts', () => {
		const { container } = render(<ToastContainer />);
		expect(container.querySelector('.toast-container')).not.toBeInTheDocument();
	});

	it('renders toast container when toasts exist', () => {
		// Add a toast via store
		act(() => {
			toast.success('Test message');
		});

		render(<ToastContainer />);
		// Toast container renders via portal to document.body
		expect(document.body.querySelector('.toast-container')).toBeInTheDocument();
	});

	it('renders success toast with correct styling', () => {
		act(() => {
			toast.success('Success message');
		});

		render(<ToastContainer />);
		expect(screen.getByText('Success message')).toBeInTheDocument();
		expect(screen.getByRole('alert')).toHaveClass('toast-success');
	});

	it('renders error toast with correct styling', () => {
		act(() => {
			toast.error('Error message');
		});

		render(<ToastContainer />);
		expect(screen.getByText('Error message')).toBeInTheDocument();
		expect(screen.getByRole('alert')).toHaveClass('toast-error');
	});

	it('renders warning toast with correct styling', () => {
		act(() => {
			toast.warning('Warning message');
		});

		render(<ToastContainer />);
		expect(screen.getByText('Warning message')).toBeInTheDocument();
		expect(screen.getByRole('alert')).toHaveClass('toast-warning');
	});

	it('renders info toast with correct styling', () => {
		act(() => {
			toast.info('Info message');
		});

		render(<ToastContainer />);
		expect(screen.getByText('Info message')).toBeInTheDocument();
		expect(screen.getByRole('alert')).toHaveClass('toast-info');
	});

	it('renders toast with title', () => {
		act(() => {
			useUIStore.getState().addToast({
				type: 'success',
				message: 'Message',
				title: 'Title',
			});
		});

		render(<ToastContainer />);
		expect(screen.getByText('Title')).toBeInTheDocument();
		expect(screen.getByText('Title')).toHaveClass('toast-title');
	});

	it('renders dismiss button for dismissible toasts', () => {
		act(() => {
			useUIStore.getState().addToast({
				type: 'info',
				message: 'Dismissible',
				dismissible: true,
			});
		});

		render(<ToastContainer />);
		expect(screen.getByRole('button', { name: 'Dismiss notification' })).toBeInTheDocument();
	});

	it('dismisses toast when dismiss button is clicked', () => {
		act(() => {
			toast.info('Dismissible toast');
		});

		render(<ToastContainer />);
		const dismissButton = screen.getByRole('button', { name: 'Dismiss notification' });

		act(() => {
			fireEvent.click(dismissButton);
		});

		expect(screen.queryByText('Dismissible toast')).not.toBeInTheDocument();
	});

	it('renders multiple toasts', () => {
		act(() => {
			toast.success('First');
			toast.error('Second');
			toast.info('Third');
		});

		render(<ToastContainer />);
		expect(screen.getByText('First')).toBeInTheDocument();
		expect(screen.getByText('Second')).toBeInTheDocument();
		expect(screen.getByText('Third')).toBeInTheDocument();
	});

	it('has proper accessibility attributes', () => {
		act(() => {
			toast.success('Accessible toast');
		});

		render(<ToastContainer />);
		expect(screen.getByRole('region', { name: 'Notifications' })).toBeInTheDocument();
		expect(screen.getByRole('alert')).toBeInTheDocument();
	});

	it('auto-dismisses toast after duration', async () => {
		act(() => {
			useUIStore.getState().addToast({
				type: 'success',
				message: 'Auto dismiss',
				duration: 1000,
			});
		});

		render(<ToastContainer />);
		expect(screen.getByText('Auto dismiss')).toBeInTheDocument();

		// Advance timers past the duration
		act(() => {
			vi.advanceTimersByTime(1100);
		});

		expect(screen.queryByText('Auto dismiss')).not.toBeInTheDocument();
	});

	it('renders via portal to document.body', () => {
		act(() => {
			toast.success('Portal toast');
		});

		render(<ToastContainer />);
		// The toast container should be a direct child of body
		const toastContainer = document.body.querySelector('.toast-container');
		expect(toastContainer).toBeInTheDocument();
	});
});
