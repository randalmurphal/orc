/**
 * DiffViewModal Component
 *
 * A full-screen diff view modal with lazygit-style navigation:
 * - Modal dialog for viewing full task diffs
 * - File list navigation with keyboard shortcuts (j/k, arrow keys)
 * - Split/unified view mode toggle
 * - Individual file diff display
 * - Search and filtering capabilities
 * - Comprehensive keyboard navigation (vim-style)
 * - Loading states and error handling
 * - Focus management and accessibility
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import { create } from '@bufbuild/protobuf';
import { Modal } from './Modal';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { DiffFile } from '@/components/task-detail/diff/DiffFile';
import { DiffStats } from '@/components/task-detail/diff/DiffStats';
import { taskClient } from '@/lib/client';
import type { DiffResult, FileDiff } from '@/gen/orc/v1/common_pb';
import { GetDiffRequestSchema, GetFileDiffRequestSchema } from '@/gen/orc/v1/task_pb';
import './DiffViewModal.css';

export interface DiffViewModalProps {
	/** Whether the modal is open */
	open: boolean;
	/** Task ID to show diff for */
	taskId: string;
	/** Project ID */
	projectId: string;
	/** Pre-selected file path to focus on */
	selectedFile?: string;
	/** Callback when modal is closed */
	onClose: () => void;
}

type ViewMode = 'split' | 'unified';
type LoadingState = 'loading' | 'success' | 'error';

interface FileListItem {
	path: string;
	status: string;
	additions: number;
	deletions: number;
	binary: boolean;
	syntax: string;
}

export function DiffViewModal({
	open,
	taskId,
	projectId,
	selectedFile,
	onClose,
}: DiffViewModalProps) {
	// State management
	const [diffResult, setDiffResult] = useState<DiffResult | null>(null);
	const [loadingState, setLoadingState] = useState<LoadingState>('loading');
	const [error, setError] = useState<string | null>(null);
	const [viewMode, setViewMode] = useState<ViewMode>('split');
	const [selectedFileIndex, setSelectedFileIndex] = useState(0);
	const [selectedFileDiff, setSelectedFileDiff] = useState<FileDiff | null>(null);
	const [fileDiffLoading, setFileDiffLoading] = useState(false);
	const [fileDiffError, setFileDiffError] = useState<string | null>(null);
	const [searchVisible, setSearchVisible] = useState(false);
	const [searchQuery, setSearchQuery] = useState('');
	const [filteredFiles, setFilteredFiles] = useState<FileListItem[]>([]);
	const [statusFilter, setStatusFilter] = useState<string | null>(null);
	const [showKeyboardHelp, setShowKeyboardHelp] = useState(false);

	// Refs for focus management
	const fileListRef = useRef<HTMLDivElement>(null);
	const searchInputRef = useRef<HTMLInputElement>(null);
	const modalRef = useRef<HTMLDivElement>(null);
	const previousFocusRef = useRef<HTMLElement | null>(null);

	// Cache for file diffs
	const fileDiffCache = useRef<Map<string, FileDiff>>(new Map());

	// Store initial focus element
	useEffect(() => {
		if (open) {
			previousFocusRef.current = document.activeElement as HTMLElement;
		}
	}, [open]);

	// Main diff loading
	useEffect(() => {
		let aborted = false;
		const controller = new AbortController();

		if (!open || !taskId || !projectId) {
			return;
		}

		const fetchDiff = async () => {
			try {
				setLoadingState('loading');
				setError(null);

				const response = await taskClient.getDiff(
					create(GetDiffRequestSchema, {
						projectId,
						taskId,
					})
				);

				if (aborted) return;

				if (!response.diff) {
					throw new Error('Invalid response from server');
				}

				// Validate response data
				const invalidFile = response.diff.files?.find((f: any) => !f.path || f.status === undefined);
				if (invalidFile) {
					throw new Error('Invalid diff data received');
				}

				setDiffResult(response.diff);
				setLoadingState('success');
			} catch (err) {
				if (aborted) return;

				const errorMessage = err instanceof Error ? err.message : 'Failed to load diff';
				setError(errorMessage);
				setLoadingState('error');
			}
		};

		fetchDiff();

		return () => {
			aborted = true;
			controller.abort();
		};
	}, [open, taskId, projectId]);

	// Process files for list display
	useEffect(() => {
		if (!diffResult?.files) {
			setFilteredFiles([]);
			return;
		}

		let files = diffResult.files.map((file: any) => ({
			path: file.path,
			status: file.status,
			additions: file.additions,
			deletions: file.deletions,
			binary: file.binary,
			syntax: file.syntax || '',
		}));

		// Apply search filter
		if (searchQuery.trim()) {
			files = files.filter((file: any) =>
				file.path.toLowerCase().includes(searchQuery.toLowerCase())
			);
		}

		// Apply status filter
		if (statusFilter) {
			files = files.filter((file: any) => file.status === statusFilter);
		}

		setFilteredFiles(files);

		// Reset selection if current selection is not in filtered results
		if (files.length > 0 && selectedFileIndex >= files.length) {
			setSelectedFileIndex(0);
		}
	}, [diffResult, searchQuery, statusFilter, selectedFileIndex]);

	// Handle selected file change from prop
	useEffect(() => {
		if (selectedFile && diffResult?.files) {
			const index = diffResult.files.findIndex((f: any) => f.path === selectedFile);
			if (index >= 0) {
				setSelectedFileIndex(index);
			}
		}
	}, [selectedFile, diffResult]);

	// Load individual file diff when selection changes
	useEffect(() => {
		const loadFileDiff = async () => {
			if (!diffResult?.files || selectedFileIndex >= diffResult.files.length) {
				return;
			}

			const file = diffResult.files[selectedFileIndex];

			// Don't load diff for binary files
			if (file.binary) {
				setSelectedFileDiff(null);
				return;
			}

			// Check cache first
			if (fileDiffCache.current.has(file.path)) {
				setSelectedFileDiff(fileDiffCache.current.get(file.path) || null);
				return;
			}

			try {
				setFileDiffLoading(true);
				setFileDiffError(null);

				const response = await taskClient.getFileDiff(
					create(GetFileDiffRequestSchema, {
						projectId,
						taskId,
						filePath: file.path,
					})
				);

				if (response.file) {
					// Validate consistency
					if (response.file.path !== file.path) {
						throw new Error('Data inconsistency detected. Please refresh.');
					}

					fileDiffCache.current.set(file.path, response.file);
					setSelectedFileDiff(response.file);
				}
			} catch (err) {
				const errorMessage = err instanceof Error ? err.message : 'Failed to load file diff';
				setFileDiffError(errorMessage);
			} finally {
				setFileDiffLoading(false);
			}
		};

		loadFileDiff();
	}, [selectedFileIndex, diffResult, projectId, taskId]);

	// Set initial focus to file list when modal opens
	useEffect(() => {
		if (open && loadingState === 'success' && fileListRef.current) {
			setTimeout(() => {
				fileListRef.current?.focus();
			}, 100);
		}
	}, [open, loadingState]);

	// Restore focus when modal closes
	useEffect(() => {
		if (!open && previousFocusRef.current) {
			previousFocusRef.current.focus();
		}
	}, [open]);

	// File navigation functions
	const navigateToFile = useCallback((index: number) => {
		const fileCount = filteredFiles.length;
		if (fileCount === 0) return;

		let newIndex = index;
		if (newIndex < 0) newIndex = fileCount - 1;
		if (newIndex >= fileCount) newIndex = 0;

		setSelectedFileIndex(newIndex);

		// Update focus for accessibility
		const fileElement = document.querySelector(`[data-testid="file-item-${filteredFiles[newIndex].path}"]`);
		if (fileElement) {
			(fileElement as HTMLElement).focus();
		}
	}, [filteredFiles]);

	const selectFile = useCallback((filePath: string) => {
		if (!diffResult?.files) return;

		const index = diffResult.files.findIndex((f: any) => f.path === filePath);
		if (index >= 0) {
			setSelectedFileIndex(index);
		}
	}, [diffResult]);

	// Keyboard navigation
	useEffect(() => {
		if (!open) return;

		const handleKeyDown = (e: KeyboardEvent) => {
			// Don't handle keys when search is visible and focused
			if (searchVisible && document.activeElement === searchInputRef.current) {
				if (e.key === 'Escape') {
					setSearchVisible(false);
					setSearchQuery('');
					fileListRef.current?.focus();
					e.preventDefault();
					e.stopPropagation();
				}
				return;
			}

			// Prevent event bubbling for handled keys
			const handledKeys = ['j', 'k', 'ArrowUp', 'ArrowDown', 'Home', 'End', 'g', 'G', 'Enter', ' ', 'o', 'v', 'Tab', 't', 'w', 's', '/', 'f', 'Escape', 'q'];
			if (handledKeys.includes(e.key) || (e.key >= '1' && e.key <= '9')) {
				e.preventDefault();
				e.stopPropagation();
			}

			switch (e.key) {
				case 'j':
				case 'ArrowDown':
					navigateToFile(selectedFileIndex + 1);
					break;
				case 'k':
				case 'ArrowUp':
					navigateToFile(selectedFileIndex - 1);
					break;
				case 'g':
					if (e.shiftKey) { // G - last file
						navigateToFile(filteredFiles.length - 1);
					} else { // gg - first file (need to handle double-g)
						// Simple implementation: single g goes to first
						navigateToFile(0);
					}
					break;
				case 'Home':
					navigateToFile(0);
					break;
				case 'End':
					navigateToFile(filteredFiles.length - 1);
					break;
				case 'Enter':
				case 'o':
					// Load file diff if not already loaded
					if (filteredFiles[selectedFileIndex] && !filteredFiles[selectedFileIndex].binary) {
						// File diff loading is handled by effect
					}
					break;
				case ' ':
					// Select file without loading diff (just highlight)
					break;
				case 'Tab':
					setViewMode(viewMode === 'split' ? 'unified' : 'split');
					break;
				case 't':
					setViewMode(viewMode === 'split' ? 'unified' : 'split');
					break;
				case '/':
					setSearchVisible(true);
					setTimeout(() => searchInputRef.current?.focus(), 0);
					break;
				case 'f':
					// Toggle status filter (cycle through: all -> added -> modified -> deleted -> all)
					const statuses = [null, 'added', 'modified', 'deleted'];
					const currentIndex = statuses.indexOf(statusFilter);
					const nextIndex = (currentIndex + 1) % statuses.length;
					setStatusFilter(statuses[nextIndex]);
					break;
				case 'Escape':
				case 'q':
					onClose();
					break;
				case '?':
					if (e.shiftKey) {
						setShowKeyboardHelp(!showKeyboardHelp);
					}
					break;
				default:
					// Number keys for quick file selection
					if (e.key >= '1' && e.key <= '9') {
						const fileIndex = parseInt(e.key) - 1;
						if (fileIndex < filteredFiles.length) {
							navigateToFile(fileIndex);
						} else if (filteredFiles.length > 0) {
							// If number is beyond file count, go to last file
							navigateToFile(filteredFiles.length - 1);
						}
					}
					break;
			}
		};

		document.addEventListener('keydown', handleKeyDown, true);
		return () => document.removeEventListener('keydown', handleKeyDown, true);
	}, [open, selectedFileIndex, filteredFiles, viewMode, navigateToFile, onClose, searchVisible, statusFilter, showKeyboardHelp]);

	// Retry function
	const retryLoadDiff = useCallback(async () => {
		if (!taskId || !projectId) return;

		setLoadingState('loading');
		setError(null);

		try {
			const response = await taskClient.getDiff(
				create(GetDiffRequestSchema, {
					projectId,
					taskId,
				})
			);

			if (!response.diff) {
				throw new Error('Invalid response from server');
			}

			setDiffResult(response.diff);
			setLoadingState('success');
		} catch (err) {
			const errorMessage = err instanceof Error ? err.message : 'An unexpected error occurred';
			setError(errorMessage);
			setLoadingState('error');
		}
	}, [taskId, projectId]);

	// Don't render if modal is closed or required props are missing
	if (!open || !taskId || !projectId) {
		return null;
	}

	const currentFile = selectedFileDiff || (diffResult?.files?.[selectedFileIndex]);

	return (
		<Modal
			open={open}
			onClose={onClose}
			size="xl"
			title={`${taskId} - Changes`}
			ariaLabel="Full diff view modal"
			showClose={true}
		>
			<div
				className="diff-view-modal"
				ref={modalRef}
				data-testid="diff-view-modal"
				role="main"
				aria-label={`Diff view for task ${taskId}`}
			>
				{/* Modal Header with Stats and Controls */}
				<div className="diff-modal-header">
					{loadingState === 'success' && diffResult?.stats && (
						<DiffStats stats={diffResult.stats} />
					)}

					<div className="diff-modal-controls">
						{/* View Mode Toggle */}
						<div className="view-mode-toggle" data-testid="view-mode-toggle" role="group" aria-label="View mode">
							<Button
								variant={viewMode === 'split' ? 'primary' : 'secondary'}
								size="sm"
								className={viewMode === 'split' ? 'active' : ''}
								onClick={() => setViewMode('split')}
								aria-label="Split view mode"
							>
								Split
							</Button>
							<Button
								variant={viewMode === 'unified' ? 'primary' : 'secondary'}
								size="sm"
								className={viewMode === 'unified' ? 'active' : ''}
								onClick={() => setViewMode('unified')}
								aria-label="Unified view mode"
							>
								Unified
							</Button>
						</div>

						{/* Help Button */}
						<Button
							variant="ghost"
							size="sm"
							onClick={() => setShowKeyboardHelp(!showKeyboardHelp)}
							aria-label="Show keyboard shortcuts"
							title="Keyboard shortcuts (?)"
						>
							<Icon name="help" size={16} />
						</Button>
					</div>
				</div>

				{/* Loading State */}
				{loadingState === 'loading' && (
					<div className="diff-modal-loading" data-testid="diff-modal-loading">
						<div className="loading-spinner" />
						<span>Loading diff...</span>
					</div>
				)}

				{/* Error State */}
				{loadingState === 'error' && (
					<div className="diff-modal-error" data-testid="diff-modal-error">
						<Icon name="alert-circle" size={20} />
						<span>{getErrorMessage(error)}</span>
						<Button
							variant="secondary"
							size="sm"
							onClick={retryLoadDiff}
							data-testid="retry-load-diff"
						>
							<Icon name="refresh" size={14} />
							Retry
						</Button>
					</div>
				)}

				{/* Empty State */}
				{loadingState === 'success' && (!diffResult?.files || diffResult.files.length === 0) && (
					<div className="diff-modal-empty" data-testid="diff-modal-empty">
						<Icon name="file" size={32} />
						<span>No files changed</span>
					</div>
				)}

				{/* Main Content */}
				{loadingState === 'success' && diffResult?.files && diffResult.files.length > 0 && (
					<div className="diff-modal-content">
						{/* File List Sidebar */}
						<div className="diff-modal-sidebar">
							{/* Search Bar */}
							{searchVisible && (
								<div className="file-search">
									<input
										ref={searchInputRef}
										type="text"
										placeholder="Search files..."
										value={searchQuery}
										onChange={(e) => setSearchQuery(e.target.value)}
										data-testid="file-search-input"
										aria-label="Search files"
									/>
								</div>
							)}

							{/* Status Filter */}
							{statusFilter && (
								<div className="status-filter-active">
									<span>Filter: {statusFilter}</span>
									<Button
										variant="ghost"
										size="sm"
										onClick={() => setStatusFilter(null)}
										aria-label="Clear filter"
									>
										<Icon name="x" size={12} />
									</Button>
								</div>
							)}

							{/* File List */}
							<div
								className="diff-modal-file-list"
								data-testid="diff-modal-file-list"
								ref={fileListRef}
								tabIndex={0}
								role="listbox"
								aria-label="File list"
								aria-activedescendant={filteredFiles[selectedFileIndex] ? `file-item-${filteredFiles[selectedFileIndex].path}` : undefined}
							>
								{filteredFiles.map((file, index) => (
									<div
										key={file.path}
										id={`file-item-${file.path}`}
										data-testid={`file-item-${file.path}`}
										className={`file-item ${index === selectedFileIndex ? 'selected' : ''}`}
										role="option"
										aria-selected={index === selectedFileIndex}
										tabIndex={-1}
										onClick={() => selectFile(file.path)}
										onKeyDown={(e) => {
											if (e.key === 'Enter') {
												selectFile(file.path);
											}
										}}
									>
										<div className="file-item-content">
											{/* Status Badge */}
											<span
												className={`status-badge ${file.status}`}
												data-testid={`file-status-${file.status}`}
												aria-label={`File status: ${file.status}`}
											>
												{getStatusBadge(file.status)}
											</span>

											{/* File Path */}
											<span className="file-path" title={file.path}>
												{file.path}
											</span>

											{/* Binary Badge */}
											{file.binary && (
												<span className="binary-badge" aria-label="Binary file">
													Binary
												</span>
											)}

											{/* Stats */}
											<div className="file-stats">
												{file.additions > 0 && (
													<span className="additions">+{file.additions}</span>
												)}
												{file.deletions > 0 && (
													<span className="deletions">-{file.deletions}</span>
												)}
											</div>
										</div>
									</div>
								))}
							</div>
						</div>

						{/* Diff Content */}
						<div className="diff-modal-main">
							{/* Content Header */}
							{currentFile && (
								<div className="diff-content-header" data-testid="diff-content-header">
									<span className="current-file-path">{currentFile.path}</span>
									{currentFile.binary && (
										<span className="binary-notice">Binary file</span>
									)}
								</div>
							)}

							{/* Diff Display */}
							<div className="diff-content-wrapper">
								{fileDiffLoading && (
									<div className="file-diff-loading" data-testid="file-diff-loading">
										<div className="loading-spinner" />
										<span>Loading file diff...</span>
									</div>
								)}

								{fileDiffError && (
									<div className="file-diff-error" data-testid="file-diff-error">
										<Icon name="alert-circle" size={16} />
										<span>{fileDiffError}</span>
									</div>
								)}

								{currentFile?.binary && !fileDiffLoading && !fileDiffError && (
									<div className="binary-file-message" data-testid="binary-file-message">
										<Icon name="file" size={32} />
										<span>Binary file - content not shown</span>
									</div>
								)}

								{selectedFileDiff && !fileDiffLoading && !fileDiffError && (
									<div
										className={`diff-content ${viewMode}`}
										data-testid="diff-content"
									>
										<DiffFile
											file={selectedFileDiff}
											expanded={true}
											viewMode={viewMode}
											comments={[]}
											activeLineNumber={null}
											onToggle={() => {}}
											onLineClick={() => {}}
											onAddComment={async () => {}}
											onResolveComment={() => {}}
											onWontFixComment={() => {}}
											onDeleteComment={() => {}}
											onCloseThread={() => {}}
										/>
									</div>
								)}
							</div>
						</div>
					</div>
				)}

				{/* Screen Reader Announcements */}
				<div
					aria-live="polite"
					aria-atomic="true"
					className="sr-only"
					data-testid="sr-announcer"
				>
					{/* Announcements will be updated by navigation functions */}
				</div>

				{/* Keyboard Shortcuts Help */}
				{showKeyboardHelp && (
					<div className="keyboard-shortcuts-help" data-testid="keyboard-shortcuts-help">
						<div className="help-content">
							<h3>Keyboard Shortcuts</h3>
							<div className="shortcuts-grid">
								<div className="shortcut-group">
									<h4>Navigation</h4>
									<div className="shortcut-item">
										<kbd>j</kbd><kbd>k</kbd> / <kbd>↓</kbd><kbd>↑</kbd>
										<span>Navigate files</span>
									</div>
									<div className="shortcut-item">
										<kbd>gg</kbd> / <kbd>Home</kbd>
										<span>First file</span>
									</div>
									<div className="shortcut-item">
										<kbd>G</kbd> / <kbd>End</kbd>
										<span>Last file</span>
									</div>
									<div className="shortcut-item">
										<kbd>1</kbd>-<kbd>9</kbd>
										<span>Jump to file</span>
									</div>
								</div>
								<div className="shortcut-group">
									<h4>Actions</h4>
									<div className="shortcut-item">
										<kbd>Enter</kbd> / <kbd>o</kbd>
										<span>Select file</span>
									</div>
									<div className="shortcut-item">
										<kbd>Tab</kbd> / <kbd>t</kbd>
										<span>Toggle view mode</span>
									</div>
									<div className="shortcut-item">
										<kbd>/</kbd>
										<span>Search files</span>
									</div>
									<div className="shortcut-item">
										<kbd>f</kbd>
										<span>Filter by status</span>
									</div>
								</div>
								<div className="shortcut-group">
									<h4>General</h4>
									<div className="shortcut-item">
										<kbd>Escape</kbd> / <kbd>q</kbd>
										<span>Close</span>
									</div>
									<div className="shortcut-item">
										<kbd>?</kbd>
										<span>Show help</span>
									</div>
								</div>
							</div>
							<Button
								variant="primary"
								size="sm"
								onClick={() => setShowKeyboardHelp(false)}
								className="help-close"
							>
								Close
							</Button>
						</div>
					</div>
				)}
			</div>
		</Modal>
	);
}

// Helper functions
function getStatusBadge(status: string): string {
	switch (status) {
		case 'added': return 'A';
		case 'modified': return 'M';
		case 'deleted': return 'D';
		case 'renamed': return 'R';
		default: return 'M';
	}
}

function getErrorMessage(error: string | null): string {
	if (!error) return 'An error occurred';

	if (error.includes('timeout') || error.includes('Timeout')) {
		return 'Request timed out. Please try again.';
	}
	if (error.includes('permission') || error.includes('Permission')) {
		return 'You do not have permission to view this diff.';
	}
	return error;
}