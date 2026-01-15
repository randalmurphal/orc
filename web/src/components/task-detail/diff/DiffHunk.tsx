import { Fragment } from 'react';
import { InlineCommentThread } from './InlineCommentThread';
import type { Hunk, ReviewComment, CreateCommentRequest } from '@/lib/types';
import './DiffHunk.css';

interface DiffHunkProps {
	hunk: Hunk;
	filePath: string;
	viewMode: 'split' | 'unified';
	comments: ReviewComment[];
	activeLineNumber: number | null;
	onLineClick: (lineNumber: number, filePath: string) => void;
	onAddComment: (comment: CreateCommentRequest) => Promise<void>;
	onResolveComment: (id: string) => void;
	onWontFixComment: (id: string) => void;
	onDeleteComment: (id: string) => void;
	onCloseThread: () => void;
}

export function DiffHunk({
	hunk,
	filePath,
	viewMode,
	comments,
	activeLineNumber,
	onLineClick,
	onAddComment,
	onResolveComment,
	onWontFixComment,
	onDeleteComment,
	onCloseThread,
}: DiffHunkProps) {
	return (
		<div className="diff-hunk">
			{/* Hunk Header */}
			<div className="hunk-header">
				@@ -{hunk.old_start},{hunk.old_lines} +{hunk.new_start},{hunk.new_lines} @@
			</div>

			{/* Lines */}
			<div className={`hunk-lines ${viewMode}`}>
				{viewMode === 'split' ? (
					<SplitView
						hunk={hunk}
						filePath={filePath}
						comments={comments}
						activeLineNumber={activeLineNumber}
						onLineClick={onLineClick}
						onAddComment={onAddComment}
						onResolveComment={onResolveComment}
						onWontFixComment={onWontFixComment}
						onDeleteComment={onDeleteComment}
						onCloseThread={onCloseThread}
					/>
				) : (
					<UnifiedView
						hunk={hunk}
						filePath={filePath}
						comments={comments}
						activeLineNumber={activeLineNumber}
						onLineClick={onLineClick}
						onAddComment={onAddComment}
						onResolveComment={onResolveComment}
						onWontFixComment={onWontFixComment}
						onDeleteComment={onDeleteComment}
						onCloseThread={onCloseThread}
					/>
				)}
			</div>
		</div>
	);
}

// Split View (side-by-side)
interface ViewProps {
	hunk: Hunk;
	filePath: string;
	comments: ReviewComment[];
	activeLineNumber: number | null;
	onLineClick: (lineNumber: number, filePath: string) => void;
	onAddComment: (comment: CreateCommentRequest) => Promise<void>;
	onResolveComment: (id: string) => void;
	onWontFixComment: (id: string) => void;
	onDeleteComment: (id: string) => void;
	onCloseThread: () => void;
}

function SplitView({
	hunk,
	filePath,
	comments,
	activeLineNumber,
	onLineClick,
	onAddComment,
	onResolveComment,
	onWontFixComment,
	onDeleteComment,
	onCloseThread,
}: ViewProps) {
	// Build pairs for split view
	const pairs: Array<{
		left: typeof hunk.lines[0] | null;
		right: typeof hunk.lines[0] | null;
	}> = [];

	let leftQueue: typeof hunk.lines = [];
	let rightQueue: typeof hunk.lines = [];

	for (const line of hunk.lines) {
		if (line.type === 'context') {
			// Flush queues
			while (leftQueue.length || rightQueue.length) {
				pairs.push({
					left: leftQueue.shift() ?? null,
					right: rightQueue.shift() ?? null,
				});
			}
			pairs.push({ left: line, right: line });
		} else if (line.type === 'deletion') {
			leftQueue.push(line);
		} else if (line.type === 'addition') {
			rightQueue.push(line);
		}
	}

	// Flush remaining
	while (leftQueue.length || rightQueue.length) {
		pairs.push({
			left: leftQueue.shift() ?? null,
			right: rightQueue.shift() ?? null,
		});
	}

	const getLineComments = (lineNumber?: number) =>
		lineNumber ? comments.filter((c) => c.line_number === lineNumber) : [];

	return (
		<table className="split-table">
			<tbody>
				{pairs.map((pair, index) => {
					const leftLine = pair.left;
					const rightLine = pair.right;
					const leftLineNum = leftLine?.old_line;
					const rightLineNum = rightLine?.new_line;
					const lineComments =
						rightLineNum
							? getLineComments(rightLineNum)
							: leftLineNum
							? getLineComments(leftLineNum)
							: [];
					const isActive = Boolean(
						(rightLineNum && activeLineNumber === rightLineNum) ||
						(leftLineNum && activeLineNumber === leftLineNum)
					);

					return (
						<Fragment key={index}>
							<tr className="split-row">
								{/* Left side (old) */}
								<td className="line-number old">{leftLine?.old_line ?? ''}</td>
								<td
									className={`line-content old ${leftLine?.type ?? 'empty'}`}
									onClick={() =>
										leftLine?.old_line && onLineClick(leftLine.old_line, filePath)
									}
								>
									{leftLine?.type === 'deletion' && <span className="prefix">-</span>}
									{leftLine?.content ?? ''}
								</td>

								{/* Right side (new) */}
								<td className="line-number new">{rightLine?.new_line ?? ''}</td>
								<td
									className={`line-content new ${rightLine?.type ?? 'empty'}`}
									onClick={() =>
										rightLine?.new_line && onLineClick(rightLine.new_line, filePath)
									}
								>
									{rightLine?.type === 'addition' && <span className="prefix">+</span>}
									{rightLine?.content ?? ''}
								</td>
							</tr>
							{/* Comment thread */}
							{(isActive || lineComments.length > 0) && (
								<tr className="comment-row">
									<td colSpan={4}>
										<InlineCommentThread
											filePath={filePath}
											lineNumber={rightLineNum ?? leftLineNum ?? 0}
											comments={lineComments}
											showForm={isActive}
											onAddComment={onAddComment}
											onResolveComment={onResolveComment}
											onWontFixComment={onWontFixComment}
											onDeleteComment={onDeleteComment}
											onCloseThread={onCloseThread}
										/>
									</td>
								</tr>
							)}
						</Fragment>
					);
				})}
			</tbody>
		</table>
	);
}

// Unified View
function UnifiedView({
	hunk,
	filePath,
	comments,
	activeLineNumber,
	onLineClick,
	onAddComment,
	onResolveComment,
	onWontFixComment,
	onDeleteComment,
	onCloseThread,
}: ViewProps) {
	const getLineComments = (lineNumber?: number) =>
		lineNumber ? comments.filter((c) => c.line_number === lineNumber) : [];

	return (
		<table className="unified-table">
			<tbody>
				{hunk.lines.map((line, index) => {
					const lineNum = line.new_line ?? line.old_line;
					const lineComments = getLineComments(lineNum);
					const isActive = Boolean(lineNum && activeLineNumber === lineNum);

					return (
						<Fragment key={index}>
							<tr className={`unified-row ${line.type}`}>
								<td className="line-number old">{line.old_line ?? ''}</td>
								<td className="line-number new">{line.new_line ?? ''}</td>
								<td
									className={`line-content ${line.type}`}
									onClick={() => lineNum && onLineClick(lineNum, filePath)}
								>
									<span className="prefix">
										{line.type === 'addition' ? '+' : line.type === 'deletion' ? '-' : ' '}
									</span>
									{line.content}
								</td>
							</tr>
							{/* Comment thread */}
							{(isActive || lineComments.length > 0) && (
								<tr className="comment-row">
									<td colSpan={3}>
										<InlineCommentThread
											filePath={filePath}
											lineNumber={lineNum ?? 0}
											comments={lineComments}
											showForm={isActive}
											onAddComment={onAddComment}
											onResolveComment={onResolveComment}
											onWontFixComment={onWontFixComment}
											onDeleteComment={onDeleteComment}
											onCloseThread={onCloseThread}
										/>
									</td>
								</tr>
							)}
						</Fragment>
					);
				})}
			</tbody>
		</table>
	);
}
