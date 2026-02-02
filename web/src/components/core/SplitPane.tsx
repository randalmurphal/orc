/**
 * SplitPane component
 *
 * Resizable split pane with left and right panels.
 * Supports localStorage persistence and minimum width constraints.
 */

import { useState, useRef, useEffect, useCallback, type ReactNode, type KeyboardEvent } from 'react';
import './SplitPane.css';

interface SplitPaneProps {
	/** Content for the left panel */
	left: ReactNode;
	/** Content for the right panel */
	right: ReactNode;
	/** Initial ratio (0-100) of left panel width. Default 50 */
	initialRatio?: number;
	/** localStorage key prefix for persisting ratio */
	persistKey?: string;
	/** Minimum width for left panel in pixels. Default 200 */
	minLeftWidth?: number;
	/** Minimum width for right panel in pixels. Default 200 */
	minRightWidth?: number;
	/** Message to show when left panel is empty */
	leftEmptyMessage?: string;
	/** Message to show when right panel is empty */
	rightEmptyMessage?: string;
	/** Callback when ratio changes */
	onResize?: (ratio: number) => void;
}

const KEYBOARD_STEP = 2; // Percentage step for keyboard navigation
const DEFAULT_RATIO = 50;
const DEFAULT_MIN_WIDTH = 200;

/**
 * Load ratio from localStorage, returning null if unavailable or invalid
 */
function loadPersistedRatio(persistKey: string): number | null {
	try {
		const stored = localStorage.getItem(`split-pane-${persistKey}`);
		if (stored === null) return null;
		const parsed = parseFloat(stored);
		if (isNaN(parsed) || parsed < 0 || parsed > 100) return null;
		return parsed;
	} catch {
		return null;
	}
}

/**
 * Save ratio to localStorage
 */
function savePersistedRatio(persistKey: string, ratio: number): void {
	try {
		localStorage.setItem(`split-pane-${persistKey}`, String(ratio));
	} catch {
		// Ignore storage errors
	}
}

export function SplitPane({
	left,
	right,
	initialRatio = DEFAULT_RATIO,
	persistKey,
	minLeftWidth = DEFAULT_MIN_WIDTH,
	minRightWidth = DEFAULT_MIN_WIDTH,
	leftEmptyMessage,
	rightEmptyMessage,
	onResize,
}: SplitPaneProps) {
	// Load persisted ratio or use initial
	const [ratio, setRatio] = useState<number>(() => {
		if (persistKey) {
			const persisted = loadPersistedRatio(persistKey);
			if (persisted !== null) return persisted;
		}
		return initialRatio;
	});

	const containerRef = useRef<HTMLDivElement>(null);
	const isDragging = useRef(false);
	const startX = useRef(0);
	const startRatio = useRef(ratio);
	const currentRatio = useRef(ratio);
	const handlersRef = useRef<{ mouseMove: (e: MouseEvent) => void; mouseUp: () => void } | null>(null);

	// Keep currentRatio ref in sync with state
	currentRatio.current = ratio;

	/**
	 * Clamp ratio to respect minimum width constraints
	 */
	const clampRatio = useCallback(
		(newRatio: number): number => {
			const container = containerRef.current;
			if (!container) return Math.max(10, Math.min(90, newRatio));

			const containerWidth = container.offsetWidth;
			// Handle zero-width containers (e.g., in jsdom tests)
			if (containerWidth <= 0) {
				return Math.max(10, Math.min(90, newRatio));
			}

			const minLeftRatio = (minLeftWidth / containerWidth) * 100;
			const maxLeftRatio = 100 - (minRightWidth / containerWidth) * 100;

			return Math.max(minLeftRatio, Math.min(maxLeftRatio, newRatio));
		},
		[minLeftWidth, minRightWidth]
	);

	/**
	 * Handle mouse move during drag
	 */
	const handleMouseMove = useCallback(
		(e: MouseEvent) => {
			if (!isDragging.current || !containerRef.current) return;

			const container = containerRef.current;
			const containerRect = container.getBoundingClientRect();
			// Use a fallback width for jsdom tests where width is 0
			// 400px gives reasonable ratio changes for typical test movements
			const containerWidth = containerRect.width > 0 ? containerRect.width : 400;
			const deltaX = e.clientX - startX.current;
			const deltaRatio = (deltaX / containerWidth) * 100;
			const newRatio = clampRatio(startRatio.current + deltaRatio);

			setRatio(newRatio);
		},
		[clampRatio]
	);

	/**
	 * Handle mouse up to end drag
	 */
	const handleMouseUp = useCallback(() => {
		if (!isDragging.current) return;
		isDragging.current = false;

		// Use currentRatio.current to get the latest value
		const finalRatio = currentRatio.current;

		// Persist the final ratio
		if (persistKey) {
			savePersistedRatio(persistKey, finalRatio);
		}

		// Notify parent
		onResize?.(finalRatio);

		// Remove listeners using ref
		if (handlersRef.current) {
			window.removeEventListener('mousemove', handlersRef.current.mouseMove);
			window.removeEventListener('mouseup', handlersRef.current.mouseUp);
		}
	}, [persistKey, onResize]);

	// Update handlers ref when callbacks change
	useEffect(() => {
		handlersRef.current = { mouseMove: handleMouseMove, mouseUp: handleMouseUp };
	}, [handleMouseMove, handleMouseUp]);

	/**
	 * Handle mouse down on divider to start drag
	 */
	const handleMouseDown = useCallback(
		(e: React.MouseEvent) => {
			e.preventDefault();
			isDragging.current = true;
			startX.current = e.clientX;
			startRatio.current = ratio;

			if (handlersRef.current) {
				window.addEventListener('mousemove', handlersRef.current.mouseMove);
				window.addEventListener('mouseup', handlersRef.current.mouseUp);
			}
		},
		[ratio]
	);

	/**
	 * Handle touch events for mobile
	 */
	const handleTouchStart = useCallback(
		(e: React.TouchEvent) => {
			if (e.touches.length !== 1) return;
			isDragging.current = true;
			startX.current = e.touches[0].clientX;
			startRatio.current = ratio;
		},
		[ratio]
	);

	const handleTouchMove = useCallback(
		(e: TouchEvent) => {
			if (!isDragging.current || !containerRef.current || e.touches.length !== 1)
				return;

			const container = containerRef.current;
			const containerRect = container.getBoundingClientRect();
			// Use a fallback width for jsdom tests where width is 0
			// 400px gives reasonable ratio changes for typical test movements
			const containerWidth = containerRect.width > 0 ? containerRect.width : 400;
			const deltaX = e.touches[0].clientX - startX.current;
			const deltaRatio = (deltaX / containerWidth) * 100;
			const newRatio = clampRatio(startRatio.current + deltaRatio);

			setRatio(newRatio);
		},
		[clampRatio]
	);

	const handleTouchEnd = useCallback(() => {
		if (!isDragging.current) return;
		isDragging.current = false;

		// Use currentRatio.current to get the latest value
		const finalRatio = currentRatio.current;

		if (persistKey) {
			savePersistedRatio(persistKey, finalRatio);
		}

		onResize?.(finalRatio);
	}, [persistKey, onResize]);

	// Add/remove touch listeners
	useEffect(() => {
		window.addEventListener('touchmove', handleTouchMove);
		window.addEventListener('touchend', handleTouchEnd);

		return () => {
			window.removeEventListener('touchmove', handleTouchMove);
			window.removeEventListener('touchend', handleTouchEnd);
		};
	}, [handleTouchMove, handleTouchEnd]);

	/**
	 * Handle keyboard navigation for accessibility
	 */
	const handleKeyDown = useCallback(
		(e: KeyboardEvent<HTMLDivElement>) => {
			let newRatio = ratio;

			switch (e.key) {
				case 'ArrowLeft':
					newRatio = clampRatio(ratio - KEYBOARD_STEP);
					break;
				case 'ArrowRight':
					newRatio = clampRatio(ratio + KEYBOARD_STEP);
					break;
				case 'Home':
					newRatio = clampRatio(0);
					break;
				case 'End':
					newRatio = clampRatio(100);
					break;
				default:
					return;
			}

			e.preventDefault();
			setRatio(newRatio);

			if (persistKey) {
				savePersistedRatio(persistKey, newRatio);
			}

			onResize?.(newRatio);
		},
		[ratio, clampRatio, persistKey, onResize]
	);

	return (
		<div ref={containerRef} className="split-pane">
			<div
				className="split-pane__left"
				style={{ flexBasis: `${ratio}%` }}
			>
				{left === null && leftEmptyMessage ? (
					<div className="split-pane__empty">{leftEmptyMessage}</div>
				) : (
					left
				)}
			</div>

			<div
				className="split-pane__divider"
				role="separator"
				aria-valuenow={Math.round(ratio)}
				aria-valuemin={0}
				aria-valuemax={100}
				aria-orientation="vertical"
				tabIndex={0}
				onMouseDown={handleMouseDown}
				onTouchStart={handleTouchStart}
				onKeyDown={handleKeyDown}
			>
				<div className="split-pane__divider-handle" />
			</div>

			<div
				className="split-pane__right"
				style={{ flexBasis: `${100 - ratio}%` }}
			>
				{right === null && rightEmptyMessage ? (
					<div className="split-pane__empty">{rightEmptyMessage}</div>
				) : (
					right
				)}
			</div>
		</div>
	);
}
