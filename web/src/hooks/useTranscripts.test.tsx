import { renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { TimestampSchema } from '@bufbuild/protobuf/wkt';
import {
	GetTranscriptResponseSchema,
	ListTranscriptsResponseSchema,
	TranscriptFileSchema,
	TranscriptSchema,
	TranscriptEntrySchema,
} from '@/gen/orc/v1/transcript_pb';
import { useTranscripts } from './useTranscripts';

const {
	listTranscriptsMock,
	getTranscriptMock,
} = vi.hoisted(() => ({
	listTranscriptsMock: vi.fn(),
	getTranscriptMock: vi.fn(),
}));

function nowTimestamp() {
	return create(TimestampSchema, {
		seconds: BigInt(Math.floor(Date.now() / 1000)),
	});
}

vi.mock('@/lib/client', () => ({
	transcriptClient: {
		listTranscripts: listTranscriptsMock,
		getTranscript: getTranscriptMock,
	},
}));

vi.mock('@/stores', () => ({
	useCurrentProjectId: () => 'proj-001',
}));

describe('useTranscripts', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('includes projectId in transcript RPC requests', async () => {
		listTranscriptsMock.mockResolvedValue(
			create(ListTranscriptsResponseSchema, {
				transcripts: [
						create(TranscriptFileSchema, {
							phase: 'implement',
							iteration: 1,
							size: BigInt(1),
							createdAt: nowTimestamp(),
						}),
					],
				})
		);
		getTranscriptMock.mockResolvedValue(
			create(GetTranscriptResponseSchema, {
				transcript: create(TranscriptSchema, {
					taskId: 'TASK-001',
					phase: 'implement',
					iteration: 1,
					entries: [
						create(TranscriptEntrySchema, {
							type: 'assistant',
							content: 'done',
							timestamp: nowTimestamp(),
						}),
					],
				}),
			})
		);

		const { result } = renderHook(() =>
			useTranscripts({ taskId: 'TASK-001' })
		);

		await waitFor(() => expect(result.current.loading).toBe(false));

		expect(listTranscriptsMock).toHaveBeenCalled();
		for (const [request] of listTranscriptsMock.mock.calls) {
			expect(request.projectId).toBe('proj-001');
		}
		expect(getTranscriptMock).toHaveBeenCalledWith(
			expect.objectContaining({
				projectId: 'proj-001',
				taskId: 'TASK-001',
				phase: 'implement',
				iteration: 1,
			})
		);
	});

	it('defaults to the latest phase when no initial phase is provided', async () => {
		listTranscriptsMock
			.mockResolvedValueOnce(
				create(ListTranscriptsResponseSchema, {
					transcripts: [
						create(TranscriptFileSchema, {
							phase: 'plan',
							iteration: 1,
							size: BigInt(1),
							createdAt: nowTimestamp(),
						}),
						create(TranscriptFileSchema, {
							phase: 'implement',
							iteration: 1,
							size: BigInt(1),
							createdAt: nowTimestamp(),
						}),
					],
				})
			)
			.mockResolvedValue(
				create(ListTranscriptsResponseSchema, {
					transcripts: [
						create(TranscriptFileSchema, {
							phase: 'plan',
							iteration: 1,
							size: BigInt(1),
							createdAt: nowTimestamp(),
						}),
						create(TranscriptFileSchema, {
							phase: 'implement',
							iteration: 1,
							size: BigInt(1),
							createdAt: nowTimestamp(),
						}),
					],
				})
			);
		getTranscriptMock
			.mockResolvedValueOnce(
				create(GetTranscriptResponseSchema, {
					transcript: create(TranscriptSchema, {
						taskId: 'TASK-001',
						phase: 'plan',
						iteration: 1,
						entries: [
							create(TranscriptEntrySchema, {
								type: 'assistant',
								content: 'plan output',
								timestamp: { seconds: BigInt(1) },
							}),
						],
					}),
				})
			)
			.mockResolvedValueOnce(
				create(GetTranscriptResponseSchema, {
					transcript: create(TranscriptSchema, {
						taskId: 'TASK-001',
						phase: 'implement',
						iteration: 1,
						entries: [
							create(TranscriptEntrySchema, {
								type: 'assistant',
								content: 'implement output',
								timestamp: { seconds: BigInt(2) },
							}),
						],
					}),
				})
			);

		const { result } = renderHook(() =>
			useTranscripts({ taskId: 'TASK-001' })
		);

		await waitFor(() => expect(result.current.currentPhase).toBe('implement'));
	});

	it('extracts session_id from JSON session metadata objects', async () => {
		listTranscriptsMock.mockResolvedValue(
			create(ListTranscriptsResponseSchema, {
				transcripts: [
					create(TranscriptFileSchema, {
						phase: 'implement',
						iteration: 1,
						size: BigInt(1),
						createdAt: nowTimestamp(),
					}),
				],
			}),
		);
		getTranscriptMock.mockResolvedValue(
			create(GetTranscriptResponseSchema, {
				transcript: create(TranscriptSchema, {
					taskId: 'TASK-001',
					phase: 'implement',
					iteration: 1,
					sessionMetadata: JSON.stringify({
						provider: 'claude',
						data: { session_id: 'sess-123' },
					}),
					entries: [
						create(TranscriptEntrySchema, {
							type: 'assistant',
							content: 'done',
							timestamp: nowTimestamp(),
						}),
					],
				}),
			}),
		);

		const { result } = renderHook(() =>
			useTranscripts({ taskId: 'TASK-001' })
		);

		await waitFor(() => expect(result.current.loading).toBe(false));
		expect(result.current.transcripts[0]?.session_id).toBe('sess-123');
	});
});
