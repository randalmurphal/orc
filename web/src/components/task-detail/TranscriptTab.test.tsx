/**
 * Tests for TranscriptTab component
 *
 * Verifies:
 * - isRunning prop is passed through to TranscriptViewer
 * - TranscriptViewer receives correct running state from task status
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@testing-library/react';
import { TranscriptTab } from './TranscriptTab';

// Mock TranscriptViewer to capture its props
const mockTranscriptViewerProps = vi.fn();
vi.mock('@/components/transcript', () => ({
	TranscriptViewer: (props: Record<string, unknown>) => {
		mockTranscriptViewerProps(props);
		return <div data-testid="transcript-viewer" data-is-running={props.isRunning} />;
	},
}));

// Mock dependencies
vi.mock('@/lib/api', () => ({
	getTranscripts: vi.fn().mockResolvedValue([]),
}));

vi.mock('@/stores/uiStore', () => ({
	toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('@/components/ui/Icon', () => ({
	Icon: ({ name }: { name: string }) => <span data-testid={`icon-${name}`} />,
}));

describe('TranscriptTab', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('SC-1: isRunning prop chain', () => {
		it('should pass isRunning=true to TranscriptViewer when task is running', () => {
			render(
				<TranscriptTab
					taskId="TASK-001"
					isRunning={true}
					streamingLines={[]}
				/>
			);

			// Verify TranscriptViewer received isRunning=true
			expect(mockTranscriptViewerProps).toHaveBeenCalledWith(
				expect.objectContaining({ isRunning: true })
			);
		});

		it('should pass isRunning=false to TranscriptViewer when task is not running', () => {
			render(
				<TranscriptTab
					taskId="TASK-001"
					isRunning={false}
					streamingLines={[]}
				/>
			);

			// Verify TranscriptViewer received isRunning=false
			expect(mockTranscriptViewerProps).toHaveBeenCalledWith(
				expect.objectContaining({ isRunning: false })
			);
		});

		it('should infer running state from streamingLines when isRunning not provided', () => {
			render(
				<TranscriptTab
					taskId="TASK-001"
					streamingLines={[
						{ phase: 'implement', iteration: 1, type: 'response', content: 'test', timestamp: new Date().toISOString() }
					]}
				/>
			);

			// TranscriptViewer should get isRunning=true when streamingLines exist
			expect(mockTranscriptViewerProps).toHaveBeenCalledWith(
				expect.objectContaining({ isRunning: true })
			);
		});
	});
});
