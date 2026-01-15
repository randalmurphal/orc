import { useState, useCallback } from 'react';
import { Button } from '@/components/ui/Button';
import type { ReviewComment, CreateCommentRequest, CommentSeverity } from '@/lib/types';
import './InlineCommentThread.css';

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
	const [severity, setSeverity] = useState<CommentSeverity>('issue');
	const [submitting, setSubmitting] = useState(false);

	const openComments = comments.filter((c) => c.status === 'open');
	const resolvedComments = comments.filter((c) => c.status !== 'open');

	const handleSubmit = useCallback(async () => {
		if (!content.trim() || submitting) return;

		setSubmitting(true);
		try {
			await onAddComment({
				file_path: filePath,
				line_number: lineNumber,
				content: content.trim(),
				severity,
			});
			setContent('');
			setSeverity('issue');
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
			{openComments.map((comment) => (
				<div key={comment.id} className={`comment ${comment.severity}`}>
					<div className="comment-header">
						<span className={`severity-badge ${comment.severity}`}>{comment.severity}</span>
						<span className="timestamp">
							{new Date(comment.created_at).toLocaleString()}
						</span>
					</div>
					<div className="comment-content">{comment.content}</div>
					<div className="comment-actions">
						<Button variant="success" size="sm" onClick={() => onResolveComment(comment.id)}>
							Resolve
						</Button>
						<Button variant="secondary" size="sm" onClick={() => onWontFixComment(comment.id)}>
							Won't Fix
						</Button>
						<Button variant="danger" size="sm" onClick={() => onDeleteComment(comment.id)}>
							Delete
						</Button>
					</div>
				</div>
			))}

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
						{(['suggestion', 'issue', 'blocker'] as const).map((sev) => (
							<Button
								key={sev}
								type="button"
								variant="ghost"
								size="sm"
								className={`severity-pill ${severity === sev ? 'selected' : ''}`}
								onClick={() => setSeverity(sev)}
							>
								{sev}
							</Button>
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
						<Button variant="secondary" size="sm" onClick={onCloseThread} disabled={submitting}>
							Cancel
						</Button>
						<Button
							variant="primary"
							size="sm"
							onClick={handleSubmit}
							disabled={!content.trim()}
							loading={submitting}
						>
							Add Comment
						</Button>
					</div>
				</div>
			)}
		</div>
	);
}
