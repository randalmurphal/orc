import { describe, it, expect, afterEach } from 'vitest';
import { isMac, getModifierKey, getModifierSymbol, formatShortcut } from './platform';

describe('platform utilities', () => {
	const originalNavigator = globalThis.navigator;

	afterEach(() => {
		// Restore original navigator
		Object.defineProperty(globalThis, 'navigator', {
			value: originalNavigator,
			writable: true,
		});
	});

	describe('isMac', () => {
		it('should return true for macOS via userAgentData', () => {
			Object.defineProperty(globalThis, 'navigator', {
				value: {
					userAgentData: { platform: 'macOS' },
					userAgent: '',
				},
				writable: true,
			});

			expect(isMac()).toBe(true);
		});

		it('should return false for Windows via userAgentData', () => {
			Object.defineProperty(globalThis, 'navigator', {
				value: {
					userAgentData: { platform: 'Windows' },
					userAgent: '',
				},
				writable: true,
			});

			expect(isMac()).toBe(false);
		});

		it('should return true for Mac via userAgent fallback', () => {
			Object.defineProperty(globalThis, 'navigator', {
				value: {
					userAgent:
						'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36',
				},
				writable: true,
			});

			expect(isMac()).toBe(true);
		});

		it('should return true for iPhone via userAgent', () => {
			Object.defineProperty(globalThis, 'navigator', {
				value: {
					userAgent:
						'Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15',
				},
				writable: true,
			});

			expect(isMac()).toBe(true);
		});

		it('should return true for iPad via userAgent', () => {
			Object.defineProperty(globalThis, 'navigator', {
				value: {
					userAgent:
						'Mozilla/5.0 (iPad; CPU OS 14_0 like Mac OS X) AppleWebKit/605.1.15',
				},
				writable: true,
			});

			expect(isMac()).toBe(true);
		});

		it('should return false for Windows via userAgent', () => {
			Object.defineProperty(globalThis, 'navigator', {
				value: {
					userAgent:
						'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
				},
				writable: true,
			});

			expect(isMac()).toBe(false);
		});

		it('should return false for Linux via userAgent', () => {
			Object.defineProperty(globalThis, 'navigator', {
				value: {
					userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36',
				},
				writable: true,
			});

			expect(isMac()).toBe(false);
		});

		it('should return false when navigator is undefined', () => {
			Object.defineProperty(globalThis, 'navigator', {
				value: undefined,
				writable: true,
			});

			expect(isMac()).toBe(false);
		});
	});

	describe('getModifierKey', () => {
		it('should return Mac symbols on Mac', () => {
			Object.defineProperty(globalThis, 'navigator', {
				value: {
					userAgentData: { platform: 'macOS' },
					userAgent: '',
				},
				writable: true,
			});

			expect(getModifierKey()).toBe('⇧⌥');
		});

		it('should return text on non-Mac', () => {
			Object.defineProperty(globalThis, 'navigator', {
				value: {
					userAgentData: { platform: 'Windows' },
					userAgent: '',
				},
				writable: true,
			});

			expect(getModifierKey()).toBe('Shift+Alt');
		});
	});

	describe('getModifierSymbol', () => {
		it('should return Mac symbols on Mac', () => {
			Object.defineProperty(globalThis, 'navigator', {
				value: {
					userAgentData: { platform: 'macOS' },
					userAgent: '',
				},
				writable: true,
			});

			expect(getModifierSymbol()).toBe('⇧⌥');
		});

		it('should return text with plus on non-Mac', () => {
			Object.defineProperty(globalThis, 'navigator', {
				value: {
					userAgentData: { platform: 'Windows' },
					userAgent: '',
				},
				writable: true,
			});

			expect(getModifierSymbol()).toBe('Shift+Alt+');
		});
	});

	describe('formatShortcut', () => {
		beforeEach(() => {
			Object.defineProperty(globalThis, 'navigator', {
				value: {
					userAgentData: { platform: 'macOS' },
					userAgent: '',
				},
				writable: true,
			});
		});

		it('should format with modifier on Mac', () => {
			expect(formatShortcut('K')).toBe('⇧⌥K');
		});

		it('should format without modifier when specified', () => {
			expect(formatShortcut('K', false)).toBe('K');
		});

		it('should format with modifier on Windows', () => {
			Object.defineProperty(globalThis, 'navigator', {
				value: {
					userAgentData: { platform: 'Windows' },
					userAgent: '',
				},
				writable: true,
			});

			expect(formatShortcut('K')).toBe('Shift+Alt+K');
		});
	});
});
