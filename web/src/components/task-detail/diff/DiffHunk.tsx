import { Fragment, useState, useCallback } from 'react';
import { InlineCommentThread } from './InlineCommentThread';
import { FeedbackIndicator } from './FeedbackIndicator';
import { FeedbackType, FeedbackTiming, type Feedback } from '@/gen/orc/v1/feedback_pb';
import type { DiffHunk as Hunk, DiffLine } from '@/gen/orc/v1/common_pb';
import type { ReviewComment } from '@/gen/orc/v1/task_pb';
import type { CreateCommentRequest } from './types';
import './DiffHunk.css';

interface InlineFeedbackInput {
	type: FeedbackType;
	text: string;
	timing: FeedbackTiming;
	file: string;
	line: number;
}

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
	// New props for inline feedback
	inlineFeedback?: Feedback[];
	onAddInlineFeedback?: (input: InlineFeedbackInput) => Promise<void>;
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
	inlineFeedback = [],
	onAddInlineFeedback,
}: DiffHunkProps) {
	// Track which line has the feedback input open
	const [feedbackInputLine, setFeedbackInputLine] = useState<number | null>(null);
	// Track which line is being hovered
	const [hoveredLine, setHoveredLine] = useState<number | null>(null);

	// Get feedback for a specific line
	const getFeedbackForLine = useCallback(
		(lineNumber: number | undefined) => {
			if (!lineNumber) return [];
			return inlineFeedback.filter((f) => f.file === filePath && f.line === lineNumber);
		},
		[inlineFeedback, filePath]
	);

	// Handle opening feedback input
	const handleOpenFeedbackInput = useCallback((lineNumber: number) => {
		setFeedbackInputLine(lineNumber);
	}, []);

	// Handle closing feedback input
	const handleCloseFeedbackInput = useCallback(() => {
		setFeedbackInputLine(null);
	}, []);

	// Handle submitting feedback
	const handleSubmitFeedback = useCallback(
		async (text: string, lineNumber: number) => {
			if (!onAddInlineFeedback || !text.trim()) return;

			await onAddInlineFeedback({
				type: FeedbackType.INLINE,
				text: text.trim(),
				timing: FeedbackTiming.WHEN_DONE,
				file: filePath,
				line: lineNumber,
			});

			setFeedbackInputLine(null);
		},
		[onAddInlineFeedback, filePath]
	);

	// Check if hunk is binary (no line-level content)
	const isBinary = (hunk as unknown as { binary?: boolean }).binary;
	if (isBinary) {
		return (
			<div className="diff-hunk">
				<div className="hunk-header">Binary file</div>
			</div>
		);
	}

	return (
		<div className="diff-hunk">
			{/* Hunk Header */}
			<div className="hunk-header">
				@@ -{hunk.oldStart},{hunk.oldLines} +{hunk.newStart},{hunk.newLines} @@
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
						getFeedbackForLine={getFeedbackForLine}
						hoveredLine={hoveredLine}
						setHoveredLine={setHoveredLine}
						feedbackInputLine={feedbackInputLine}
						onOpenFeedbackInput={handleOpenFeedbackInput}
						onCloseFeedbackInput={handleCloseFeedbackInput}
						onSubmitFeedback={handleSubmitFeedback}
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
						getFeedbackForLine={getFeedbackForLine}
						hoveredLine={hoveredLine}
						setHoveredLine={setHoveredLine}
						feedbackInputLine={feedbackInputLine}
						onOpenFeedbackInput={handleOpenFeedbackInput}
						onCloseFeedbackInput={handleCloseFeedbackInput}
						onSubmitFeedback={handleSubmitFeedback}
					/>
				)}
			</div>
		</div>
	);
}

// Shared view props
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
	getFeedbackForLine: (lineNumber: number | undefined) => Feedback[];
	hoveredLine: number | null;
	setHoveredLine: (line: number | null) => void;
	feedbackInputLine: number | null;
	onOpenFeedbackInput: (lineNumber: number) => void;
	onCloseFeedbackInput: () => void;
	onSubmitFeedback: (text: string, lineNumber: number) => Promise<void>;
}

// Line Number Cell with hover "+" button and feedback indicator
interface LineNumberCellProps {
	lineNumber: number | undefined;
	side: 'old' | 'new';
	feedback: Feedback[];
	isHovered: boolean;
	onHover: () => void;
	onUnhover: () => void;
	onAddClick: () => void;
}

function LineNumberCell({
	lineNumber,
	side,
	feedback,
	isHovered,
	onHover,
	onUnhover,
	onAddClick,
}: LineNumberCellProps) {
	const hasFeedback = feedback.length > 0;

	return (
		<td
			className={`line-number ${side}`}
			onMouseEnter={onHover}
			onMouseLeave={onUnhover}
		>
			{lineNumber ?? ''}
			{isHovered && lineNumber && (
				<button
					type="button"
					className="add-feedback-button"
					onClick={(e) => {
						e.stopPropagation();
						onAddClick();
					}}
					aria-label="Add feedback"
				>
					+
				</button>
			)}
			{hasFeedback && <FeedbackIndicator feedback={feedback} />}
		</td>
	);
}

// Inline Feedback Input Row
interface InlineFeedbackRowProps {
	colSpan: number;
	onSubmit: (text: string) => Promise<void>;
	onCancel: () => void;
}

function InlineFeedbackRow({ colSpan, onSubmit, onCancel }: InlineFeedbackRowProps) {
	const [text, setText] = useState('');
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const handleSubmit = async () => {
		if (!text.trim()) return;
		setSubmitting(true);
		setError(null);
		try {
			await onSubmit(text);
		} catch (_err) {
			setError('Failed to add feedback');
			setSubmitting(false);
		}
	};

	const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			handleSubmit();
		} else if (e.key === 'Escape') {
			onCancel();
		}
	};

	return (
		<tr className="inline-feedback-row" data-testid="inline-feedback-row">
			<td colSpan={colSpan}>
				<div className="inline-feedback-input">
					<input
						type="text"
						placeholder="Add feedback for this line..."
						value={text}
						onChange={(e) => setText(e.target.value)}
						onKeyDown={handleKeyDown}
						disabled={submitting}
						autoFocus
					/>
					<div className="inline-feedback-actions">
						{error && <span className="inline-feedback-error">{error}</span>}
						<button
							type="button"
							className="cancel-button"
							onClick={onCancel}
							disabled={submitting}
						>
							Cancel
						</button>
						<button
							type="button"
							className="submit-button"
							onClick={handleSubmit}
							disabled={!text.trim() || submitting}
						>
							Add
						</button>
					</div>
				</div>
			</td>
		</tr>
	);
}

// Split View (side-by-side)
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
	getFeedbackForLine,
	hoveredLine,
	setHoveredLine,
	feedbackInputLine,
	onOpenFeedbackInput,
	onCloseFeedbackInput,
	onSubmitFeedback,
}: ViewProps) {
	// Build pairs for split view
	const pairs: Array<{
		left: DiffLine | null;
		right: DiffLine | null;
	}> = [];

	const leftQueue: DiffLine[] = [];
	const rightQueue: DiffLine[] = [];

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
		lineNumber ? comments.filter((c) => c.lineNumber === lineNumber) : [];

	return (
		<table className="split-table">
			<tbody>
				{pairs.map((pair, index) => {
					const leftLine = pair.left;
					const rightLine = pair.right;
					const leftLineNum = leftLine?.oldLine;
					const rightLineNum = rightLine?.newLine;
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

					// Determine which line number is used for feedback input
					const effectiveLineNum = rightLineNum ?? leftLineNum;
					const showFeedbackInput = feedbackInputLine === effectiveLineNum;

					return (
						<Fragment key={index}>
							<tr className="split-row">
								{/* Left side (old) */}
								<LineNumberCell
									lineNumber={leftLineNum}
									side="old"
									feedback={getFeedbackForLine(leftLineNum)}
									isHovered={hoveredLine === leftLineNum}
									onHover={() => leftLineNum && setHoveredLine(leftLineNum)}
									onUnhover={() => setHoveredLine(null)}
									onAddClick={() => leftLineNum && onOpenFeedbackInput(leftLineNum)}
								/>
								<td
									className={`line-content old ${leftLine?.type ?? 'empty'}`}
									onClick={() =>
										leftLine?.oldLine && onLineClick(leftLine.oldLine, filePath)
									}
								>
									{leftLine?.type === 'deletion' && <span className="prefix">-</span>}
									{leftLine?.content ?? ''}
								</td>

								{/* Right side (new) */}
								<LineNumberCell
									lineNumber={rightLineNum}
									side="new"
									feedback={getFeedbackForLine(rightLineNum)}
									isHovered={hoveredLine === rightLineNum}
									onHover={() => rightLineNum && setHoveredLine(rightLineNum)}
									onUnhover={() => setHoveredLine(null)}
									onAddClick={() => rightLineNum && onOpenFeedbackInput(rightLineNum)}
								/>
								<td
									className={`line-content new ${rightLine?.type ?? 'empty'}`}
									onClick={() =>
										rightLine?.newLine && onLineClick(rightLine.newLine, filePath)
									}
								>
									{rightLine?.type === 'addition' && <span className="prefix">+</span>}
									{rightLine?.content ?? ''}
								</td>
							</tr>

							{/* Inline feedback input row */}
							{showFeedbackInput && effectiveLineNum && (
								<InlineFeedbackRow
									colSpan={4}
									onSubmit={(text) => onSubmitFeedback(text, effectiveLineNum)}
									onCancel={onCloseFeedbackInput}
								/>
							)}

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
	getFeedbackForLine,
	hoveredLine,
	setHoveredLine,
	feedbackInputLine,
	onOpenFeedbackInput,
	onCloseFeedbackInput,
	onSubmitFeedback,
}: ViewProps) {
	const getLineComments = (lineNumber?: number) =>
		lineNumber ? comments.filter((c) => c.lineNumber === lineNumber) : [];

	return (
		<table className="unified-table">
			<tbody>
				{hunk.lines.map((line, index) => {
					const lineNum = line.newLine ?? line.oldLine;
					const lineComments = getLineComments(lineNum);
					const isActive = Boolean(lineNum && activeLineNumber === lineNum);
					const showFeedbackInput = feedbackInputLine === lineNum;

					// For unified view, use oldLine for deletions, newLine for additions/context
					const effectiveLineNum = line.type === 'deletion' ? line.oldLine : line.newLine;
					const feedback = getFeedbackForLine(effectiveLineNum);

					return (
						<Fragment key={index}>
							<tr className={`unified-row ${line.type}`}>
								<LineNumberCell
									lineNumber={line.oldLine}
									side="old"
									feedback={line.type === 'deletion' ? feedback : []}
									isHovered={hoveredLine === line.oldLine && line.type !== 'addition'}
									onHover={() => line.oldLine && setHoveredLine(line.oldLine)}
									onUnhover={() => setHoveredLine(null)}
									onAddClick={() => line.oldLine && onOpenFeedbackInput(line.oldLine)}
								/>
								<LineNumberCell
									lineNumber={line.newLine}
									side="new"
									feedback={line.type !== 'deletion' ? feedback : []}
									isHovered={hoveredLine === line.newLine && line.type !== 'deletion'}
									onHover={() => line.newLine && setHoveredLine(line.newLine)}
									onUnhover={() => setHoveredLine(null)}
									onAddClick={() => line.newLine && onOpenFeedbackInput(line.newLine)}
								/>
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

							{/* Inline feedback input row */}
							{showFeedbackInput && lineNum && (
								<InlineFeedbackRow
									colSpan={3}
									onSubmit={(text) => onSubmitFeedback(text, lineNum)}
									onCancel={onCloseFeedbackInput}
								/>
							)}

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
