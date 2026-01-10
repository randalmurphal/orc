/**
 * Platform detection utilities for keyboard shortcuts
 */

/**
 * Detects if the user is on a Mac platform
 * Uses userAgentData if available, falls back to userAgent string
 */
export function isMac(): boolean {
	if (typeof navigator === 'undefined') return false;

	// Try modern userAgentData API first
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	const nav = navigator as any;
	if (nav.userAgentData?.platform) {
		return nav.userAgentData.platform.toLowerCase() === 'macos';
	}

	// Fall back to userAgent string parsing
	return /Mac|iPod|iPhone|iPad/.test(navigator.userAgent);
}

/**
 * Returns the appropriate modifier key symbol for the platform
 * Mac: ⌘ (Command)
 * Others: Ctrl
 */
export function getModifierKey(): string {
	return isMac() ? '⌘' : 'Ctrl';
}

/**
 * Returns the appropriate modifier key symbol (short form)
 * Mac: ⌘
 * Others: ^
 */
export function getModifierSymbol(): string {
	return isMac() ? '⌘' : '^';
}

/**
 * Formats a keyboard shortcut for display
 * @param key The key (e.g., 'K', 'P', 'B')
 * @param includeModifier Whether to include the modifier key
 */
export function formatShortcut(key: string, includeModifier: boolean = true): string {
	if (!includeModifier) return key;
	return `${getModifierSymbol()}${key}`;
}
