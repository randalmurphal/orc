/**
 * Keyboard Shortcuts Help Modal
 * Displays available keyboard shortcuts organized by category
 */

import { useMemo } from 'react';
import { Modal } from './Modal';
import { isMac } from '@/lib/platform';
import './KeyboardShortcutsHelp.css';

interface KeyboardShortcutsHelpProps {
	open: boolean;
	onClose: () => void;
}

interface ShortcutItem {
	keys: string;
	description: string;
}

interface ShortcutCategory {
	name: string;
	shortcuts: ShortcutItem[];
}

/**
 * Get platform-appropriate modifier display
 * Mac: ⇧⌥ (Shift+Option)
 * Windows/Linux: Shift+Alt+
 */
function getModifier(): string {
	return isMac() ? '⇧⌥' : 'Shift+Alt+';
}

/**
 * Build categories with platform-appropriate keys
 */
function getCategories(): ShortcutCategory[] {
	const mod = getModifier();
	return [
		{
			name: 'Global',
			shortcuts: [
				{ keys: `${mod}K`, description: 'Open command palette' },
				{ keys: `${mod}N`, description: 'Create new task' },
				{ keys: `${mod}B`, description: 'Toggle sidebar' },
				{ keys: `${mod}P`, description: 'Switch project' },
				{ keys: '/', description: 'Focus search' },
				{ keys: '?', description: 'Show this help' },
				{ keys: 'Esc', description: 'Close overlay' },
			],
		},
		{
			name: 'Navigation',
			shortcuts: [
				{ keys: 'g d', description: 'Go to dashboard' },
				{ keys: 'g t', description: 'Go to tasks' },
				{ keys: 'g e', description: 'Go to environment' },
				{ keys: 'g r', description: 'Go to preferences' },
				{ keys: 'g p', description: 'Go to prompts' },
				{ keys: 'g h', description: 'Go to hooks' },
				{ keys: 'g k', description: 'Go to skills' },
			],
		},
		{
			name: 'Task List',
			shortcuts: [
				{ keys: 'j', description: 'Select next task' },
				{ keys: 'k', description: 'Select previous task' },
				{ keys: 'Enter', description: 'Open selected task' },
				{ keys: 'r', description: 'Run selected task' },
				{ keys: 'p', description: 'Pause selected task' },
				{ keys: 'd', description: 'Delete selected task' },
			],
		},
	];
}

export function KeyboardShortcutsHelp({ open, onClose }: KeyboardShortcutsHelpProps) {
	// Memoize categories since they depend on platform detection (which doesn't change)
	const categories = useMemo(() => getCategories(), []);

	return (
		<Modal open={open} onClose={onClose} title="Keyboard Shortcuts" size="md">
			<div className="shortcuts-help">
				{categories.map((category) => (
					<section key={category.name} className="category">
						<h3 className="category-title">{category.name}</h3>
						<div className="shortcuts-list">
							{category.shortcuts.map((shortcut) => (
								<div key={shortcut.keys} className="shortcut-row">
									<div className="shortcut-keys">
										{shortcut.keys.split(' ').map((key, index) => (
											<kbd key={index} className="key">
												{key}
											</kbd>
										))}
									</div>
									<span className="shortcut-description">{shortcut.description}</span>
								</div>
							))}
						</div>
					</section>
				))}
			</div>
		</Modal>
	);
}
