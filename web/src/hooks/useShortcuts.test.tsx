import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ShortcutProvider, useShortcuts, useShortcutContext, useGlobalShortcuts } from './useShortcuts';
import { resetShortcutManager, getShortcutManager } from '@/lib/shortcuts';
import type { ReactNode } from 'react';

// Wrapper component for hooks
function createWrapper() {
	return function Wrapper({ children }: { children: ReactNode }) {
		return (
			<MemoryRouter>
				<ShortcutProvider>{children}</ShortcutProvider>
			</MemoryRouter>
		);
	};
}

describe('ShortcutProvider', () => {
	beforeEach(() => {
		resetShortcutManager();
	});

	afterEach(() => {
		resetShortcutManager();
	});

	it('should provide shortcut context', () => {
		const { result } = renderHook(() => useShortcuts(), {
			wrapper: createWrapper(),
		});

		expect(result.current).toBeDefined();
		expect(typeof result.current.registerShortcut).toBe('function');
		expect(typeof result.current.registerSequence).toBe('function');
		expect(typeof result.current.setContext).toBe('function');
		expect(typeof result.current.getContext).toBe('function');
		expect(typeof result.current.setEnabled).toBe('function');
		expect(typeof result.current.isEnabled).toBe('function');
		expect(typeof result.current.getShortcuts).toBe('function');
	});
});

describe('useShortcuts', () => {
	beforeEach(() => {
		resetShortcutManager();
	});

	afterEach(() => {
		resetShortcutManager();
	});

	it('should throw if used outside ShortcutProvider', () => {
		// Suppress console.error for this test
		const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

		expect(() => {
			renderHook(() => useShortcuts());
		}).toThrow('useShortcuts must be used within a ShortcutProvider');

		consoleSpy.mockRestore();
	});

	it('should register and unregister shortcuts', () => {
		const action = vi.fn();
		const { result } = renderHook(() => useShortcuts(), {
			wrapper: createWrapper(),
		});

		let unsubscribe: () => void;
		act(() => {
			unsubscribe = result.current.registerShortcut({
				key: 'k',
				description: 'Test',
				action,
			});
		});

		// Trigger shortcut
		window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k' }));
		expect(action).toHaveBeenCalledTimes(1);

		// Unsubscribe
		act(() => {
			unsubscribe();
		});

		// Should not trigger after unsubscribe
		window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k' }));
		expect(action).toHaveBeenCalledTimes(1);
	});

	it('should manage context', () => {
		const { result } = renderHook(() => useShortcuts(), {
			wrapper: createWrapper(),
		});

		expect(result.current.getContext()).toBe('global');

		act(() => {
			result.current.setContext('tasks');
		});

		expect(result.current.getContext()).toBe('tasks');
	});

	it('should manage enabled state', () => {
		const { result } = renderHook(() => useShortcuts(), {
			wrapper: createWrapper(),
		});

		expect(result.current.isEnabled()).toBe(true);

		act(() => {
			result.current.setEnabled(false);
		});

		expect(result.current.isEnabled()).toBe(false);
	});
});

describe('useShortcutContext', () => {
	beforeEach(() => {
		resetShortcutManager();
	});

	afterEach(() => {
		resetShortcutManager();
	});

	it('should set context on mount', () => {
		renderHook(() => useShortcutContext('tasks'), {
			wrapper: createWrapper(),
		});

		expect(getShortcutManager().getContext()).toBe('tasks');
	});

	it('should reset context to global on unmount', () => {
		const { unmount } = renderHook(() => useShortcutContext('tasks'), {
			wrapper: createWrapper(),
		});

		expect(getShortcutManager().getContext()).toBe('tasks');

		unmount();

		expect(getShortcutManager().getContext()).toBe('global');
	});
});

describe('useGlobalShortcuts', () => {
	beforeEach(() => {
		resetShortcutManager();
	});

	afterEach(() => {
		resetShortcutManager();
	});

	it('should register help shortcut', () => {
		const onHelp = vi.fn();
		renderHook(() => useGlobalShortcuts({ onHelp }), {
			wrapper: createWrapper(),
		});

		window.dispatchEvent(
			new KeyboardEvent('keydown', { key: '?', shiftKey: true })
		);

		expect(onHelp).toHaveBeenCalled();
	});

	it('should register escape shortcut', () => {
		const onEscape = vi.fn();
		renderHook(() => useGlobalShortcuts({ onEscape }), {
			wrapper: createWrapper(),
		});

		window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));

		expect(onEscape).toHaveBeenCalled();
	});

	it('should clean up shortcuts on unmount', () => {
		const onHelp = vi.fn();
		const { unmount } = renderHook(() => useGlobalShortcuts({ onHelp }), {
			wrapper: createWrapper(),
		});

		unmount();

		window.dispatchEvent(
			new KeyboardEvent('keydown', { key: '?', shiftKey: true })
		);

		expect(onHelp).not.toHaveBeenCalled();
	});

	describe('navigation sequences', () => {
		beforeEach(() => {
			vi.useFakeTimers();
		});

		afterEach(() => {
			vi.useRealTimers();
		});

		it('should register navigation sequences', () => {
			// Navigation callbacks are handled internally via useNavigate
			// We can verify the manager has the sequences registered
			renderHook(() => useGlobalShortcuts({}), {
				wrapper: createWrapper(),
			});

			const shortcuts = getShortcutManager().getShortcuts();
			const dashboardShortcut = shortcuts.find((s) => s.key === 'g d');
			const tasksShortcut = shortcuts.find((s) => s.key === 'g t');

			expect(dashboardShortcut).toBeDefined();
			expect(tasksShortcut).toBeDefined();
		});
	});
});
