/**
 * Tests for TranscriptViewer utility functions
 * 
 * SC-1: computePhaseIterations derives max iteration per phase from transcript data
 * SC-2: getPhaseNavStatus maps ExecutionState to TranscriptNav status string
 */

import { describe, it, expect } from 'vitest';
import { computePhaseIterations, getPhaseNavStatus } from './transcript-utils';
import type { FlatTranscriptEntry } from '@/hooks/useTranscripts';
import { PhaseStatus, TaskStatus } from '@/gen/orc/v1/task_pb';
import type { ExecutionState } from '@/gen/orc/v1/task_pb';

// Helper to create transcript entry for testing
const createEntry = (phase: string, iteration: number): FlatTranscriptEntry => ({
	id: Math.random(),
	task_id: 'TASK-001',
	phase,
	iteration,
	session_id: 'session-1',
	type: 'assistant',
	content: 'test',
	input_tokens: 0,
	output_tokens: 0,
	timestamp: new Date().toISOString(),
});

describe('computePhaseIterations', () => {
	it('returns empty map for empty transcripts', () => {
		const result = computePhaseIterations([]);
		expect(result.size).toBe(0);
	});

	it('returns max iteration for single phase', () => {
		const transcripts: FlatTranscriptEntry[] = [
			createEntry('implement', 1),
			createEntry('implement', 2),
			createEntry('implement', 3),
			createEntry('implement', 2), // duplicate iteration
		];
		
		const result = computePhaseIterations(transcripts);
		expect(result.get('implement')).toBe(3);
	});

	it('returns max iteration per phase for multiple phases', () => {
		const transcripts: FlatTranscriptEntry[] = [
			createEntry('spec', 1),
			createEntry('implement', 1),
			createEntry('implement', 2),
			createEntry('review', 1),
		];
		
		const result = computePhaseIterations(transcripts);
		expect(result.get('spec')).toBe(1);
		expect(result.get('implement')).toBe(2);
		expect(result.get('review')).toBe(1);
	});

	it('handles phase with single iteration', () => {
		const transcripts: FlatTranscriptEntry[] = [
			createEntry('docs', 1),
		];
		
		const result = computePhaseIterations(transcripts);
		expect(result.get('docs')).toBe(1);
	});
});

describe('getPhaseNavStatus', () => {
	// Helper to create minimal ExecutionState
	const createExecutionState = (
		phases: Record<string, { status: PhaseStatus }>,
	): ExecutionState => ({
		$typeName: 'orc.v1.ExecutionState',
		currentIteration: 1,
		phases: Object.fromEntries(
			Object.entries(phases).map(([name, p]) => [
				name,
				{
					$typeName: 'orc.v1.PhaseState',
					status: p.status,
					iterations: 1,
					artifacts: [],
					validationHistory: [],
				},
			])
		),
		gates: [],
	});

	describe('with execution state', () => {
		it('returns "completed" for PHASE_STATUS_COMPLETED', () => {
			const state = createExecutionState({
				spec: { status: PhaseStatus.COMPLETED },
			});
			
			const result = getPhaseNavStatus('spec', state, undefined, TaskStatus.RUNNING);
			expect(result).toBe('completed');
		});

		it('returns "completed" for PHASE_STATUS_SKIPPED', () => {
			const state = createExecutionState({
				review: { status: PhaseStatus.SKIPPED },
			});
			
			const result = getPhaseNavStatus('review', state, undefined, TaskStatus.RUNNING);
			expect(result).toBe('completed');
		});

		it('returns "pending" for PHASE_STATUS_PENDING when not current phase', () => {
			const state = createExecutionState({
				docs: { status: PhaseStatus.PENDING },
			});
			
			const result = getPhaseNavStatus('docs', state, 'implement', TaskStatus.RUNNING);
			expect(result).toBe('pending');
		});

		it('returns "running" for current phase when task is running', () => {
			const state = createExecutionState({
				implement: { status: PhaseStatus.PENDING },
			});
			
			const result = getPhaseNavStatus('implement', state, 'implement', TaskStatus.RUNNING);
			expect(result).toBe('running');
		});

		it('returns "failed" for current phase when task is failed', () => {
			const state = createExecutionState({
				implement: { status: PhaseStatus.PENDING },
			});
			
			const result = getPhaseNavStatus('implement', state, 'implement', TaskStatus.FAILED);
			expect(result).toBe('failed');
		});

		it('returns "pending" for phase not in execution state', () => {
			const state = createExecutionState({});
			
			const result = getPhaseNavStatus('unknown', state, undefined, TaskStatus.RUNNING);
			expect(result).toBe('pending');
		});
	});

	describe('without execution state', () => {
		it('returns "running" for current phase when task is running', () => {
			const result = getPhaseNavStatus('implement', undefined, 'implement', TaskStatus.RUNNING);
			expect(result).toBe('running');
		});

		it('returns "pending" for non-current phase', () => {
			const result = getPhaseNavStatus('docs', undefined, 'implement', TaskStatus.RUNNING);
			expect(result).toBe('pending');
		});

		it('returns "failed" for current phase when task failed', () => {
			const result = getPhaseNavStatus('implement', undefined, 'implement', TaskStatus.FAILED);
			expect(result).toBe('failed');
		});
	});
});
