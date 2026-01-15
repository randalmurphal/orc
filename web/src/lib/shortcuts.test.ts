import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
	ShortcutManager,
	getShortcutManager,
	resetShortcutManager,
	setupGlobalShortcuts,
	setupTaskListShortcuts,
	SHORTCUTS,
} from './shortcuts';

describe('ShortcutManager', () => {
	let manager: ShortcutManager;

	beforeEach(() => {
		// Reset singleton and create fresh manager
		resetShortcutManager();
		manager = getShortcutManager();
	});

	afterEach(() => {
		resetShortcutManager();
	});

	describe('singleton', () => {
		it('should return the same instance', () => {
			const manager1 = getShortcutManager();
			const manager2 = getShortcutManager();
			expect(manager1).toBe(manager2);
		});

		it('should create new instance after reset', () => {
			const manager1 = getShortcutManager();
			resetShortcutManager();
			const manager2 = getShortcutManager();
			expect(manager1).not.toBe(manager2);
		});
	});

	describe('register', () => {
		it('should register a shortcut', () => {
			const action = vi.fn();
			manager.register({
				key: 'k',
				description: 'Test',
				action,
			});

			// Dispatch keydown event
			const event = new KeyboardEvent('keydown', { key: 'k' });
			window.dispatchEvent(event);

			expect(action).toHaveBeenCalled();
		});

		it('should return unsubscribe function', () => {
			const action = vi.fn();
			const unsubscribe = manager.register({
				key: 'k',
				description: 'Test',
				action,
			});

			unsubscribe();

			// Dispatch keydown event
			const event = new KeyboardEvent('keydown', { key: 'k' });
			window.dispatchEvent(event);

			expect(action).not.toHaveBeenCalled();
		});

		it('should register shortcut with modifiers', () => {
			const action = vi.fn();
			manager.register({
				key: 'k',
				modifiers: ['shift', 'alt'],
				description: 'Test',
				action,
			});

			// Without modifiers - should not trigger
			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k' }));
			expect(action).not.toHaveBeenCalled();

			// With modifiers - should trigger
			window.dispatchEvent(
				new KeyboardEvent('keydown', { key: 'k', shiftKey: true, altKey: true })
			);
			expect(action).toHaveBeenCalled();
		});

		it('should handle shifted characters like ? without double-counting shift', () => {
			const action = vi.fn();
			manager.register({
				key: '?',
				description: 'Help',
				action,
			});

			// Pressing Shift+/ produces '?' key
			window.dispatchEvent(
				new KeyboardEvent('keydown', { key: '?', shiftKey: true })
			);
			expect(action).toHaveBeenCalled();
		});
	});

	describe('registerSequence', () => {
		beforeEach(() => {
			vi.useFakeTimers();
		});

		afterEach(() => {
			vi.useRealTimers();
		});

		it('should register a key sequence', () => {
			const action = vi.fn();
			manager.registerSequence({
				keys: ['g', 'd'],
				description: 'Go to dashboard',
				action,
			});

			// Press 'g'
			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }));
			expect(action).not.toHaveBeenCalled();

			// Press 'd' within timeout
			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'd' }));
			expect(action).toHaveBeenCalled();
		});

		it('should not trigger sequence if timeout expires', () => {
			const action = vi.fn();
			manager.registerSequence({
				keys: ['g', 'd'],
				description: 'Go to dashboard',
				action,
			});

			// Press 'g'
			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }));

			// Wait for timeout (1 second)
			vi.advanceTimersByTime(1000);

			// Press 'd' after timeout
			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'd' }));
			expect(action).not.toHaveBeenCalled();
		});

		it('should return unsubscribe function', () => {
			const action = vi.fn();
			const unsubscribe = manager.registerSequence({
				keys: ['g', 'd'],
				description: 'Go to dashboard',
				action,
			});

			unsubscribe();

			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }));
			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'd' }));

			expect(action).not.toHaveBeenCalled();
		});
	});

	describe('context', () => {
		it('should filter shortcuts by context', () => {
			const globalAction = vi.fn();
			const tasksAction = vi.fn();

			manager.register({
				key: 'g',
				description: 'Global',
				action: globalAction,
				context: 'global',
			});

			manager.register({
				key: 'j',
				description: 'Tasks',
				action: tasksAction,
				context: 'tasks',
			});

			// In global context
			manager.setContext('global');
			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }));
			expect(globalAction).toHaveBeenCalled();

			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'j' }));
			expect(tasksAction).not.toHaveBeenCalled();

			// In tasks context
			manager.setContext('tasks');
			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'j' }));
			expect(tasksAction).toHaveBeenCalled();
		});

		it('should get current context', () => {
			expect(manager.getContext()).toBe('global');
			manager.setContext('tasks');
			expect(manager.getContext()).toBe('tasks');
		});
	});

	describe('enabled state', () => {
		it('should not trigger when disabled', () => {
			const action = vi.fn();
			manager.register({
				key: 'k',
				description: 'Test',
				action,
			});

			manager.setEnabled(false);
			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k' }));

			expect(action).not.toHaveBeenCalled();
		});

		it('should trigger when re-enabled', () => {
			const action = vi.fn();
			manager.register({
				key: 'k',
				description: 'Test',
				action,
			});

			manager.setEnabled(false);
			manager.setEnabled(true);
			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k' }));

			expect(action).toHaveBeenCalled();
		});

		it('should check enabled state', () => {
			expect(manager.isEnabled()).toBe(true);
			manager.setEnabled(false);
			expect(manager.isEnabled()).toBe(false);
		});
	});

	describe('input element handling', () => {
		it('should not trigger shortcuts when typing in input', () => {
			const action = vi.fn();
			manager.register({
				key: 'k',
				description: 'Test',
				action,
			});

			const input = document.createElement('input');
			document.body.appendChild(input);

			const event = new KeyboardEvent('keydown', {
				key: 'k',
				bubbles: true,
			});
			Object.defineProperty(event, 'target', { value: input });
			window.dispatchEvent(event);

			expect(action).not.toHaveBeenCalled();
			document.body.removeChild(input);
		});

		it('should not trigger shortcuts when typing in textarea', () => {
			const action = vi.fn();
			manager.register({
				key: 'k',
				description: 'Test',
				action,
			});

			const textarea = document.createElement('textarea');
			document.body.appendChild(textarea);

			const event = new KeyboardEvent('keydown', {
				key: 'k',
				bubbles: true,
			});
			Object.defineProperty(event, 'target', { value: textarea });
			window.dispatchEvent(event);

			expect(action).not.toHaveBeenCalled();
			document.body.removeChild(textarea);
		});

		it('should always trigger Escape even in input', () => {
			const action = vi.fn();
			manager.register({
				key: 'escape',
				description: 'Close',
				action,
			});

			const input = document.createElement('input');
			document.body.appendChild(input);

			const event = new KeyboardEvent('keydown', {
				key: 'Escape',
				bubbles: true,
			});
			Object.defineProperty(event, 'target', { value: input });
			window.dispatchEvent(event);

			expect(action).toHaveBeenCalled();
			document.body.removeChild(input);
		});
	});

	describe('getShortcuts', () => {
		it('should return registered shortcuts', () => {
			manager.register({
				key: 'k',
				modifiers: ['shift', 'alt'],
				description: 'Command palette',
				action: vi.fn(),
			});

			manager.registerSequence({
				keys: ['g', 'd'],
				description: 'Go to dashboard',
				action: vi.fn(),
			});

			const shortcuts = manager.getShortcuts();
			expect(shortcuts).toHaveLength(2);
			expect(shortcuts[0].description).toBe('Command palette');
			expect(shortcuts[1].description).toBe('Go to dashboard');
			expect(shortcuts[1].key).toBe('g d');
		});
	});

	describe('destroy', () => {
		it('should remove event listener', () => {
			const action = vi.fn();
			manager.register({
				key: 'k',
				description: 'Test',
				action,
			});

			manager.destroy();

			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k' }));
			expect(action).not.toHaveBeenCalled();
		});
	});
});

describe('SHORTCUTS constants', () => {
	it('should have global shortcuts with Shift+Alt modifiers', () => {
		expect(SHORTCUTS.COMMAND_PALETTE.modifiers).toEqual(['shift', 'alt']);
		expect(SHORTCUTS.NEW_TASK.modifiers).toEqual(['shift', 'alt']);
		expect(SHORTCUTS.TOGGLE_SIDEBAR.modifiers).toEqual(['shift', 'alt']);
		expect(SHORTCUTS.PROJECT_SWITCHER.modifiers).toEqual(['shift', 'alt']);
	});

	it('should have navigation sequences', () => {
		expect(SHORTCUTS.GO_DASHBOARD.keys).toEqual(['g', 'd']);
		expect(SHORTCUTS.GO_TASKS.keys).toEqual(['g', 't']);
		expect(SHORTCUTS.GO_ENVIRONMENT.keys).toEqual(['g', 'e']);
	});

	it('should have task context shortcuts', () => {
		expect(SHORTCUTS.TASK_NAV_DOWN.context).toBe('tasks');
		expect(SHORTCUTS.TASK_NAV_UP.context).toBe('tasks');
		expect(SHORTCUTS.TASK_OPEN.context).toBe('tasks');
	});
});

describe('setupGlobalShortcuts', () => {
	beforeEach(() => {
		resetShortcutManager();
	});

	afterEach(() => {
		resetShortcutManager();
	});

	it('should register command palette shortcut', () => {
		const onCommandPalette = vi.fn();
		setupGlobalShortcuts({ onCommandPalette });

		window.dispatchEvent(
			new KeyboardEvent('keydown', { key: 'k', shiftKey: true, altKey: true })
		);

		expect(onCommandPalette).toHaveBeenCalled();
	});

	it('should register help shortcut', () => {
		const onHelp = vi.fn();
		setupGlobalShortcuts({ onHelp });

		window.dispatchEvent(
			new KeyboardEvent('keydown', { key: '?', shiftKey: true })
		);

		expect(onHelp).toHaveBeenCalled();
	});

	it('should return cleanup function', () => {
		const onHelp = vi.fn();
		const cleanup = setupGlobalShortcuts({ onHelp });

		cleanup();

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

		it('should register go to dashboard sequence', () => {
			const onGoDashboard = vi.fn();
			setupGlobalShortcuts({ onGoDashboard });

			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }));
			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'd' }));

			expect(onGoDashboard).toHaveBeenCalled();
		});

		it('should register go to tasks sequence', () => {
			const onGoTasks = vi.fn();
			setupGlobalShortcuts({ onGoTasks });

			window.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }));
			window.dispatchEvent(new KeyboardEvent('keydown', { key: 't' }));

			expect(onGoTasks).toHaveBeenCalled();
		});
	});
});

describe('setupTaskListShortcuts', () => {
	beforeEach(() => {
		resetShortcutManager();
	});

	afterEach(() => {
		resetShortcutManager();
	});

	it('should set context to tasks', () => {
		setupTaskListShortcuts({});
		expect(getShortcutManager().getContext()).toBe('tasks');
	});

	it('should register navigation shortcuts', () => {
		const onNavDown = vi.fn();
		const onNavUp = vi.fn();
		setupTaskListShortcuts({ onNavDown, onNavUp });

		window.dispatchEvent(new KeyboardEvent('keydown', { key: 'j' }));
		expect(onNavDown).toHaveBeenCalled();

		window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k' }));
		expect(onNavUp).toHaveBeenCalled();
	});

	it('should register action shortcuts', () => {
		const onOpen = vi.fn();
		const onRun = vi.fn();
		setupTaskListShortcuts({ onOpen, onRun });

		window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
		expect(onOpen).toHaveBeenCalled();

		window.dispatchEvent(new KeyboardEvent('keydown', { key: 'r' }));
		expect(onRun).toHaveBeenCalled();
	});

	it('should reset context to global on cleanup', () => {
		const cleanup = setupTaskListShortcuts({});

		expect(getShortcutManager().getContext()).toBe('tasks');

		cleanup();

		expect(getShortcutManager().getContext()).toBe('global');
	});
});
