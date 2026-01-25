import { useCallback } from 'react';

/**
 * Hook for handling Enter/Space key presses as clicks.
 * Use on interactive non-button elements for accessibility.
 *
 * @example
 * const handleKeyDown = useClickKeyboard(onClick);
 * <div onClick={onClick} onKeyDown={handleKeyDown} role="button" tabIndex={0}>
 *   Click me
 * </div>
 */
export function useClickKeyboard(onClick: () => void) {
	return useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				onClick();
			}
		},
		[onClick]
	);
}
