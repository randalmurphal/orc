import { useState, useEffect, useCallback, useMemo } from 'react';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { FilesPanel } from './FilesPanel';
import { ChangesTab } from './ChangesTab';
import { DiffViewModal } from '@/components/overlays/DiffViewModal';
import { useCurrentProjectId } from '@/stores';

export interface ChangesTabEnhancedProps {
  taskId: string;
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

  // Handle view mode changes and sync between panels
  const handleViewModeChange = useCallback((newViewMode: 'list' | 'tree') => {
    setIsTransitioning(true);
    setViewMode(newViewMode);
    setTimeout(() => setIsTransitioning(false), 300); // Match transition duration
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

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.ctrlKey || event.metaKey) {
        // Ctrl+Shift+F - Open full diff modal
        if (event.shiftKey && event.key === 'F') {
          event.preventDefault();
          handleExpandDiff();
          return;
        }

        switch (event.key) {
          case 'f': {
            event.preventDefault();
            // Focus file panel
            const filesPanel = document.querySelector('[data-testid="files-panel"]');
            if (filesPanel && filesPanel instanceof HTMLElement) {
              filesPanel.focus();
            }
            break;
          }
          case 't':
            event.preventDefault();
            // Toggle view mode
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
              <Button
                variant="primary"
                size="sm"
                onClick={handleRetryEnhancedView}
              >
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

  // Enhanced view
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
          {/* Mobile panel toggle */}
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
          {/* View mode indicators */}
          <div data-testid="tree-view-active" style={{ display: viewMode === 'tree' ? 'block' : 'none' }}>
            <Icon name="folder" size={14} />
          </div>

          {/* Tablet collapse button */}
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

          {/* Classic view toggle */}
          <Button
            data-testid="classic-diff-toggle"
            variant="ghost"
            size="sm"
            onClick={() => setUseEnhancedView(false)}
          >
            Classic View
          </Button>

          {/* Test error trigger (only in test environment) */}
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
          <FilesPanel
            taskId={taskId}
            projectId={projectId}
          />
        </div>

        {/* Diff Viewer Section */}
        <div data-testid="diff-viewer-section" className="diff-viewer-section">
          {selectedFile ? (
            <div className="diff-content">
              {/* File selection indicator */}
              <div data-testid="selected-file-indicator" className="selected-file-indicator">
                <Icon name="file" size={14} />
                <span>{selectedFile}</span>
              </div>

              {/* Expanded file indicator */}
              <div
                data-testid={`file-expanded-${selectedFile}`}
                className="file-expanded-indicator"
                style={{ display: expandedFiles.has(selectedFile) ? 'block' : 'none' }}
              >
                <Icon name="chevron-down" size={12} />
              </div>

              {/* Tree mode indicator for diff */}
              <div
                data-testid="diff-tree-mode"
                style={{ display: viewMode === 'tree' ? 'block' : 'none' }}
              >
                <Icon name="folder" size={12} />
              </div>

              {/* Unified diff mode indicator */}
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