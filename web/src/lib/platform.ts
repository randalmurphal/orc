/**
 * Platform detection utilities for keyboard shortcuts
 *
 * NOTE: The app uses Shift+Alt (Shift+Option on Mac) as the primary modifier
 * for global shortcuts to avoid conflicts with browser shortcuts like
 * Cmd+K, Cmd+N, Cmd+P, etc.
 */

/**
 * Detects if the user is on a Mac platform
 * Uses userAgentData if available, falls back to userAgent string
 */
export function isMac(): boolean {
	if (typeof navigator === 'undefined') return false;

	// Try modern userAgentData API first
	const nav = navigator as Navigator & {
		userAgentData?: { platform: string };
	};
	if (nav.userAgentData?.platform) {
		return nav.userAgentData.platform.toLowerCase() === 'macos';
	}

	// Fall back to userAgent string parsing
	return /Mac|iPod|iPhone|iPad/.test(navigator.userAgent);
}

/**
 * Returns the appropriate modifier key display for the platform
 * Mac: ⇧⌥ (Shift+Option)
 * Others: Shift+Alt
 *
 * Uses Shift+Alt instead of Cmd/Ctrl to avoid browser shortcut conflicts
 */
export function getModifierKey(): string {
	return isMac() ? '⇧⌥' : 'Shift+Alt';
}

/**
 * Returns the appropriate modifier key symbol (short form)
 * Mac: ⇧⌥
 * Others: Shift+Alt+
 */
export function getModifierSymbol(): string {
	return isMac() ? '⇧⌥' : 'Shift+Alt+';
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
