import { useState, useEffect, useCallback, useMemo, useRef, FormEvent, KeyboardEvent } from 'react';
import type { TaskComment, CreateTaskCommentRequest, TaskCommentAuthorType } from '@/lib/types';
import {
	getTaskComments,
	createTaskComment,
	deleteTaskComment,
	updateTaskComment,
} from '@/lib/api';
import { Button } from '@/components/ui/Button';
import { Icon, type IconName } from '@/components/ui/Icon';
import { toast } from '@/stores/uiStore';
import './CommentsTab.css';

interface CommentsTabProps {
	taskId: string;
	phases?: string[];
}

function formatRelativeTime(dateStr: string): string {
	const date = new Date(dateStr);
	const now = new Date();
	const diffMs = now.getTime() - date.getTime();
	const diffSec = Math.floor(diffMs / 1000);
	const diffMin = Math.floor(diffSec / 60);
	const diffHour = Math.floor(diffMin / 60);
	const diffDay = Math.floor(diffHour / 24);

	if (diffSec < 60) return 'just now';
	if (diffMin < 60) return `${diffMin}m ago`;
	if (diffHour < 24) return `${diffHour}h ago`;
	if (diffDay < 7) return `${diffDay}d ago`;

	return date.toLocaleDateString(undefined, {
		month: 'short',
		day: 'numeric',
	});
}

const authorTypeConfig: Record<
	TaskCommentAuthorType,
	{ color: string; bg: string; label: string; icon: IconName }
> = {
	human: {
		color: 'var(--status-info)',
		bg: 'var(--status-info-bg)',
		label: 'Human',
		icon: 'user',
	},
	agent: {
		color: 'var(--accent-primary)',
		bg: 'var(--accent-glow)',
		label: 'Agent',
		icon: 'cpu',
	},
	system: {
		color: 'var(--text-muted)',
		bg: 'var(--bg-tertiary)',
		label: 'System',
		icon: 'settings',
	},
};

const authorTypeOptions: { value: TaskCommentAuthorType; label: string; description: string }[] = [
	{ value: 'human', label: 'Human', description: 'Manual note or feedback' },
	{ value: 'agent', label: 'Agent', description: 'Note from Claude/AI' },
	{ value: 'system', label: 'System', description: 'Automated system note' },
];

interface CommentThreadProps {
	comment: TaskComment;
	onEdit?: (id: string) => void;
	onDelete?: (id: string) => void;
}

function CommentThread({ comment, onEdit, onDelete }: CommentThreadProps) {
	const authorType = authorTypeConfig[comment.author_type];

	return (
		<div className="comment-thread">
			<div className="comment-header">
				<div
					className="author-badge"
					style={{ background: authorType.bg, color: authorType.color }}
				>
					<Icon name={authorType.icon} size={12} />
					<span>{comment.author || authorType.label}</span>
				</div>
				{comment.phase && (
					<div className="phase-badge">
						<Icon name="layers" size={12} />
						<span>{comment.phase}</span>
					</div>
				)}
				<span className="timestamp">{formatRelativeTime(comment.created_at)}</span>
			</div>

			<div className="comment-content">{comment.content}</div>

			{comment.updated_at !== comment.created_at && (
				<div className="edited-info">
					<span>edited {formatRelativeTime(comment.updated_at)}</span>
				</div>
			)}

			{(onEdit || onDelete) && (
				<div className="comment-actions">
					{onEdit && (
						<Button
							variant="ghost"
							size="sm"
							iconOnly
							onClick={() => onEdit(comment.id)}
							title="Edit comment"
							aria-label="Edit comment"
						>
							<Icon name="edit" size={14} />
						</Button>
					)}
					{onDelete && (
						<Button
							variant="danger"
							size="sm"
							iconOnly
							onClick={() => onDelete(comment.id)}
							title="Delete comment"
							aria-label="Delete comment"
						>
							<Icon name="trash" size={14} />
						</Button>
					)}
				</div>
			)}
		</div>
	);
}

interface CommentFormProps {
	phases?: string[];
	onSubmit: (comment: CreateTaskCommentRequest) => void;
	onCancel: () => void;
	isLoading?: boolean;
	editMode?: boolean;
	initialContent?: string;
	initialPhase?: string;
}

function CommentForm({
	phases = [],
	onSubmit,
	onCancel,
	isLoading = false,
	editMode = false,
	initialContent = '',
	initialPhase = '',
}: CommentFormProps) {
	const [content, setContent] = useState(initialContent);
	const [phase, setPhase] = useState(initialPhase);
	const [authorType, setAuthorType] = useState<TaskCommentAuthorType>('human');
	const [author, setAuthor] = useState('');
	const textareaRef = useRef<HTMLTextAreaElement>(null);

	// Focus textarea on mount
	useEffect(() => {
		textareaRef.current?.focus();
	}, []);

	// Platform detection for keyboard hints
	const isMac =
		typeof navigator !== 'undefined' && /Mac|iPhone|iPad|iPod/.test(navigator.platform);
	const modifierKey = isMac ? 'Cmd' : 'Ctrl';

	const canSubmit = content.trim().length > 0 && !isLoading;

	const handleSubmit = useCallback(
		(e: FormEvent) => {
			e.preventDefault();
			if (!canSubmit) return;

			const comment: CreateTaskCommentRequest = {
				content: content.trim(),
				author_type: authorType,
			};

			if (author.trim()) {
				comment.author = author.trim();
			}

			if (phase) {
				comment.phase = phase;
			}

			onSubmit(comment);
		},
		[canSubmit, content, authorType, author, phase, onSubmit]
	);

	const handleKeyDown = useCallback(
		(e: KeyboardEvent) => {
			if (e.key === 'Escape') {
				onCancel();
			} else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
				handleSubmit(e as unknown as FormEvent);
			}
		},
		[onCancel, handleSubmit]
	);

	return (
		<form className="comment-form" onSubmit={handleSubmit} onKeyDown={handleKeyDown}>
			<div className="form-header">
				<h3>{editMode ? 'Edit Comment' : 'Add Comment'}</h3>
				<Button
					type="button"
					variant="ghost"
					size="sm"
					iconOnly
					onClick={onCancel}
					title="Cancel"
					aria-label="Cancel"
				>
					<Icon name="x" size={16} />
				</Button>
			</div>

			{!editMode && (
				<>
					<div className="form-row">
						<div className="form-field author-field">
							<label htmlFor="author">Author (optional)</label>
							<input
								id="author"
								type="text"
								value={author}
								onChange={(e) => setAuthor(e.target.value)}
								placeholder="Your name"
								disabled={isLoading}
							/>
						</div>
						{phases.length > 0 && (
							<div className="form-field phase-field">
								<label htmlFor="phase">Phase (optional)</label>
								<select
									id="phase"
									value={phase}
									onChange={(e) => setPhase(e.target.value)}
									disabled={isLoading}
								>
									<option value="">No phase</option>
									{phases.map((p) => (
										<option key={p} value={p}>
											{p}
										</option>
									))}
								</select>
							</div>
						)}
					</div>

					<div className="form-field">
						<label htmlFor="author-type">Type</label>
						<div className="author-type-options">
							{authorTypeOptions.map((option) => (
								<label
									key={option.value}
									className={`author-type-option ${option.value} ${authorType === option.value ? 'selected' : ''}`}
								>
									<input
										type="radio"
										name="author-type"
										value={option.value}
										checked={authorType === option.value}
										onChange={() => setAuthorType(option.value)}
										disabled={isLoading}
									/>
									<span className="author-type-label">{option.label}</span>
									<span className="author-type-desc">{option.description}</span>
								</label>
							))}
						</div>
					</div>
				</>
			)}

			<div className="form-field">
				<label htmlFor="content">Comment</label>
				<textarea
					id="content"
					ref={textareaRef}
					value={content}
					onChange={(e) => setContent(e.target.value)}
					placeholder="Add a note, feedback, or context..."
					rows={4}
					disabled={isLoading}
				/>
			</div>

			<div className="form-actions">
				<Button type="button" variant="secondary" size="sm" onClick={onCancel} disabled={isLoading}>
					Cancel
				</Button>
				<Button
					type="submit"
					variant="primary"
					size="sm"
					disabled={!canSubmit || isLoading}
					loading={isLoading}
				>
					{isLoading
						? editMode
							? 'Saving...'
							: 'Adding...'
						: editMode
							? 'Save Changes'
							: 'Add Comment'}
				</Button>
			</div>

			<div className="keyboard-hint">
				<kbd>{modifierKey}</kbd> + <kbd>Enter</kbd> to submit
			</div>
		</form>
	);
}

export function CommentsTab({ taskId, phases = [] }: CommentsTabProps) {
	const [comments, setComments] = useState<TaskComment[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [showForm, setShowForm] = useState(false);
	const [isSubmitting, setIsSubmitting] = useState(false);
	const [filterAuthorType, setFilterAuthorType] = useState<TaskCommentAuthorType | ''>('');
	const [editingCommentId, setEditingCommentId] = useState<string | null>(null);

	const loadComments = useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			const data = await getTaskComments(taskId);
			setComments(data);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load comments');
		} finally {
			setLoading(false);
		}
	}, [taskId]);

	useEffect(() => {
		loadComments();
	}, [loadComments]);

	// Filtered comments
	const filteredComments = useMemo(() => {
		if (!filterAuthorType) return comments;
		return comments.filter((c) => c.author_type === filterAuthorType);
	}, [comments, filterAuthorType]);

	// Comments by author type
	const humanComments = useMemo(() => comments.filter((c) => c.author_type === 'human'), [comments]);
	const agentComments = useMemo(() => comments.filter((c) => c.author_type === 'agent'), [comments]);
	const systemComments = useMemo(
		() => comments.filter((c) => c.author_type === 'system'),
		[comments]
	);

	const handleSubmit = useCallback(
		async (comment: CreateTaskCommentRequest) => {
			setIsSubmitting(true);
			try {
				if (editingCommentId) {
					// Update existing comment
					await updateTaskComment(taskId, editingCommentId, {
						content: comment.content,
						phase: comment.phase,
					});
					setEditingCommentId(null);
					toast.success('Comment updated');
				} else {
					// Create new comment
					await createTaskComment(taskId, comment);
					toast.success('Comment added');
				}
				setShowForm(false);
				await loadComments();
			} catch (e) {
				setError(e instanceof Error ? e.message : 'Failed to save comment');
				toast.error('Failed to save comment');
			} finally {
				setIsSubmitting(false);
			}
		},
		[taskId, editingCommentId, loadComments]
	);

	const handleCancel = useCallback(() => {
		setShowForm(false);
		setEditingCommentId(null);
	}, []);

	const handleEdit = useCallback((commentId: string) => {
		setEditingCommentId(commentId);
		setShowForm(true);
	}, []);

	const handleDelete = useCallback(
		async (commentId: string) => {
			if (!confirm('Delete this comment?')) return;

			try {
				await deleteTaskComment(taskId, commentId);
				await loadComments();
				toast.success('Comment deleted');
			} catch (e) {
				setError(e instanceof Error ? e.message : 'Failed to delete comment');
				toast.error('Failed to delete comment');
			}
		},
		[taskId, loadComments]
	);

	const handleAddComment = useCallback(() => {
		setEditingCommentId(null);
		setShowForm(true);
	}, []);

	// Get editing comment for form
	const editingComment = editingCommentId
		? comments.find((c) => c.id === editingCommentId)
		: null;

	return (
		<div className="comments-panel">
			<div className="panel-header">
				<div className="header-left">
					<h3>
						<Icon name="message-square" size={16} />
						Comments
						{comments.length > 0 && <span className="comment-count">{comments.length}</span>}
					</h3>
				</div>
				<div className="header-right">
					{!showForm && (
						<Button
							variant="primary"
							size="sm"
							onClick={handleAddComment}
							leftIcon={<Icon name="plus" size={14} />}
						>
							Add Comment
						</Button>
					)}
				</div>
			</div>

			{error && (
				<div className="error-message">
					<Icon name="alert-circle" size={14} />
					{error}
					<Button variant="ghost" size="sm" onClick={loadComments}>
						Retry
					</Button>
				</div>
			)}

			{showForm && (
				<CommentForm
					phases={phases}
					onSubmit={handleSubmit}
					onCancel={handleCancel}
					isLoading={isSubmitting}
					editMode={!!editingCommentId}
					initialContent={editingComment?.content ?? ''}
					initialPhase={editingComment?.phase}
				/>
			)}

			{comments.length > 0 && !showForm && (
				<div className="filter-bar">
					<span className="filter-label">Filter:</span>
					<Button
						variant="ghost"
						size="sm"
						className={`filter-btn ${filterAuthorType === '' ? 'active' : ''}`}
						onClick={() => setFilterAuthorType('')}
					>
						All ({comments.length})
					</Button>
					{humanComments.length > 0 && (
						<Button
							variant="ghost"
							size="sm"
							className={`filter-btn human ${filterAuthorType === 'human' ? 'active' : ''}`}
							onClick={() => setFilterAuthorType('human')}
						>
							Human ({humanComments.length})
						</Button>
					)}
					{agentComments.length > 0 && (
						<Button
							variant="ghost"
							size="sm"
							className={`filter-btn agent ${filterAuthorType === 'agent' ? 'active' : ''}`}
							onClick={() => setFilterAuthorType('agent')}
						>
							Agent ({agentComments.length})
						</Button>
					)}
					{systemComments.length > 0 && (
						<Button
							variant="ghost"
							size="sm"
							className={`filter-btn system ${filterAuthorType === 'system' ? 'active' : ''}`}
							onClick={() => setFilterAuthorType('system')}
						>
							System ({systemComments.length})
						</Button>
					)}
				</div>
			)}

			{loading ? (
				<div className="loading-state">
					<div className="spinner" />
					<span>Loading comments...</span>
				</div>
			) : comments.length === 0 && !showForm ? (
				<div className="empty-state">
					<Icon name="message-square" size={32} />
					<p>No comments yet</p>
					<span>Add comments to track feedback, notes, and context.</span>
				</div>
			) : !showForm ? (
				<div className="comments-list">
					{filteredComments.map((comment) => (
						<CommentThread
							key={comment.id}
							comment={comment}
							onEdit={handleEdit}
							onDelete={handleDelete}
						/>
					))}
				</div>
			) : null}
		</div>
	);
}
