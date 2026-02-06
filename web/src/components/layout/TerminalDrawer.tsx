import { useState, useEffect, useCallback } from 'react';

const STORAGE_KEY = 'orc-terminal-drawer-open';

function getInitialState(): boolean {
	if (typeof window === 'undefined') return false;
	try {
		return localStorage.getItem(STORAGE_KEY) === 'true';
	} catch {
		return false;
	}
}

export function TerminalDrawer() {
	const [isOpen, setIsOpen] = useState(getInitialState);

	// Persist to localStorage
	useEffect(() => {
		try {
			localStorage.setItem(STORAGE_KEY, String(isOpen));
		} catch {
			// Ignore localStorage errors
		}
	}, [isOpen]);

	const toggle = useCallback(() => {
		setIsOpen((prev) => !prev);
	}, []);

	// Keyboard shortcut: Cmd+J / Ctrl+J
	useEffect(() => {
		const handleKeyDown = (e: KeyboardEvent) => {
			if (e.key === 'j' && (e.metaKey || e.ctrlKey)) {
				// Don't toggle if focus is in a text input or textarea
				const target = e.target as HTMLElement;
				if (
					target.tagName === 'INPUT' && (target as HTMLInputElement).type === 'text' ||
					target.tagName === 'TEXTAREA' ||
					target.isContentEditable
				) {
					return;
				}

				e.preventDefault();
				setIsOpen((prev) => !prev);
			}
		};

		document.addEventListener('keydown', handleKeyDown);
		return () => document.removeEventListener('keydown', handleKeyDown);
	}, []);

	const shortcutHint = '⌘J';

	const drawerClasses = ['terminal-drawer', isOpen && 'terminal-drawer--open'].filter(Boolean).join(' ');

	return (
		<div className={drawerClasses} data-testid="terminal-drawer">
			<button
				className="terminal-drawer__toggle"
				onClick={toggle}
				aria-label="Terminal"
			>
				<span className="terminal-drawer__toggle-label">Terminal</span>
				<span className="terminal-drawer__shortcut-hint">{shortcutHint}</span>
			</button>

			{isOpen && (
				<div className="terminal-drawer__content">
					<div className="terminal-drawer__placeholder">
						Terminal emulator will be available in a future update.
					</div>
				</div>
			)}
		</div>
	);
}
