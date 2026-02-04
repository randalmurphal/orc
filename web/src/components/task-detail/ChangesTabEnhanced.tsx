import { useState, useEffect, useCallback, useMemo } from 'react';
import { create } from '@bufbuild/protobuf';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { FilesPanel } from './FilesPanel';
import { ChangesTab } from './ChangesTab';
import { DiffFile } from './diff/DiffFile';
import { DiffViewModal } from '@/components/overlays/DiffViewModal';
import { useCurrentProjectId } from '@/stores';
import { feedbackClient, taskClient } from '@/lib/client';
import { FeedbackType, FeedbackTiming, type Feedback } from '@/gen/orc/v1/feedback_pb';
import { GetDiffRequestSchema } from '@/gen/orc/v1/task_pb';
import type { FileDiff } from '@/gen/orc/v1/common_pb';

export interface ChangesTabEnhancedProps {
	taskId: string;
}

interface InlineFeedbackInput {
	type: FeedbackType;
	text: string;
	timing: FeedbackTiming;
	file: string;
	line: number;
}

export function ChangesTabEnhanced({ taskId }: ChangesTabEnhancedProps) {
	const projectId = useCurrentProjectId();
	const [useEnhancedView, setUseEnhancedView] = useState(true);
	const [hasError, setHasError] = useState(false);
	const [viewMode, setViewMode] = useState<'list' | 'tree'>('list');
	const [diffMode] = useState<'split' | 'unified'>('split');
	const [selectedFile] = useState<string | null>(null);
	const [expandedFiles] = useState<Set<string>>(new Set());
	const [isMobile, setIsMobile] = useState(false);
	const [isTablet, setIsTablet] = useState(false);
	const [panelCollapsed, setPanelCollapsed] = useState(false);
	const [isTransitioning, setIsTransitioning] = useState(false);
	const [modalOpen, setModalOpen] = useState(false);

	// Diff state
	const [diffFiles, setDiffFiles] = useState<FileDiff[]>([]);
	const [diffLoading, setDiffLoading] = useState(false);

	// Inline feedback state
	const [inlineFeedback, setInlineFeedback] = useState<Feedback[]>([]);
	const [feedbackError, setFeedbackError] = useState<string | null>(null);

	// Window size detection for responsive design
	useEffect(() => {
		const handleResize = () => {
			const width = window.innerWidth;
			setIsMobile(width <= 600);
			setIsTablet(width <= 800 && width > 600);
		};

		handleResize();
		window.addEventListener('resize', handleResize);
		return () => window.removeEventListener('resize', handleResize);
	}, []);

	// Load diff content
	const loadDiff = useCallback(async () => {
		if (!projectId) return;

		setDiffLoading(true);
		try {
			const response = await taskClient.getDiff(
				create(GetDiffRequestSchema, { projectId, taskId })
			);
			if (response.diff?.files) {
				setDiffFiles(response.diff.files);
			}
		} catch (error) {
			console.error('Failed to load diff:', error);
		} finally {
			setDiffLoading(false);
		}
	}, [projectId, taskId]);

	// Load inline feedback for this task
	const loadInlineFeedback = useCallback(async () => {
		if (!projectId) return;

		try {
			const response = await feedbackClient.listFeedback({
				projectId,
				taskId,
				excludeReceived: false,
			});
			// Filter for inline feedback only (client-side since API doesn't support type filter)
			const inlineOnly = response.feedback.filter((f) => f.type === FeedbackType.INLINE);
			setInlineFeedback(inlineOnly);
		} catch (error) {
			console.error('Failed to load inline feedback:', error);
		}
	}, [projectId, taskId]);

	// Handle adding inline feedback
	const handleAddInlineFeedback = useCallback(
		async (input: InlineFeedbackInput) => {
			if (!projectId) return;

			setFeedbackError(null);
			try {
				await feedbackClient.addFeedback({
					projectId,
					taskId,
					type: input.type,
					text: input.text,
					timing: input.timing,
					file: input.file,
					line: input.line,
				});
				// Refresh feedback list after successful addition
				await loadInlineFeedback();
			} catch (error) {
				console.error('Failed to add inline feedback:', error);
				setFeedbackError('Failed to add feedback');
				throw error; // Re-throw to let the caller handle it
			}
		},
		[projectId, taskId, loadInlineFeedback]
	);

	// Load diff and feedback on mount
	useEffect(() => {
		loadDiff();
		loadInlineFeedback();
	}, [loadDiff, loadInlineFeedback]);

	// Handle view mode changes and sync between panels
	const handleViewModeChange = useCallback((newViewMode: 'list' | 'tree') => {
		setIsTransitioning(true);
		setViewMode(newViewMode);
		setTimeout(() => setIsTransitioning(false), 300);
	}, []);

	// Error boundary functionality
	const handleEnhancedViewError = useCallback((error: Error) => {
		console.error('Enhanced view error:', error);
		setHasError(true);
		setUseEnhancedView(false);
	}, []);

	// Reset to enhanced view
	const handleRetryEnhancedView = useCallback(() => {
		setHasError(false);
		setUseEnhancedView(true);
	}, []);

	// Trigger panel error for testing
	const triggerPanelError = useCallback(() => {
		handleEnhancedViewError(new Error('Test error'));
	}, [handleEnhancedViewError]);

	// Modal handler
	const handleExpandDiff = useCallback(() => {
		setModalOpen(true);
	}, []);

	const handleCloseModal = useCallback(() => {
		setModalOpen(false);
	}, []);

	// Get feedback for a specific file
	const getFeedbackForFile = useCallback(
		(filePath: string) => {
			return inlineFeedback.filter((f) => f.file === filePath);
		},
		[inlineFeedback]
	);

	// Keyboard shortcuts
	useEffect(() => {
		const handleKeyDown = (event: KeyboardEvent) => {
			if (event.ctrlKey || event.metaKey) {
				if (event.shiftKey && event.key === 'F') {
					event.preventDefault();
					handleExpandDiff();
					return;
				}

				switch (event.key) {
					case 'f': {
						event.preventDefault();
						const filesPanel = document.querySelector('[data-testid="files-panel"]');
						if (filesPanel && filesPanel instanceof HTMLElement) {
							filesPanel.focus();
						}
						break;
					}
					case 't':
						event.preventDefault();
						handleViewModeChange(viewMode === 'list' ? 'tree' : 'list');
						break;
				}
			}
		};

		document.addEventListener('keydown', handleKeyDown);
		return () => document.removeEventListener('keydown', handleKeyDown);
	}, [viewMode, handleViewModeChange, handleExpandDiff]);

	// Responsive layout classes
	const layoutClasses = useMemo(() => {
		const classes = ['changes-tab-enhanced', 'transition-all'];
		if (isMobile) classes.push('mobile-layout');
		if (isTablet) classes.push('tablet-layout');
		if (panelCollapsed) classes.push('panel-collapsed');
		if (isTransitioning) classes.push('transitioning');
		return classes.join(' ');
	}, [isMobile, isTablet, panelCollapsed, isTransitioning]);

	// Don't render if no projectId
	if (!projectId) {
		return (
			<div className="changes-tab-enhanced">
				<div className="loading-state">
					<Icon name="loader" size={24} />
					<span>Loading project...</span>
				</div>
			</div>
		);
	}

	// Error state with fallback to classic view
	if (hasError) {
		return (
			<div className="changes-tab-enhanced">
				<div data-testid="file-panel-error-boundary" className="error-boundary">
					<Icon name="alert-triangle" size={24} />
					<div className="error-content">
						<h3>Unable to load file list</h3>
						<p>There was an error loading the enhanced file view.</p>
						<div className="error-actions">
							<Button variant="primary" size="sm" onClick={handleRetryEnhancedView}>
								Switch back to enhanced view
							</Button>
						</div>
					</div>
				</div>

				<div data-testid="fallback-classic-view" className="fallback-classic">
					<ChangesTab taskId={taskId} />
				</div>
			</div>
		);
	}

	// Classic view fallback
	if (!useEnhancedView) {
		return (
			<div className="changes-tab-enhanced">
				<div className="classic-view-header">
					<Button
						data-testid="enhanced-view-toggle"
						variant="secondary"
						size="sm"
						onClick={() => setUseEnhancedView(true)}
					>
						Switch to Enhanced View
					</Button>
				</div>
				<div data-testid="classic-diff-view" className="classic-view">
					<ChangesTab taskId={taskId} />
				</div>
			</div>
		);
	}

	// Enhanced view with inline diff support
	return (
		<div
			data-testid="changes-tab-enhanced"
			className={layoutClasses}
			aria-label="Enhanced file changes view"
		>
			{/* Header with view controls */}
			<div className="enhanced-header">
				<div className="header-left">
					<h3>File Changes</h3>
					{isMobile && (
						<Button
							data-testid="files-panel-collapsed"
							variant="ghost"
							size="sm"
							onClick={() => setPanelCollapsed(!panelCollapsed)}
						>
							<Icon name={panelCollapsed ? 'panel-left-open' : 'panel-left-close'} size={16} />
						</Button>
					)}
				</div>

				<div className="header-right">
					<div
						data-testid="tree-view-active"
						style={{ display: viewMode === 'tree' ? 'block' : 'none' }}
					>
						<Icon name="folder" size={14} />
					</div>

					{isTablet && (
						<Button
							data-testid="collapse-file-panel"
							variant="ghost"
							size="sm"
							onClick={() => setPanelCollapsed(!panelCollapsed)}
						>
							<Icon name={panelCollapsed ? 'chevron-right' : 'chevron-left'} size={14} />
						</Button>
					)}

					<Button
						data-testid="classic-diff-toggle"
						variant="ghost"
						size="sm"
						onClick={() => setUseEnhancedView(false)}
					>
						Classic View
					</Button>

					{process.env.NODE_ENV === 'test' && (
						<Button
							data-testid="trigger-panel-error"
							variant="ghost"
							size="sm"
							onClick={triggerPanelError}
						>
							Trigger Error
						</Button>
					)}
				</div>
			</div>

			{/* Main content */}
			<div className="enhanced-content">
				{/* Files Panel */}
				<div
					className={`files-section ${panelCollapsed && (isMobile || isTablet) ? 'collapsed' : ''}`}
					role="complementary"
				>
					<FilesPanel taskId={taskId} projectId={projectId} />
				</div>

				{/* Diff Viewer Section with inline feedback support */}
				<div data-testid="diff-viewer-section" className="diff-viewer-section">
					{diffLoading ? (
						<div className="diff-loading">
							<Icon name="loader" size={24} />
							<span>Loading diff...</span>
						</div>
					) : diffFiles.length > 0 ? (
						<div className="diff-content">
							{/* Render diff files with inline feedback support */}
							{diffFiles.map((file) => (
								<DiffFile
									key={file.path}
									file={file}
									expanded={true}
									viewMode={diffMode}
									comments={[]}
									activeLineNumber={null}
									onToggle={() => {}}
									onLineClick={() => {}}
									onAddComment={() => Promise.resolve()}
									onResolveComment={() => {}}
									onWontFixComment={() => {}}
									onDeleteComment={() => {}}
									onCloseThread={() => {}}
									inlineFeedback={getFeedbackForFile(file.path)}
									onAddInlineFeedback={handleAddInlineFeedback}
								/>
							))}
							{feedbackError && (
								<div className="feedback-error">{feedbackError}</div>
							)}
						</div>
					) : selectedFile ? (
						<div className="diff-content">
							<div data-testid="selected-file-indicator" className="selected-file-indicator">
								<Icon name="file" size={14} />
								<span>{selectedFile}</span>
							</div>

							<div
								data-testid={`file-expanded-${selectedFile}`}
								className="file-expanded-indicator"
								style={{ display: expandedFiles.has(selectedFile) ? 'block' : 'none' }}
							>
								<Icon name="chevron-down" size={12} />
							</div>

							<div
								data-testid="diff-tree-mode"
								style={{ display: viewMode === 'tree' ? 'block' : 'none' }}
							>
								<Icon name="folder" size={12} />
							</div>

							<div
								data-testid="unified-diff-mode"
								style={{ display: diffMode === 'unified' ? 'block' : 'none' }}
							>
								Unified View
							</div>
						</div>
					) : (
						<div className="diff-placeholder">
							<Icon name="file-text" size={32} />
							<p>Select a file to view its changes</p>
						</div>
					)}
				</div>
			</div>

			{/* Full Diff Modal */}
			{modalOpen && projectId && (
				<DiffViewModal
					open={modalOpen}
					taskId={taskId}
					projectId={projectId}
					onClose={handleCloseModal}
				/>
			)}
		</div>
	);
}
