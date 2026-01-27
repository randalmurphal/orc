/**
 * Utility functions for TranscriptViewer
 *
 * Pure functions for computing phase iterations and status from transcript data.
 */

import type { FlatTranscriptEntry } from '@/hooks/useTranscripts';
import { PhaseStatus, TaskStatus, type ExecutionState } from '@/gen/orc/v1/task_pb';
import type { TranscriptNavPhase } from './TranscriptNav';

/**
 * Compute max iteration count per phase from transcript entries.
 *
 * @param transcripts - Flattened transcript entries
 * @returns Map of phase name to max iteration number
 */
export function computePhaseIterations(
	transcripts: FlatTranscriptEntry[]
): Map<string, number> {
	const iterations = new Map<string, number>();

	for (const entry of transcripts) {
		const current = iterations.get(entry.phase) ?? 0;
		if (entry.iteration > current) {
			iterations.set(entry.phase, entry.iteration);
		}
	}

	return iterations;
}

/**
 * Determine navigation status for a phase based on execution state and task status.
 *
 * Status mapping:
 * - COMPLETED/SKIPPED -> 'completed'
 * - Phase === currentPhase && TaskStatus.RUNNING -> 'running'
 * - Phase === currentPhase && TaskStatus.FAILED -> 'failed'
 * - PENDING (not current) -> 'pending'
 * - No execution state + is current + running -> 'running'
 * - Default -> 'pending'
 *
 * @param phase - Phase name to check
 * @param execState - Execution state from task store (may be undefined)
 * @param currentPhase - Current active phase from task
 * @param taskStatus - Overall task status
 * @returns Navigation status string
 */
export function getPhaseNavStatus(
	phase: string,
	execState: ExecutionState | undefined,
	currentPhase: string | undefined,
	taskStatus: TaskStatus
): TranscriptNavPhase['status'] {
	const isCurrentPhase = phase === currentPhase;

	// Check execution state first if available
	if (execState?.phases) {
		const phaseState = execState.phases[phase];

		if (phaseState) {
			if (phaseState.status === PhaseStatus.COMPLETED) {
				return 'completed';
			}
			if (phaseState.status === PhaseStatus.SKIPPED) {
				return 'completed';
			}
		}
	}

	// Current phase status depends on task status
	if (isCurrentPhase) {
		if (taskStatus === TaskStatus.RUNNING) {
			return 'running';
		}
		if (taskStatus === TaskStatus.FAILED) {
			return 'failed';
		}
	}

	// Default to pending
	return 'pending';
}
