import { useState, useCallback } from 'react';
import type { ReviewComment } from '@/gen/orc/v1/task_pb';
import { CommentSeverity, CommentStatus } from '@/gen/orc/v1/task_pb';
import { timestampToDate } from '@/lib/time';
import type { CreateCommentRequest } from './types';
import './InlineCommentThread.css';

// Helper to convert CommentSeverity enum to display string
const severityToString = (severity: CommentSeverity): string => {
	switch (severity) {
		case CommentSeverity.SUGGESTION:
			return 'suggestion';
		case CommentSeverity.ISSUE:
			return 'issue';
		case CommentSeverity.BLOCKER:
			return 'blocker';
		default:
			return 'issue';
	}
};

interface InlineCommentThreadProps {
	filePath: string;
	lineNumber: number;
	comments: ReviewComment[];
	showForm: boolean;
	onAddComment: (comment: CreateCommentRequest) => Promise<void>;
	onResolveComment: (id: string) => void;
	onWontFixComment: (id: string) => void;
	onDeleteComment: (id: string) => void;
	onCloseThread: () => void;
}

export function InlineCommentThread({
	filePath,
	lineNumber,
	comments,
	showForm,
	onAddComment,
	onResolveComment,
	onWontFixComment,
	onDeleteComment,
	onCloseThread,
}: InlineCommentThreadProps) {
	const [content, setContent] = useState('');
	const [severity, setSeverity] = useState<CommentSeverity>(CommentSeverity.ISSUE);
	const [submitting, setSubmitting] = useState(false);

	const openComments = comments.filter((c) => c.status === CommentStatus.OPEN);
	const resolvedComments = comments.filter((c) => c.status !== CommentStatus.OPEN);

	const handleSubmit = useCallback(async () => {
		if (!content.trim() || submitting) return;

		setSubmitting(true);
		try {
			await onAddComment({
				filePath,
				lineNumber,
				content: content.trim(),
				severity,
			});
			setContent('');
			setSeverity(CommentSeverity.ISSUE);
		} finally {
			setSubmitting(false);
		}
	}, [content, severity, filePath, lineNumber, onAddComment, submitting]);

	if (!showForm && comments.length === 0) {
		return null;
	}

	return (
		<div className="inline-comment-thread">
			{/* Open Comments */}
			{openComments.map((comment) => {
				const severityStr = severityToString(comment.severity);
				const createdDate = timestampToDate(comment.createdAt);
				return (
					<div key={comment.id} className={`comment ${severityStr}`}>
						<div className="comment-header">
							<span className={`severity-badge ${severityStr}`}>{severityStr}</span>
							<span className="timestamp">
								{createdDate?.toLocaleString() ?? 'N/A'}
							</span>
						</div>
						<div className="comment-content">{comment.content}</div>
						<div className="comment-actions">
							<button onClick={() => onResolveComment(comment.id)}>Resolve</button>
							<button onClick={() => onWontFixComment(comment.id)}>Won't Fix</button>
							<button className="delete" onClick={() => onDeleteComment(comment.id)}>
								Delete
							</button>
						</div>
					</div>
				);
			})}

			{/* Resolved Comments (collapsed) */}
			{resolvedComments.length > 0 && (
				<div className="resolved-comments">
					<span className="resolved-label">
						{resolvedComments.length} resolved comment{resolvedComments.length > 1 ? 's' : ''}
					</span>
				</div>
			)}

			{/* Add Comment Form */}
			{showForm && (
				<div className="comment-form">
					<div className="severity-pills">
						{([
							{ value: CommentSeverity.SUGGESTION, label: 'suggestion' },
							{ value: CommentSeverity.ISSUE, label: 'issue' },
							{ value: CommentSeverity.BLOCKER, label: 'blocker' },
						]).map((sev) => (
							<button
								key={sev.label}
								type="button"
								className={`severity-pill ${severity === sev.value ? 'selected' : ''}`}
								onClick={() => setSeverity(sev.value)}
							>
								{sev.label}
							</button>
						))}
					</div>
					<textarea
						value={content}
						onChange={(e) => setContent(e.target.value)}
						placeholder="Add a review comment..."
						rows={2}
						disabled={submitting}
						autoFocus
					/>
					<div className="form-actions">
						<button className="cancel-btn" onClick={onCloseThread} disabled={submitting}>
							Cancel
						</button>
						<button
							className="submit-btn"
							onClick={handleSubmit}
							disabled={!content.trim() || submitting}
						>
							{submitting ? 'Adding...' : 'Add Comment'}
						</button>
					</div>
				</div>
			)}
		</div>
	);
}
