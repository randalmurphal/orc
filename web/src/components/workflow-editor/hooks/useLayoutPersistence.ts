/**
 * Hook for debounced layout persistence (SC-10)
 *
 * Provides debounced saving of node positions with:
 * - 1 second debounce period
 * - Position collection across multiple phase updates
 * - Error handling via callback
 * - Cleanup on unmount
 * - Immediate flush capability
 */

import { useCallback, useRef, useEffect } from 'react';
import { workflowClient } from '@/lib/client';

/** Position data for a single phase */
interface PhasePosition {
	phaseTemplateId: string;
	x: number;
	y: number;
}

interface UseLayoutPersistenceOptions {
	workflowId: string;
	onError?: (error: Error) => void;
}

interface UseLayoutPersistenceReturn {
	/** Save a single phase position (debounced) */
	savePosition: (phaseTemplateId: string, x: number, y: number) => void;
	/** Save multiple positions immediately */
	saveAllPositions: (positions: PhasePosition[]) => Promise<void>;
	/** Flush pending saves immediately (fire-and-forget) */
	flush: () => void;
}

const DEBOUNCE_MS = 1000;

export function useLayoutPersistence({
	workflowId,
	onError,
}: UseLayoutPersistenceOptions): UseLayoutPersistenceReturn {
	// Pending positions keyed by phaseTemplateId
	const pendingPositions = useRef<Map<string, { x: number; y: number }>>(
		new Map()
	);

	// Debounce timer ref
	const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

	// Track if component is mounted for cleanup
	const isMounted = useRef(true);

	// Clear timer on unmount
	useEffect(() => {
		isMounted.current = true;
		return () => {
			isMounted.current = false;
			if (timerRef.current) {
				clearTimeout(timerRef.current);
				timerRef.current = null;
			}
		};
	}, []);

	// Internal save function - synchronous to ensure mock tracking works with fake timers in tests
	const doSave = useCallback(() => {
		if (!isMounted.current) return;
		if (pendingPositions.current.size === 0) return;

		// Collect positions and clear pending
		const positions = Array.from(pendingPositions.current.entries()).map(
			([phaseTemplateId, { x, y }]) => ({
				phaseTemplateId,
				positionX: x,
				positionY: y,
			})
		);
		pendingPositions.current.clear();

		// Fire-and-forget: call API synchronously, handle errors in .catch()
		workflowClient
			.saveWorkflowLayout({
				workflowId,
				positions,
			})
			.catch((error) => {
				if (onError && error instanceof Error) {
					onError(error);
				}
			});
	}, [workflowId, onError]);

	// Save a single position with debounce
	const savePosition = useCallback(
		(phaseTemplateId: string, x: number, y: number) => {
			// Update pending positions (overwrites if same phase updated multiple times)
			pendingPositions.current.set(phaseTemplateId, { x, y });

			// Reset debounce timer
			if (timerRef.current) {
				clearTimeout(timerRef.current);
			}

			timerRef.current = setTimeout(() => {
				timerRef.current = null;
				doSave();
			}, DEBOUNCE_MS);
		},
		[doSave]
	);

	// Save all positions immediately (no debounce)
	const saveAllPositions = useCallback(
		async (positions: PhasePosition[]) => {
			const mappedPositions = positions.map(({ phaseTemplateId, x, y }) => ({
				phaseTemplateId,
				positionX: x,
				positionY: y,
			}));

			try {
				await workflowClient.saveWorkflowLayout({
					workflowId,
					positions: mappedPositions,
				});
			} catch (error) {
				if (onError && error instanceof Error) {
					onError(error);
				}
				throw error;
			}
		},
		[workflowId, onError]
	);

	// Flush pending saves immediately
	const flush = useCallback(() => {
		if (timerRef.current) {
			clearTimeout(timerRef.current);
			timerRef.current = null;
		}
		doSave();
	}, [doSave]);

	return {
		savePosition,
		saveAllPositions,
		flush,
	};
}
