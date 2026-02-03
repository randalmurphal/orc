/**
 * FilesPanel component for right panel showing files changed by running tasks
 *
 * Displays changed files with:
 * - Blue-themed section header with file icon and count
 * - File icon based on extension (different icon for binary files)
 * - File path in monospace, truncated from left to show filename clearly
 * - Status badge: M (modified, amber), A (added, green), D (deleted, red), R (renamed, cyan)
 * - Files grouped by task if multiple tasks are running
 *
 * Reference: example_ui/board.html (.file-item class)
 */

import { useState, useCallback, useMemo } from 'react';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { DiffViewModal } from '@/components/overlays/DiffViewModal';
import { useCurrentProjectId } from '@/stores';
import './FilesPanel.css';

/** File status type matching FileDiff.status from types.ts */
export type FileStatus = 'modified' | 'added' | 'deleted' | 'renamed';

/** Changed file data */
export interface ChangedFile {
	/** Full file path */
	path: string;
	/** Change status */
	status: FileStatus;
	/** Whether the file is binary */
	binary?: boolean;
	/** Task ID that changed this file (for grouping) */
	taskId?: string;
}

export interface FilesPanelProps {
	/** Changed files to display */
	files: ChangedFile[];
	/** Callback when a file is clicked */
	onFileClick: (file: ChangedFile) => void;
	/** Maximum number of files to show before "more" link (default: 5) */
	maxVisible?: number;
	/** Callback when "more" link is clicked */
	onShowMore?: () => void;
}

/** Status badge labels */
const STATUS_LABELS: Record<FileStatus, string> = {
	modified: 'M',
	added: 'A',
	deleted: 'D',
	renamed: 'R',
};

/** Status badge aria labels */
const STATUS_ARIA_LABELS: Record<FileStatus, string> = {
	modified: 'Modified',
	added: 'Added',
	deleted: 'Deleted',
	renamed: 'Renamed',
};

/** Binary file extensions */
const BINARY_EXTENSIONS = new Set([
	'png',
	'jpg',
	'jpeg',
	'gif',
	'webp',
	'ico',
	'svg',
	'bmp',
	'tiff',
	'pdf',
	'zip',
	'tar',
	'gz',
	'rar',
	'7z',
	'exe',
	'dll',
	'so',
	'dylib',
	'wasm',
	'woff',
	'woff2',
	'ttf',
	'otf',
	'eot',
	'mp3',
	'mp4',
	'wav',
	'ogg',
	'webm',
	'avi',
	'mov',
	'sqlite',
	'db',
]);

/**
 * Check if a file path is likely a binary file based on extension
 */
function isBinaryFile(path: string): boolean {
	const ext = path.split('.').pop()?.toLowerCase() || '';
	return BINARY_EXTENSIONS.has(ext);
}

/**
 * Get display name for a file (just the filename, not the path)
 */
function getFileName(path: string): string {
	return path.split('/').pop() || path;
}

/**
 * FilesPanel displays files changed by running tasks with status badges and grouping.
 */
export function FilesPanel({
	files,
	onFileClick,
	maxVisible = 5,
	onShowMore,
}: FilesPanelProps) {
	const [collapsed, setCollapsed] = useState(false);
	const [showAll, setShowAll] = useState(false);
	const [modalOpen, setModalOpen] = useState(false);
	const [selectedFilePath, setSelectedFilePath] = useState<string | undefined>();

	const projectId = useCurrentProjectId();

	const handleToggle = useCallback(() => {
		setCollapsed((prev) => !prev);
	}, []);

	const handleFileClick = useCallback(
		(file: ChangedFile) => {
			onFileClick(file);
		},
		[onFileClick]
	);

	const handleKeyDown = useCallback(
		(file: ChangedFile, event: React.KeyboardEvent) => {
			if (event.shiftKey && event.key === 'Enter') {
				// Shift+Enter to open modal for this file
				event.preventDefault();
				setSelectedFilePath(file.path);
				setModalOpen(true);
			} else if (event.key === 'Enter' || event.key === ' ') {
				event.preventDefault();
				onFileClick(file);
			}
		},
		[onFileClick]
	);

	const handleFileDoubleClick = useCallback(
		(file: ChangedFile) => {
			setSelectedFilePath(file.path);
			setModalOpen(true);
		},
		[]
	);

	const handleFileCtrlClick = useCallback(
		(file: ChangedFile, event: React.MouseEvent) => {
			if (event.ctrlKey) {
				event.preventDefault();
				setSelectedFilePath(file.path);
				setModalOpen(true);
			}
		},
		[]
	);

	const handleViewFullDiff = useCallback(() => {
		if (files.length > 0) {
			setSelectedFilePath(undefined); // No specific file selected
			setModalOpen(true);
		}
	}, [files]);

	const handleShowMore = useCallback(() => {
		if (onShowMore) {
			onShowMore();
		} else {
			setShowAll(true);
		}
	}, [onShowMore]);

	// Group files by task ID
	const groupedFiles = useMemo(() => {
		const groups = new Map<string, ChangedFile[]>();

		for (const file of files) {
			const taskId = file.taskId || 'unknown';
			const group = groups.get(taskId) || [];
			group.push(file);
			groups.set(taskId, group);
		}

		return groups;
	}, [files]);

	// Check if we have multiple task groups
	const hasMultipleGroups = groupedFiles.size > 1;

	// Determine task ID for modal (single task vs multiple tasks)
	const modalTaskId = useMemo(() => {
		if (files.length === 0) return undefined;

		const uniqueTaskIds = Array.from(new Set(files.map(f => f.taskId).filter(Boolean)));
		return uniqueTaskIds.length === 1 ? uniqueTaskIds[0] : undefined;
	}, [files]);

	// Determine visible files
	const visibleFiles = useMemo(() => {
		if (showAll || files.length <= maxVisible) {
			return files;
		}
		return files.slice(0, maxVisible);
	}, [files, maxVisible, showAll]);

	const hiddenCount = files.length - visibleFiles.length;

	return (
		<div className={`files-panel panel-section ${collapsed ? 'collapsed' : ''}`}>
			<button
				className="panel-header"
				onClick={handleToggle}
				aria-expanded={!collapsed}
				aria-controls="files-panel-body"
			>
				<div className="panel-title">
					<div className="panel-title-icon blue">
						<Icon name="file-text" size={12} />
					</div>
					<span>Files Changed</span>
				</div>
				<span className="panel-badge" aria-label={`${files.length} files changed`}>
					{files.length}
				</span>
				<Icon
					name={collapsed ? 'chevron-right' : 'chevron-down'}
					size={12}
					className="panel-chevron"
				/>
			</button>

			<div id="files-panel-body" className="panel-body" role="region">
				{files.length === 0 ? (
					<div className="files-empty">No changed files</div>
				) : hasMultipleGroups ? (
					// Grouped by task
					Array.from(groupedFiles.entries()).map(([taskId, taskFiles]) => (
						<div key={taskId} className="files-task-group">
							<div className="files-task-header">
								<span className="files-task-header-id">{taskId}</span>
							</div>
							{taskFiles.map((file: ChangedFile) => (
								<FileItem
									key={file.path}
									file={file}
									onClick={handleFileClick}
									onKeyDown={handleKeyDown}
									onDoubleClick={handleFileDoubleClick}
									onCtrlClick={handleFileCtrlClick}
								/>
							))}
						</div>
					))
				) : (
					// Flat list
					<>
						{visibleFiles.map((file: ChangedFile) => (
							<FileItem
								key={file.path}
								file={file}
								onClick={handleFileClick}
								onKeyDown={handleKeyDown}
								onDoubleClick={handleFileDoubleClick}
								onCtrlClick={handleFileCtrlClick}
							/>
						))}

						{hiddenCount > 0 && (
							<button
								className="files-more"
								onClick={handleShowMore}
								aria-label={`Show ${hiddenCount} more files`}
							>
								+ {hiddenCount} more files
							</button>
						)}
					</>
				)}

				{/* View Full Diff Button */}
				{files.length > 0 && (
					<div className="files-actions">
						<Button
							variant="secondary"
							size="sm"
							onClick={handleViewFullDiff}
							aria-label="Open full diff view modal"
							title="Open full diff view modal"
						>
							View Full Diff
						</Button>
					</div>
				)}
			</div>

			{/* Diff View Modal */}
			{modalOpen && projectId && (
				<DiffViewModal
					open={modalOpen}
					taskId={modalTaskId || 'unknown'}
					projectId={projectId}
					selectedFile={selectedFilePath}
					onClose={() => {
						setModalOpen(false);
						setSelectedFilePath(undefined);
					}}
				/>
			)}
		</div>
	);
}

/** Individual file item component */
interface FileItemProps {
	file: ChangedFile;
	onClick: (file: ChangedFile) => void;
	onKeyDown: (file: ChangedFile, event: React.KeyboardEvent) => void;
	onDoubleClick?: (file: ChangedFile) => void;
	onCtrlClick?: (file: ChangedFile, event: React.MouseEvent) => void;
}

function FileItem({ file, onClick, onKeyDown, onDoubleClick, onCtrlClick }: FileItemProps) {
	const isBinary = file.binary ?? isBinaryFile(file.path);
	const fileName = getFileName(file.path);

	const handleClick = (e: React.MouseEvent) => {
		if (onCtrlClick && e.ctrlKey) {
			onCtrlClick(file, e);
		} else {
			onClick(file);
		}
	};

	const handleDoubleClick = () => {
		if (onDoubleClick) {
			onDoubleClick(file);
		}
	};

	return (
		<div
			className="file-item"
			onClick={handleClick}
			onDoubleClick={handleDoubleClick}
			onKeyDown={(e) => onKeyDown(file, e)}
			tabIndex={0}
			role="button"
			aria-label={`${fileName}, ${STATUS_ARIA_LABELS[file.status]}${isBinary ? ', binary file' : ''}`}
		>
			<div className={`file-icon ${isBinary ? 'binary' : ''}`}>
				<Icon name={isBinary ? 'image' : 'file-text'} size={12} />
			</div>
			<span className="file-path" title={file.path}>
				{file.path}
			</span>
			<span
				className={`file-status ${file.status}`}
				aria-label={STATUS_ARIA_LABELS[file.status]}
			>
				{STATUS_LABELS[file.status]}
			</span>
		</div>
	);
}
