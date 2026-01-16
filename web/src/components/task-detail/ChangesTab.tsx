import { useState, useEffect, useCallback } from 'react';
import { Icon } from '@/components/ui/Icon';
import { DiffFile } from '@/components/task-detail/diff/DiffFile';
import { DiffStats } from '@/components/task-detail/diff/DiffStats';
import {
	getReviewComments,
	createReviewComment,
	updateReviewComment,
	deleteReviewComment,
	triggerReviewRetry,
} from '@/lib/api';
import { toast } from '@/stores/uiStore';
import type { DiffResult, FileDiff, ReviewComment, CreateCommentRequest } from '@/lib/types';
import './ChangesTab.css';

interface ChangesTabProps {
	taskId: string;
}

export function ChangesTab({ taskId }: ChangesTabProps) {
	const [diff, setDiff] = useState<DiffResult | null>(null);
	const [comments, setComments] = useState<ReviewComment[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [viewMode, setViewMode] = useState<'split' | 'unified'>('split');
	const [expandedFiles, setExpandedFiles] = useState<Set<string>>(new Set());
	const [activeLineNumber, setActiveLineNumber] = useState<number | null>(null);
	const [activeFilePath, setActiveFilePath] = useState<string | null>(null);
	const [sendingToAgent, setSendingToAgent] = useState(false);
	const [showGeneralCommentForm, setShowGeneralCommentForm] = useState(false);
	const [generalCommentContent, setGeneralCommentContent] = useState('');
	const [generalCommentSeverity, setGeneralCommentSeverity] = useState<'suggestion' | 'issue' | 'blocker'>('issue');
	const [addingGeneralComment, setAddingGeneralComment] = useState(false);

	// Comment stats
	const openComments = comments.filter((c) => c.status === 'open');
	const blockerCount = openComments.filter((c) => c.severity === 'blocker').length;
	const issueCount = openComments.filter((c) => c.severity === 'issue').length;
	const suggestionCount = openComments.filter((c) => c.severity === 'suggestion').length;
	const hasBlockers = blockerCount > 0;

	// General comments (not tied to a specific line)
	const generalComments = comments.filter((c) => !c.file_path && !c.line_number);

	// Load diff
	const loadDiff = useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			const res = await fetch(`/api/tasks/${taskId}/diff?files=true`);
			if (!res.ok) throw new Error('Failed to load diff');
			const data = await res.json();
			setDiff(data);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Unknown error');
		} finally {
			setLoading(false);
		}
	}, [taskId]);

	// Load comments
	const loadComments = useCallback(async () => {
		try {
			const data = await getReviewComments(taskId);
			setComments(data);
		} catch (e) {
			console.error('Failed to load comments:', e);
		}
	}, [taskId]);

	useEffect(() => {
		Promise.all([loadDiff(), loadComments()]);
	}, [loadDiff, loadComments]);

	// Load file hunks
	const loadFileHunks = useCallback(async (filePath: string) => {
		try {
			const res = await fetch(`/api/tasks/${taskId}/diff/file/${encodeURIComponent(filePath)}`);
			if (!res.ok) {
				const errorMsg = `Failed to load file diff (${res.status})`;
				setDiff((prev) =>
					prev
						? {
								...prev,
								files: prev.files.map((f) =>
									f.path === filePath ? { ...f, loadError: errorMsg } : f
								),
						  }
						: null
				);
				return;
			}
			const fileDiff = (await res.json()) as FileDiff;
			setDiff((prev) =>
				prev
					? {
							...prev,
							files: prev.files.map((f) =>
								f.path === filePath ? { ...f, hunks: fileDiff.hunks, loadError: undefined } : f
							),
					  }
					: null
			);
		} catch (e) {
			const errorMsg = e instanceof Error ? e.message : 'Unknown error loading file';
			setDiff((prev) =>
				prev
					? {
							...prev,
							files: prev.files.map((f) =>
								f.path === filePath ? { ...f, loadError: errorMsg } : f
							),
					  }
					: null
			);
		}
	}, [taskId]);

	// Toggle file expansion
	const toggleFile = useCallback((path: string) => {
		const file = diff?.files.find((f) => f.path === path);
		if (!file?.hunks?.length && !file?.loadError) {
			loadFileHunks(path);
		}

		setExpandedFiles((prev) => {
			const next = new Set(prev);
			if (next.has(path)) {
				next.delete(path);
			} else {
				next.add(path);
			}
			return next;
		});
	}, [diff, loadFileHunks]);

	// Expand/collapse all
	const expandAll = useCallback(() => {
		if (!diff) return;
		for (const file of diff.files) {
			if (!file.hunks?.length) {
				loadFileHunks(file.path);
			}
		}
		setExpandedFiles(new Set(diff.files.map((f) => f.path)));
	}, [diff, loadFileHunks]);

	const collapseAll = useCallback(() => {
		setExpandedFiles(new Set());
	}, []);

	const allExpanded = diff ? expandedFiles.size === diff.files.length : false;

	// Handle line click for comments
	const handleLineClick = useCallback((lineNumber: number, filePath: string) => {
		if (activeLineNumber === lineNumber && activeFilePath === filePath) {
			setActiveLineNumber(null);
			setActiveFilePath(null);
		} else {
			setActiveLineNumber(lineNumber);
			setActiveFilePath(filePath);
		}
	}, [activeLineNumber, activeFilePath]);

	const handleCloseThread = useCallback(() => {
		setActiveLineNumber(null);
		setActiveFilePath(null);
	}, []);

	// Comment handlers
	const handleAddComment = useCallback(async (comment: CreateCommentRequest) => {
		try {
			const newComment = await createReviewComment(taskId, comment);
			setComments((prev) => [...prev, newComment]);
			setActiveLineNumber(null);
			setActiveFilePath(null);
			toast.success('Comment added');
		} catch (e) {
			toast.error('Failed to add comment');
			throw e;
		}
	}, [taskId]);

	const handleResolveComment = useCallback(async (id: string) => {
		try {
			const updated = await updateReviewComment(taskId, id, { status: 'resolved' });
			setComments((prev) => prev.map((c) => (c.id === id ? updated : c)));
			toast.success('Comment resolved');
		} catch (_e) {
			toast.error('Failed to resolve comment');
		}
	}, [taskId]);

	const handleWontFixComment = useCallback(async (id: string) => {
		try {
			const updated = await updateReviewComment(taskId, id, { status: 'wont_fix' });
			setComments((prev) => prev.map((c) => (c.id === id ? updated : c)));
			toast.success("Marked as won't fix");
		} catch (_e) {
			toast.error('Failed to update comment');
		}
	}, [taskId]);

	const handleDeleteComment = useCallback(async (id: string) => {
		try {
			await deleteReviewComment(taskId, id);
			setComments((prev) => prev.filter((c) => c.id !== id));
			toast.success('Comment deleted');
		} catch (_e) {
			toast.error('Failed to delete comment');
		}
	}, [taskId]);

	// Add general comment
	const handleAddGeneralComment = useCallback(async () => {
		if (!generalCommentContent.trim() || addingGeneralComment) return;
		setAddingGeneralComment(true);
		try {
			const newComment = await createReviewComment(taskId, {
				content: generalCommentContent.trim(),
				severity: generalCommentSeverity,
			});
			setComments((prev) => [...prev, newComment]);
			setGeneralCommentContent('');
			setGeneralCommentSeverity('issue');
			setShowGeneralCommentForm(false);
			toast.success('Comment added');
		} catch (_e) {
			toast.error('Failed to add comment');
		} finally {
			setAddingGeneralComment(false);
		}
	}, [taskId, generalCommentContent, generalCommentSeverity, addingGeneralComment]);

	// Send to agent
	const handleSendToAgent = useCallback(async () => {
		if (openComments.length === 0 || sendingToAgent) return;
		setSendingToAgent(true);
		try {
			await triggerReviewRetry(taskId);
			toast.success('Comments sent to agent for review');
		} catch (_e) {
			toast.error('Failed to send comments to agent');
		} finally {
			setSendingToAgent(false);
		}
	}, [taskId, openComments.length, sendingToAgent]);

	// Render loading state
	if (loading) {
		return (
			<div className="changes-tab">
				<div className="changes-loading">
					<div className="loading-spinner" />
					<span>Loading diff...</span>
				</div>
			</div>
		);
	}

	// Render error state
	if (error) {
		return (
			<div className="changes-tab">
				<div className="changes-error">
					<Icon name="alert-circle" size={24} />
					<span>{error}</span>
				</div>
			</div>
		);
	}

	// Render empty state
	if (!diff || diff.files.length === 0) {
		return (
			<div className="changes-tab">
				<div className="changes-empty">
					<Icon name="branch" size={32} />
					<h3>No Changes</h3>
					<p>No code changes to display for this task.</p>
				</div>
			</div>
		);
	}

	return (
		<div className="changes-tab">
			{/* Toolbar */}
			<div className="diff-toolbar">
				<div className="toolbar-left">
					{/* View Mode Toggle */}
					<div className="view-toggle" role="tablist" aria-label="Diff view mode">
						<button
							role="tab"
							aria-selected={viewMode === 'split'}
							className={viewMode === 'split' ? 'active' : ''}
							onClick={() => setViewMode('split')}
						>
							Split
						</button>
						<button
							role="tab"
							aria-selected={viewMode === 'unified'}
							className={viewMode === 'unified' ? 'active' : ''}
							onClick={() => setViewMode('unified')}
						>
							Unified
						</button>
					</div>

					{/* Expand/Collapse */}
					<button className="expand-btn" onClick={() => (allExpanded ? collapseAll() : expandAll())}>
						{allExpanded ? 'Collapse all' : 'Expand all'}
					</button>
				</div>

				<div className="toolbar-right">
					{/* Review Summary */}
					{openComments.length > 0 && (
						<>
							<div className={`review-summary ${hasBlockers ? 'has-blockers' : ''}`}>
								{blockerCount > 0 && (
									<span className="count blocker">
										{blockerCount} blocker{blockerCount > 1 ? 's' : ''}
									</span>
								)}
								{issueCount > 0 && (
									<span className="count issue">
										{issueCount} issue{issueCount > 1 ? 's' : ''}
									</span>
								)}
								{suggestionCount > 0 && (
									<span className="count suggestion">
										{suggestionCount} suggestion{suggestionCount > 1 ? 's' : ''}
									</span>
								)}
							</div>
							<button
								className="send-to-agent-btn"
								onClick={handleSendToAgent}
								disabled={sendingToAgent}
							>
								{sendingToAgent ? (
									<>
										<span className="btn-spinner" />
										Sending...
									</>
								) : (
									<>
										<Icon name="play" size={14} />
										Send to Agent
									</>
								)}
							</button>
						</>
					)}

					<DiffStats stats={diff.stats} />
				</div>
			</div>

			{/* File List */}
			<div className="file-list">
				{diff.files.map((file) => (
					<DiffFile
						key={file.path}
						file={file}
						expanded={expandedFiles.has(file.path)}
						viewMode={viewMode}
						comments={comments}
						activeLineNumber={activeFilePath === file.path ? activeLineNumber : null}
						onToggle={() => toggleFile(file.path)}
						onLineClick={handleLineClick}
						onAddComment={handleAddComment}
						onResolveComment={handleResolveComment}
						onWontFixComment={handleWontFixComment}
						onDeleteComment={handleDeleteComment}
						onCloseThread={handleCloseThread}
					/>
				))}
			</div>

			{/* General Comments Section */}
			<div className="general-comments-section">
				<div className="general-comments-header">
					<h3>General Comments</h3>
					{!showGeneralCommentForm && (
						<button className="add-general-btn" onClick={() => setShowGeneralCommentForm(true)}>
							<span>+</span> Add Comment
						</button>
					)}
				</div>

				{/* Open general comments */}
				{generalComments.filter((c) => c.status === 'open').length > 0 && (
					<div className="general-comments-list">
						{generalComments
							.filter((c) => c.status === 'open')
							.map((comment) => (
								<div key={comment.id} className="general-comment">
									<div className="comment-header">
										<span className={`severity-badge ${comment.severity}`}>
											{comment.severity}
										</span>
										<span className="timestamp">
											{new Date(comment.created_at).toLocaleString()}
										</span>
									</div>
									<div className="comment-content">{comment.content}</div>
									<div className="comment-actions">
										<button
											className="action-btn"
											onClick={() => handleResolveComment(comment.id)}
										>
											Resolve
										</button>
										<button
											className="action-btn"
											onClick={() => handleWontFixComment(comment.id)}
										>
											Won't Fix
										</button>
										<button
											className="action-btn delete"
											onClick={() => handleDeleteComment(comment.id)}
										>
											Delete
										</button>
									</div>
								</div>
							))}
					</div>
				)}

				{/* Add comment form */}
				{showGeneralCommentForm && (
					<div className="general-comment-form">
						<div className="severity-pills">
							{(['suggestion', 'issue', 'blocker'] as const).map((sev) => (
								<button
									key={sev}
									type="button"
									className={`severity-pill ${generalCommentSeverity === sev ? 'selected' : ''}`}
									onClick={() => setGeneralCommentSeverity(sev)}
								>
									{sev}
								</button>
							))}
						</div>
						<textarea
							value={generalCommentContent}
							onChange={(e) => setGeneralCommentContent(e.target.value)}
							placeholder="Add a general comment about this change..."
							rows={3}
							disabled={addingGeneralComment}
						/>
						<div className="form-actions">
							<button
								className="cancel-btn"
								onClick={() => {
									setShowGeneralCommentForm(false);
									setGeneralCommentContent('');
								}}
							>
								Cancel
							</button>
							<button
								className="submit-btn"
								onClick={handleAddGeneralComment}
								disabled={!generalCommentContent.trim() || addingGeneralComment}
							>
								{addingGeneralComment ? 'Adding...' : 'Add Comment'}
							</button>
						</div>
					</div>
				)}
			</div>
		</div>
	);
}
