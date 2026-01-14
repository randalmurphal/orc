import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { useUIStore, toast } from './uiStore';

describe('UIStore', () => {
	beforeEach(() => {
		// Reset store before each test
		useUIStore.getState().reset();
		localStorage.clear();
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	describe('sidebar state', () => {
		it('should default to expanded', () => {
			// Reset doesn't preserve localStorage-based initial state
			// So we test the default without localStorage
			expect(useUIStore.getState().sidebarExpanded).toBe(true);
		});

		it('should toggle sidebar', () => {
			expect(useUIStore.getState().sidebarExpanded).toBe(true);

			useUIStore.getState().toggleSidebar();
			expect(useUIStore.getState().sidebarExpanded).toBe(false);

			useUIStore.getState().toggleSidebar();
			expect(useUIStore.getState().sidebarExpanded).toBe(true);
		});

		it('should set sidebar expanded state', () => {
			useUIStore.getState().setSidebarExpanded(false);
			expect(useUIStore.getState().sidebarExpanded).toBe(false);

			useUIStore.getState().setSidebarExpanded(true);
			expect(useUIStore.getState().sidebarExpanded).toBe(true);
		});

		it('should persist sidebar state to localStorage', () => {
			useUIStore.getState().setSidebarExpanded(false);

			expect(localStorage.getItem('orc-sidebar-expanded')).toBe('false');
		});

		it('should persist sidebar toggle to localStorage', () => {
			useUIStore.getState().toggleSidebar();

			expect(localStorage.getItem('orc-sidebar-expanded')).toBe('false');
		});
	});

	describe('WebSocket status', () => {
		it('should default to disconnected', () => {
			expect(useUIStore.getState().wsStatus).toBe('disconnected');
		});

		it('should update WebSocket status', () => {
			useUIStore.getState().setWsStatus('connecting');
			expect(useUIStore.getState().wsStatus).toBe('connecting');

			useUIStore.getState().setWsStatus('connected');
			expect(useUIStore.getState().wsStatus).toBe('connected');

			useUIStore.getState().setWsStatus('error');
			expect(useUIStore.getState().wsStatus).toBe('error');

			useUIStore.getState().setWsStatus('disconnected');
			expect(useUIStore.getState().wsStatus).toBe('disconnected');
		});
	});

	describe('toast notifications', () => {
		describe('addToast', () => {
			it('should add toast to queue', () => {
				useUIStore.getState().addToast({
					type: 'success',
					message: 'Test message',
				});

				expect(useUIStore.getState().toasts).toHaveLength(1);
				expect(useUIStore.getState().toasts[0].message).toBe('Test message');
			});

			it('should generate unique IDs', () => {
				useUIStore.getState().addToast({ type: 'success', message: 'Toast 1' });
				useUIStore.getState().addToast({ type: 'success', message: 'Toast 2' });

				const ids = useUIStore.getState().toasts.map((t) => t.id);
				expect(new Set(ids).size).toBe(2); // All unique
			});

			it('should return toast ID', () => {
				const id = useUIStore.getState().addToast({
					type: 'success',
					message: 'Test',
				});

				expect(typeof id).toBe('string');
				expect(id.startsWith('toast-')).toBe(true);
			});

			it('should allow custom ID', () => {
				const id = useUIStore.getState().addToast({
					id: 'custom-id',
					type: 'success',
					message: 'Test',
				});

				expect(id).toBe('custom-id');
				expect(useUIStore.getState().toasts[0].id).toBe('custom-id');
			});

			it('should set default duration by type', () => {
				useUIStore.getState().addToast({ type: 'success', message: 'Success' });
				useUIStore.getState().addToast({ type: 'error', message: 'Error' });

				expect(useUIStore.getState().toasts[0].duration).toBe(5000);
				expect(useUIStore.getState().toasts[1].duration).toBe(8000);
			});

			it('should allow custom duration', () => {
				useUIStore.getState().addToast({
					type: 'success',
					message: 'Test',
					duration: 10000,
				});

				expect(useUIStore.getState().toasts[0].duration).toBe(10000);
			});

			it('should set dismissible to true by default', () => {
				useUIStore.getState().addToast({ type: 'success', message: 'Test' });

				expect(useUIStore.getState().toasts[0].dismissible).toBe(true);
			});
		});

		describe('auto-dismiss', () => {
			it('should auto-dismiss after duration', () => {
				useUIStore.getState().addToast({
					type: 'success',
					message: 'Test',
					duration: 1000,
				});

				expect(useUIStore.getState().toasts).toHaveLength(1);

				vi.advanceTimersByTime(1000);

				expect(useUIStore.getState().toasts).toHaveLength(0);
			});

			it('should not auto-dismiss with duration 0', () => {
				useUIStore.getState().addToast({
					type: 'success',
					message: 'Test',
					duration: 0,
				});

				vi.advanceTimersByTime(10000);

				expect(useUIStore.getState().toasts).toHaveLength(1);
			});
		});

		describe('dismissToast', () => {
			it('should remove toast by ID', () => {
				const id = useUIStore.getState().addToast({
					type: 'success',
					message: 'Test',
					duration: 0, // Prevent auto-dismiss
				});

				useUIStore.getState().dismissToast(id);

				expect(useUIStore.getState().toasts).toHaveLength(0);
			});

			it('should not affect other toasts', () => {
				useUIStore.getState().addToast({
					id: 'toast-1',
					type: 'success',
					message: 'Toast 1',
					duration: 0,
				});
				useUIStore.getState().addToast({
					id: 'toast-2',
					type: 'error',
					message: 'Toast 2',
					duration: 0,
				});

				useUIStore.getState().dismissToast('toast-1');

				expect(useUIStore.getState().toasts).toHaveLength(1);
				expect(useUIStore.getState().toasts[0].id).toBe('toast-2');
			});
		});

		describe('clearToasts', () => {
			it('should remove all toasts', () => {
				useUIStore.getState().addToast({ type: 'success', message: '1', duration: 0 });
				useUIStore.getState().addToast({ type: 'error', message: '2', duration: 0 });
				useUIStore.getState().addToast({ type: 'warning', message: '3', duration: 0 });

				useUIStore.getState().clearToasts();

				expect(useUIStore.getState().toasts).toHaveLength(0);
			});
		});

		describe('convenience methods', () => {
			it('should add success toast', () => {
				useUIStore.getState().toast.success('Success message');

				expect(useUIStore.getState().toasts[0].type).toBe('success');
				expect(useUIStore.getState().toasts[0].message).toBe('Success message');
			});

			it('should add error toast', () => {
				useUIStore.getState().toast.error('Error message');

				expect(useUIStore.getState().toasts[0].type).toBe('error');
				expect(useUIStore.getState().toasts[0].message).toBe('Error message');
			});

			it('should add warning toast', () => {
				useUIStore.getState().toast.warning('Warning message');

				expect(useUIStore.getState().toasts[0].type).toBe('warning');
				expect(useUIStore.getState().toasts[0].message).toBe('Warning message');
			});

			it('should add info toast', () => {
				useUIStore.getState().toast.info('Info message');

				expect(useUIStore.getState().toasts[0].type).toBe('info');
				expect(useUIStore.getState().toasts[0].message).toBe('Info message');
			});

			it('should accept options', () => {
				useUIStore.getState().toast.success('Test', {
					title: 'Title',
					duration: 10000,
				});

				expect(useUIStore.getState().toasts[0].title).toBe('Title');
				expect(useUIStore.getState().toasts[0].duration).toBe(10000);
			});
		});

		describe('exported toast object', () => {
			it('should provide success method', () => {
				toast.success('Success via export');

				expect(useUIStore.getState().toasts[0].type).toBe('success');
			});

			it('should provide error method', () => {
				toast.error('Error via export');

				expect(useUIStore.getState().toasts[0].type).toBe('error');
			});

			it('should provide warning method', () => {
				toast.warning('Warning via export');

				expect(useUIStore.getState().toasts[0].type).toBe('warning');
			});

			it('should provide info method', () => {
				toast.info('Info via export');

				expect(useUIStore.getState().toasts[0].type).toBe('info');
			});

			it('should provide dismiss method', () => {
				const id = toast.success('Test');
				toast.dismiss(id);

				// Wait for any pending timers
				vi.runAllTimers();

				// The toast should be dismissed immediately by dismiss()
				// but we need to check after the success duration would have fired
				expect(
					useUIStore.getState().toasts.find((t) => t.id === id)
				).toBeUndefined();
			});

			it('should provide clear method', () => {
				toast.success('1');
				toast.error('2');
				toast.clear();

				expect(useUIStore.getState().toasts).toHaveLength(0);
			});
		});
	});

	describe('reset', () => {
		it('should reset store to initial state', () => {
			useUIStore.getState().setSidebarExpanded(false);
			useUIStore.getState().setWsStatus('connected');
			useUIStore.getState().addToast({ type: 'success', message: 'Test', duration: 0 });

			useUIStore.getState().reset();

			// Note: sidebarExpanded uses localStorage for initial value
			// After reset, it goes back to the initial state (true by default)
			expect(useUIStore.getState().sidebarExpanded).toBe(true);
			expect(useUIStore.getState().wsStatus).toBe('disconnected');
			expect(useUIStore.getState().toasts).toHaveLength(0);
		});
	});
});
