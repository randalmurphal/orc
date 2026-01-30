import { useState, useEffect, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { DiffFile } from '@/components/task-detail/diff/DiffFile';
import { DiffStats } from '@/components/task-detail/diff/DiffStats';
import { taskClient } from '@/lib/client';
import {
	type ReviewComment,
	CommentSeverity,
	CommentStatus,
	GetDiffRequestSchema,
	GetFileDiffRequestSchema,
	ListReviewCommentsRequestSchema,
	CreateReviewCommentRequestSchema,
	UpdateReviewCommentRequestSchema,
	DeleteReviewCommentRequestSchema,
	RetryTaskRequestSchema,
} from '@/gen/orc/v1/task_pb';
import type { DiffResult } from '@/gen/orc/v1/common_pb';
import type { CreateCommentRequest } from '@/components/task-detail/diff/types';
import { timestampToDate } from '@/lib/time';
import { toast } from '@/stores/uiStore';
import { useCurrentProjectId } from '@/stores';
import './ChangesTab.css';

// Helper to convert CommentSeverity enum to string label
function getSeverityLabel(severity: CommentSeverity): string {
	switch (severity) {
		case CommentSeverity.BLOCKER:
			return 'blocker';
		case CommentSeverity.ISSUE:
			return 'issue';
		case CommentSeverity.SUGGESTION:
			return 'suggestion';
		default:
			return 'issue';
	}
}


interface ChangesTabProps {
	taskId: string;
}

export function ChangesTab({ taskId }: ChangesTabProps) {
	const projectId = useCurrentProjectId();
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
	const [generalCommentSeverity, setGeneralCommentSeverity] = useState<CommentSeverity>(CommentSeverity.ISSUE);
	const [addingGeneralComment, setAddingGeneralComment] = useState(false);

	// Comment stats
	const openComments = comments.filter((c) => c.status === CommentStatus.OPEN);
	const blockerCount = openComments.filter((c) => c.severity === CommentSeverity.BLOCKER).length;
	const issueCount = openComments.filter((c) => c.severity === CommentSeverity.ISSUE).length;
	const suggestionCount = openComments.filter((c) => c.severity === CommentSeverity.SUGGESTION).length;
	const hasBlockers = blockerCount > 0;

	// General comments (not tied to a specific line)
	const generalComments = comments.filter((c) => !c.filePath && !c.lineNumber);

	// Load diff
	const loadDiff = useCallback(async () => {
		if (!projectId) return;
		setLoading(true);
		setError(null);
		try {
			const response = await taskClient.getDiff(
				create(GetDiffRequestSchema, { projectId, taskId })
			);
			if (response.diff) {
				setDiff(response.diff);
			}
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Unknown error');
		} finally {
			setLoading(false);
		}
	}, [projectId, taskId]);

	// Load comments
	const loadComments = useCallback(async () => {
		if (!projectId) return;
		try {
			const response = await taskClient.listReviewComments(
				create(ListReviewCommentsRequestSchema, { projectId, taskId })
			);
			setComments(response.comments);
		} catch (e) {
			console.error('Failed to load comments:', e);
		}
	}, [projectId, taskId]);

	useEffect(() => {
		Promise.all([loadDiff(), loadComments()]);
	}, [loadDiff, loadComments]);

	// Load file hunks using Connect RPC
	const loadFileHunks = useCallback(async (filePath: string) => {
		if (!projectId) return;
		try {
			const response = await taskClient.getFileDiff(
				create(GetFileDiffRequestSchema, { projectId, taskId, filePath })
			);
			if (response.file) {
				setDiff((prev) =>
					prev
						? {
								...prev,
								files: prev.files.map((f) =>
									f.path === filePath ? { ...f, hunks: response.file!.hunks, loadError: undefined } : f
								),
						  }
						: null
				);
			}
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
	}, [projectId, taskId]);

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
		if (!projectId) return;
		try {
			const response = await taskClient.createReviewComment(
				create(CreateReviewCommentRequestSchema, {
					projectId,
					taskId,
					content: comment.content,
					severity: comment.severity,
					filePath: comment.filePath,
					lineNumber: comment.lineNumber,
				})
			);
			if (response.comment) {
				setComments((prev) => [...prev, response.comment!]);
			}
			setActiveLineNumber(null);
			setActiveFilePath(null);
			toast.success('Comment added');
		} catch (e) {
			toast.error('Failed to add comment');
			throw e;
		}
	}, [projectId, taskId]);

	const handleResolveComment = useCallback(async (id: string) => {
		if (!projectId) return;
		try {
			const response = await taskClient.updateReviewComment(
				create(UpdateReviewCommentRequestSchema, { projectId, taskId, commentId: id, status: CommentStatus.RESOLVED })
			);
			if (response.comment) {
				setComments((prev) => prev.map((c) => (c.id === id ? response.comment! : c)));
			}
			toast.success('Comment resolved');
		} catch (_e) {
			toast.error('Failed to resolve comment');
		}
	}, [projectId, taskId]);

	const handleWontFixComment = useCallback(async (id: string) => {
		if (!projectId) return;
		try {
			const response = await taskClient.updateReviewComment(
				create(UpdateReviewCommentRequestSchema, { projectId, taskId, commentId: id, status: CommentStatus.WONT_FIX })
			);
			if (response.comment) {
				setComments((prev) => prev.map((c) => (c.id === id ? response.comment! : c)));
			}
			toast.success("Marked as won't fix");
		} catch (_e) {
			toast.error('Failed to update comment');
		}
	}, [projectId, taskId]);

	const handleDeleteComment = useCallback(async (id: string) => {
		if (!projectId) return;
		try {
			await taskClient.deleteReviewComment(
				create(DeleteReviewCommentRequestSchema, { projectId, taskId, commentId: id })
			);
			setComments((prev) => prev.filter((c) => c.id !== id));
			toast.success('Comment deleted');
		} catch (_e) {
			toast.error('Failed to delete comment');
		}
	}, [projectId, taskId]);

	// Add general comment
	const handleAddGeneralComment = useCallback(async () => {
		if (!projectId || !generalCommentContent.trim() || addingGeneralComment) return;
		setAddingGeneralComment(true);
		try {
			const response = await taskClient.createReviewComment(
				create(CreateReviewCommentRequestSchema, {
					projectId,
					taskId,
					content: generalCommentContent.trim(),
					severity: generalCommentSeverity,
				})
			);
			if (response.comment) {
				setComments((prev) => [...prev, response.comment!]);
			}
			setGeneralCommentContent('');
			setGeneralCommentSeverity(CommentSeverity.ISSUE);
			setShowGeneralCommentForm(false);
			toast.success('Comment added');
		} catch (_e) {
			toast.error('Failed to add comment');
		} finally {
			setAddingGeneralComment(false);
		}
	}, [projectId, taskId, generalCommentContent, generalCommentSeverity, addingGeneralComment]);

	// Send review comments to agent for retry
	const handleSendToAgent = useCallback(async () => {
		if (!projectId || openComments.length === 0 || sendingToAgent) return;
		setSendingToAgent(true);
		try {
			await taskClient.retryTask(
				create(RetryTaskRequestSchema, { projectId, taskId, includeReviewComments: true })
			);
			toast.success('Comments sent to agent for review');
		} catch (_e) {
			toast.error('Failed to send comments to agent');
		} finally {
			setSendingToAgent(false);
		}
	}, [projectId, taskId, openComments.length, sendingToAgent]);

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
						<Button
							variant={viewMode === 'split' ? 'primary' : 'ghost'}
							size="sm"
							onClick={() => setViewMode('split')}
							aria-selected={viewMode === 'split'}
						>
							Split
						</Button>
						<Button
							variant={viewMode === 'unified' ? 'primary' : 'ghost'}
							size="sm"
							onClick={() => setViewMode('unified')}
							aria-selected={viewMode === 'unified'}
						>
							Unified
						</Button>
					</div>

					{/* Expand/Collapse */}
					<Button variant="ghost" size="sm" className="expand-btn" onClick={() => (allExpanded ? collapseAll() : expandAll())}>
						{allExpanded ? 'Collapse all' : 'Expand all'}
					</Button>
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
							<Button
								variant="primary"
								size="sm"
								className="send-to-agent-btn"
								onClick={handleSendToAgent}
								loading={sendingToAgent}
								leftIcon={<Icon name="play" size={14} />}
							>
								Send to Agent
							</Button>
						</>
					)}

					{diff.stats && <DiffStats stats={diff.stats} />}
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
						<Button variant="secondary" size="sm" className="add-general-btn" onClick={() => setShowGeneralCommentForm(true)}>
							<span>+</span> Add Comment
						</Button>
					)}
				</div>

				{/* Open general comments */}
				{generalComments.filter((c) => c.status === CommentStatus.OPEN).length > 0 && (
					<div className="general-comments-list">
						{generalComments
							.filter((c) => c.status === CommentStatus.OPEN)
							.map((comment) => {
								const severityLabel = getSeverityLabel(comment.severity);
								const createdDate = timestampToDate(comment.createdAt);
								return (
								<div key={comment.id} className="general-comment">
									<div className="comment-header">
										<span className={`severity-badge ${severityLabel}`}>
											{severityLabel}
										</span>
										<span className="timestamp">
											{createdDate?.toLocaleString() ?? 'N/A'}
										</span>
									</div>
									<div className="comment-content">{comment.content}</div>
									<div className="comment-actions">
										<Button
											variant="ghost"
											size="sm"
											className="action-btn"
											onClick={() => handleResolveComment(comment.id)}
										>
											Resolve
										</Button>
										<Button
											variant="ghost"
											size="sm"
											className="action-btn"
											onClick={() => handleWontFixComment(comment.id)}
										>
											Won't Fix
										</Button>
										<Button
											variant="danger"
											size="sm"
											className="action-btn delete"
											onClick={() => handleDeleteComment(comment.id)}
										>
											Delete
										</Button>
									</div>
								</div>
								);
							})}
					</div>
				)}

				{/* Add comment form */}
				{showGeneralCommentForm && (
					<div className="general-comment-form">
						<div className="severity-pills">
							{([
								{ value: CommentSeverity.SUGGESTION, label: 'suggestion' },
								{ value: CommentSeverity.ISSUE, label: 'issue' },
								{ value: CommentSeverity.BLOCKER, label: 'blocker' },
							]).map((sev) => (
								<Button
									key={sev.label}
									variant={generalCommentSeverity === sev.value ? 'primary' : 'ghost'}
									size="sm"
									className={`severity-pill ${generalCommentSeverity === sev.value ? 'selected' : ''}`}
									onClick={() => setGeneralCommentSeverity(sev.value)}
								>
									{sev.label}
								</Button>
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
							<Button
								variant="secondary"
								onClick={() => {
									setShowGeneralCommentForm(false);
									setGeneralCommentContent('');
								}}
							>
								Cancel
							</Button>
							<Button
								variant="primary"
								onClick={handleAddGeneralComment}
								disabled={!generalCommentContent.trim()}
								loading={addingGeneralComment}
							>
								Add Comment
							</Button>
						</div>
					</div>
				)}
			</div>
		</div>
	);
}
