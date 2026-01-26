/**
 * Types for diff components
 */

import type { CommentSeverity } from '@/gen/orc/v1/task_pb';

/**
 * Request type for creating a review comment from the UI.
 * This is an internal type used by diff components.
 */
export interface CreateCommentRequest {
	filePath?: string;
	lineNumber?: number;
	content: string;
	severity: CommentSeverity;
}
