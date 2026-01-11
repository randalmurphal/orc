/**
 * Keyboard Shortcut Manager
 * Handles global and context-specific keyboard shortcuts
 */

export interface Shortcut {
	key: string;
	modifiers?: readonly ('ctrl' | 'meta' | 'shift' | 'alt')[];
	description: string;
	action: () => void;
	context?: 'global' | 'tasks' | 'editor';
}

export interface ShortcutSequence {
	keys: string[];
	description: string;
	action: () => void;
	context?: 'global' | 'tasks' | 'editor';
}

type ShortcutCallback = () => void;

class ShortcutManager {
	private shortcuts: Map<string, Shortcut> = new Map();
	private sequences: ShortcutSequence[] = [];
	private sequenceBuffer: string[] = [];
	private sequenceTimeout: ReturnType<typeof setTimeout> | null = null;
	private enabled = true;
	private currentContext: 'global' | 'tasks' | 'editor' = 'global';

	constructor() {
		if (typeof window !== 'undefined') {
			window.addEventListener('keydown', this.handleKeydown.bind(this));
		}
	}

	/**
	 * Register a single-key shortcut
	 */
	register(shortcut: Shortcut): () => void {
		const key = this.normalizeKey(shortcut.key, shortcut.modifiers);
		this.shortcuts.set(key, shortcut);
		return () => this.shortcuts.delete(key);
	}

	/**
	 * Register a key sequence (e.g., 'g d' for go to dashboard)
	 */
	registerSequence(sequence: ShortcutSequence): () => void {
		this.sequences.push(sequence);
		return () => {
			const idx = this.sequences.indexOf(sequence);
			if (idx !== -1) this.sequences.splice(idx, 1);
		};
	}

	/**
	 * Set current context (affects which shortcuts are active)
	 */
	setContext(context: 'global' | 'tasks' | 'editor'): void {
		this.currentContext = context;
	}

	/**
	 * Enable/disable all shortcuts
	 */
	setEnabled(enabled: boolean): void {
		this.enabled = enabled;
	}

	/**
	 * Get all registered shortcuts for display in help modal
	 */
	getShortcuts(): { key: string; description: string; context: string }[] {
		const result: { key: string; description: string; context: string }[] = [];

		// Single key shortcuts
		for (const [key, shortcut] of this.shortcuts) {
			result.push({
				key: this.formatKey(key),
				description: shortcut.description,
				context: shortcut.context || 'global'
			});
		}

		// Sequences
		for (const seq of this.sequences) {
			result.push({
				key: seq.keys.join(' '),
				description: seq.description,
				context: seq.context || 'global'
			});
		}

		return result;
	}

	private handleKeydown(e: KeyboardEvent): void {
		if (!this.enabled) return;

		// Skip if user is typing in an input
		const target = e.target as HTMLElement;
		if (this.isInputElement(target) && e.key !== 'Escape') {
			return;
		}

		// Build the key string
		const key = this.getKeyString(e);

		// Check for sequence matches first
		if (this.handleSequence(key)) {
			e.preventDefault();
			return;
		}

		// Check for single shortcuts
		const normalizedKey = this.normalizeKey(
			e.key.toLowerCase(),
			this.getModifiers(e)
		);

		const shortcut = this.shortcuts.get(normalizedKey);
		if (shortcut && this.matchesContext(shortcut.context)) {
			e.preventDefault();
			shortcut.action();
		}
	}

	private handleSequence(key: string): boolean {
		// Add key to buffer
		this.sequenceBuffer.push(key);

		// Clear previous timeout
		if (this.sequenceTimeout) {
			clearTimeout(this.sequenceTimeout);
		}

		// Check for matching sequence
		const bufferStr = this.sequenceBuffer.join(' ');
		for (const seq of this.sequences) {
			if (!this.matchesContext(seq.context)) continue;

			const seqStr = seq.keys.join(' ');
			if (seqStr === bufferStr) {
				this.clearSequenceBuffer();
				seq.action();
				return true;
			}

			// Check if this could be a partial match
			if (seqStr.startsWith(bufferStr)) {
				// Set timeout to clear buffer if no more keys
				this.sequenceTimeout = setTimeout(() => {
					this.clearSequenceBuffer();
				}, 1000);
				return false;
			}
		}

		// No match found, clear buffer
		this.clearSequenceBuffer();
		return false;
	}

	private clearSequenceBuffer(): void {
		this.sequenceBuffer = [];
		if (this.sequenceTimeout) {
			clearTimeout(this.sequenceTimeout);
			this.sequenceTimeout = null;
		}
	}

	private matchesContext(context?: string): boolean {
		if (!context || context === 'global') return true;
		return context === this.currentContext;
	}

	private isInputElement(el: HTMLElement): boolean {
		return (
			el.tagName === 'INPUT' ||
			el.tagName === 'TEXTAREA' ||
			el.isContentEditable
		);
	}

	private getKeyString(e: KeyboardEvent): string {
		return e.key.toLowerCase();
	}

	private getModifiers(e: KeyboardEvent): ('ctrl' | 'meta' | 'shift' | 'alt')[] {
		const mods: ('ctrl' | 'meta' | 'shift' | 'alt')[] = [];
		if (e.ctrlKey) mods.push('ctrl');
		if (e.metaKey) mods.push('meta');
		if (e.shiftKey) mods.push('shift');
		if (e.altKey) mods.push('alt');
		return mods;
	}

	private normalizeKey(key: string, modifiers?: readonly ('ctrl' | 'meta' | 'shift' | 'alt')[]): string {
		const sortedMods = modifiers ? [...modifiers].sort() : [];
		const parts = [...sortedMods, key.toLowerCase()];
		return parts.join('+');
	}

	private formatKey(key: string): string {
		const parts = key.split('+');
		return parts
			.map((p) => {
				switch (p) {
					case 'meta':
						return '⌘';
					case 'ctrl':
						return 'Ctrl';
					case 'shift':
						return '⇧';
					case 'alt':
						return '⌥';
					default:
						return p.toUpperCase();
				}
			})
			.join(' + ');
	}

	/**
	 * Clean up event listeners
	 */
	destroy(): void {
		if (typeof window !== 'undefined') {
			window.removeEventListener('keydown', this.handleKeydown.bind(this));
		}
	}
}

// Singleton instance
let instance: ShortcutManager | null = null;

export function getShortcutManager(): ShortcutManager {
	if (!instance) {
		instance = new ShortcutManager();
	}
	return instance;
}

/**
 * Pre-defined shortcut definitions
 */
export const SHORTCUTS = {
	// Global shortcuts
	COMMAND_PALETTE: { key: 'k', modifiers: ['meta'] as const, description: 'Open command palette' },
	NEW_TASK: { key: 'n', modifiers: ['meta'] as const, description: 'Create new task' },
	TOGGLE_SIDEBAR: { key: 'b', modifiers: ['meta'] as const, description: 'Toggle sidebar' },
	SEARCH: { key: '/', description: 'Focus search' },
	HELP: { key: '?', description: 'Show keyboard shortcuts' },
	ESCAPE: { key: 'escape', description: 'Close overlay / Cancel' },

	// Navigation sequences
	GO_DASHBOARD: { keys: ['g', 'd'], description: 'Go to dashboard' },
	GO_TASKS: { keys: ['g', 't'], description: 'Go to tasks' },
	GO_SETTINGS: { keys: ['g', 's'], description: 'Go to settings' },
	GO_PROMPTS: { keys: ['g', 'p'], description: 'Go to prompts' },
	GO_HOOKS: { keys: ['g', 'h'], description: 'Go to hooks' },
	GO_SKILLS: { keys: ['g', 'k'], description: 'Go to skills' },

	// Task list shortcuts
	TASK_NAV_DOWN: { key: 'j', context: 'tasks' as const, description: 'Select next task' },
	TASK_NAV_UP: { key: 'k', context: 'tasks' as const, description: 'Select previous task' },
	TASK_OPEN: { key: 'enter', context: 'tasks' as const, description: 'Open selected task' },
	TASK_RUN: { key: 'r', context: 'tasks' as const, description: 'Run selected task' },
	TASK_PAUSE: { key: 'p', context: 'tasks' as const, description: 'Pause selected task' },
	TASK_DELETE: { key: 'd', context: 'tasks' as const, description: 'Delete selected task' }
};

/**
 * Helper to setup common shortcuts
 */
export function setupGlobalShortcuts(callbacks: {
	onCommandPalette?: ShortcutCallback;
	onNewTask?: ShortcutCallback;
	onToggleSidebar?: ShortcutCallback;
	onSearch?: ShortcutCallback;
	onHelp?: ShortcutCallback;
	onEscape?: ShortcutCallback;
	onGoDashboard?: ShortcutCallback;
	onGoTasks?: ShortcutCallback;
	onGoSettings?: ShortcutCallback;
	onGoPrompts?: ShortcutCallback;
	onGoHooks?: ShortcutCallback;
	onGoSkills?: ShortcutCallback;
}): () => void {
	const manager = getShortcutManager();
	const unsubscribers: (() => void)[] = [];

	// Single-key shortcuts
	if (callbacks.onCommandPalette) {
		unsubscribers.push(
			manager.register({
				...SHORTCUTS.COMMAND_PALETTE,
				action: callbacks.onCommandPalette
			})
		);
	}

	if (callbacks.onNewTask) {
		unsubscribers.push(
			manager.register({
				...SHORTCUTS.NEW_TASK,
				action: callbacks.onNewTask
			})
		);
	}

	if (callbacks.onToggleSidebar) {
		unsubscribers.push(
			manager.register({
				...SHORTCUTS.TOGGLE_SIDEBAR,
				action: callbacks.onToggleSidebar
			})
		);
	}

	if (callbacks.onSearch) {
		unsubscribers.push(
			manager.register({
				...SHORTCUTS.SEARCH,
				action: callbacks.onSearch
			})
		);
	}

	if (callbacks.onHelp) {
		unsubscribers.push(
			manager.register({
				...SHORTCUTS.HELP,
				action: callbacks.onHelp
			})
		);
	}

	if (callbacks.onEscape) {
		unsubscribers.push(
			manager.register({
				...SHORTCUTS.ESCAPE,
				action: callbacks.onEscape
			})
		);
	}

	// Navigation sequences
	if (callbacks.onGoDashboard) {
		unsubscribers.push(
			manager.registerSequence({
				...SHORTCUTS.GO_DASHBOARD,
				action: callbacks.onGoDashboard
			})
		);
	}

	if (callbacks.onGoTasks) {
		unsubscribers.push(
			manager.registerSequence({
				...SHORTCUTS.GO_TASKS,
				action: callbacks.onGoTasks
			})
		);
	}

	if (callbacks.onGoSettings) {
		unsubscribers.push(
			manager.registerSequence({
				...SHORTCUTS.GO_SETTINGS,
				action: callbacks.onGoSettings
			})
		);
	}

	if (callbacks.onGoPrompts) {
		unsubscribers.push(
			manager.registerSequence({
				...SHORTCUTS.GO_PROMPTS,
				action: callbacks.onGoPrompts
			})
		);
	}

	if (callbacks.onGoHooks) {
		unsubscribers.push(
			manager.registerSequence({
				...SHORTCUTS.GO_HOOKS,
				action: callbacks.onGoHooks
			})
		);
	}

	if (callbacks.onGoSkills) {
		unsubscribers.push(
			manager.registerSequence({
				...SHORTCUTS.GO_SKILLS,
				action: callbacks.onGoSkills
			})
		);
	}

	return () => {
		unsubscribers.forEach((unsub) => unsub());
	};
}

/**
 * Helper to setup task list shortcuts
 */
export function setupTaskListShortcuts(callbacks: {
	onNavDown?: ShortcutCallback;
	onNavUp?: ShortcutCallback;
	onOpen?: ShortcutCallback;
	onRun?: ShortcutCallback;
	onPause?: ShortcutCallback;
	onDelete?: ShortcutCallback;
}): () => void {
	const manager = getShortcutManager();
	manager.setContext('tasks');
	const unsubscribers: (() => void)[] = [];

	if (callbacks.onNavDown) {
		unsubscribers.push(
			manager.register({
				...SHORTCUTS.TASK_NAV_DOWN,
				action: callbacks.onNavDown
			})
		);
	}

	if (callbacks.onNavUp) {
		unsubscribers.push(
			manager.register({
				...SHORTCUTS.TASK_NAV_UP,
				action: callbacks.onNavUp
			})
		);
	}

	if (callbacks.onOpen) {
		unsubscribers.push(
			manager.register({
				...SHORTCUTS.TASK_OPEN,
				action: callbacks.onOpen
			})
		);
	}

	if (callbacks.onRun) {
		unsubscribers.push(
			manager.register({
				...SHORTCUTS.TASK_RUN,
				action: callbacks.onRun
			})
		);
	}

	if (callbacks.onPause) {
		unsubscribers.push(
			manager.register({
				...SHORTCUTS.TASK_PAUSE,
				action: callbacks.onPause
			})
		);
	}

	if (callbacks.onDelete) {
		unsubscribers.push(
			manager.register({
				...SHORTCUTS.TASK_DELETE,
				action: callbacks.onDelete
			})
		);
	}

	return () => {
		manager.setContext('global');
		unsubscribers.forEach((unsub) => unsub());
	};
}
