import { useMemo } from 'react';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { DiffHunk } from './DiffHunk';
import type { FileDiff } from '@/gen/orc/v1/common_pb';
import type { ReviewComment } from '@/gen/orc/v1/task_pb';
import { CommentStatus } from '@/gen/orc/v1/task_pb';
import type { CreateCommentRequest } from './types';
import './DiffFile.css';

interface DiffFileProps {
	file: FileDiff;
	expanded: boolean;
	viewMode: 'split' | 'unified';
	comments: ReviewComment[];
	activeLineNumber: number | null;
	onToggle: () => void;
	onLineClick: (lineNumber: number, filePath: string) => void;
	onAddComment: (comment: CreateCommentRequest) => Promise<void>;
	onResolveComment: (id: string) => void;
	onWontFixComment: (id: string) => void;
	onDeleteComment: (id: string) => void;
	onCloseThread: () => void;
}

export function DiffFile({
	file,
	expanded,
	viewMode,
	comments,
	activeLineNumber,
	onToggle,
	onLineClick,
	onAddComment,
	onResolveComment,
	onWontFixComment,
	onDeleteComment,
	onCloseThread,
}: DiffFileProps) {
	// Filter comments for this file
	const fileComments = useMemo(
		() => comments.filter((c) => c.filePath === file.path),
		[comments, file.path]
	);

	// Get status icon
	const getStatusIcon = () => {
		switch (file.status) {
			case 'added':
				return 'plus';
			case 'deleted':
				return 'trash';
			case 'renamed':
				return 'arrow-left';
			default:
				return 'edit';
		}
	};

	// Get status class
	const getStatusClass = () => {
		switch (file.status) {
			case 'added':
				return 'added';
			case 'deleted':
				return 'deleted';
			case 'renamed':
				return 'renamed';
			default:
				return 'modified';
		}
	};

	return (
		<div className={`diff-file ${getStatusClass()}`}>
			{/* File Header */}
			<Button variant="ghost" className="file-header" onClick={onToggle}>
				<Icon name={expanded ? 'chevron-down' : 'chevron-right'} size={16} />
				<Icon name={getStatusIcon()} size={14} className={`status-icon ${getStatusClass()}`} />
				<span className="file-path">
					{file.status === 'renamed' && file.oldPath ? (
						<>
							<span className="old-path">{file.oldPath}</span>
							<Icon name="arrow-left" size={12} className="rename-arrow" />
						</>
					) : null}
					{file.path}
				</span>
				{file.binary && <span className="binary-badge">Binary</span>}
				<div className="file-stats">
					{file.additions > 0 && <span className="additions">+{file.additions}</span>}
					{file.deletions > 0 && <span className="deletions">-{file.deletions}</span>}
				</div>
				{fileComments.filter((c) => c.status === CommentStatus.OPEN).length > 0 && (
					<span className="comment-count">
						{fileComments.filter((c) => c.status === CommentStatus.OPEN).length}
					</span>
				)}
			</Button>

			{/* File Content */}
			{expanded && (
				<div className="file-content">
					{file.binary ? (
						<div className="binary-notice">Binary file not shown</div>
					) : file.loadError ? (
						<div className="load-error">
							<Icon name="alert-circle" size={16} />
							<span>{file.loadError}</span>
						</div>
					) : file.hunks && file.hunks.length > 0 ? (
						<div className={`diff-content ${viewMode}`}>
							{file.hunks.map((hunk, index) => (
								<DiffHunk
									key={index}
									hunk={hunk}
									filePath={file.path}
									viewMode={viewMode}
									comments={fileComments}
									activeLineNumber={activeLineNumber}
									onLineClick={onLineClick}
									onAddComment={onAddComment}
									onResolveComment={onResolveComment}
									onWontFixComment={onWontFixComment}
									onDeleteComment={onDeleteComment}
									onCloseThread={onCloseThread}
								/>
							))}
						</div>
					) : (
						<div className="loading-hunks">
							<div className="loading-spinner" />
							<span>Loading diff...</span>
						</div>
					)}
				</div>
			)}
		</div>
	);
}
