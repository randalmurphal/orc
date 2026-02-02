import { useState, useEffect, useCallback, useMemo } from 'react';
import { create } from '@bufbuild/protobuf';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { FileList } from './FileList';
import { DiffFile } from './diff/DiffFile';
import { taskClient } from '@/lib/client';
import {
  GetDiffRequestSchema,
  GetFileDiffRequestSchema,
} from '@/gen/orc/v1/task_pb';
import type { DiffResult, FileDiff } from '@/gen/orc/v1/common_pb';
import { toast } from '@/stores/uiStore';

export interface FilesPanelProps {
  taskId: string;
  projectId: string;
}

export function FilesPanel({ taskId, projectId }: FilesPanelProps) {
  const [diff, setDiff] = useState<DiffResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // UI State
  const [viewMode, setViewMode] = useState<'list' | 'tree'>('list');
  const [layoutMode, setLayoutMode] = useState<'split' | 'full'>('split');
  const [diffMode, setDiffMode] = useState<'split' | 'unified'>('split');
  const [statusFilter, setStatusFilter] = useState<'all' | 'added' | 'modified' | 'deleted'>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [expandedFiles, setExpandedFiles] = useState<Set<string>>(new Set());

  // File selection state
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [selectedFileData, setSelectedFileData] = useState<FileDiff | null>(null);
  const [fileLoadingStates, setFileLoadingStates] = useState<Map<string, boolean>>(new Map());
  const [fileDiffCache, setFileDiffCache] = useState<Map<string, FileDiff>>(new Map());

  // Load diff overview
  const loadDiff = useCallback(async () => {
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
      const errorMsg = e instanceof Error ? e.message : 'Failed to load diff';
      setError(errorMsg);
      toast.error(errorMsg);
    } finally {
      setLoading(false);
    }
  }, [projectId, taskId]);

  // Load specific file diff
  const loadFileDiff = useCallback(async (filePath: string) => {
    // Check cache first
    if (fileDiffCache.has(filePath)) {
      setSelectedFileData(fileDiffCache.get(filePath)!);
      return;
    }

    setFileLoadingStates(prev => new Map(prev.set(filePath, true)));
    try {
      const response = await taskClient.getFileDiff(
        create(GetFileDiffRequestSchema, { projectId, taskId, filePath })
      );
      if (response.file) {
        // Cache the result
        setFileDiffCache(prev => new Map(prev.set(filePath, response.file!)));
        setSelectedFileData(response.file);
      }
    } catch (e) {
      const errorMsg = e instanceof Error ? e.message : 'File too large';
      toast.error(`Failed to load diff for ${filePath}: ${errorMsg}`);
    } finally {
      setFileLoadingStates(prev => {
        const next = new Map(prev);
        next.delete(filePath);
        return next;
      });
    }
  }, [projectId, taskId, fileDiffCache]);

  // Handle file selection
  const handleFileSelect = useCallback((filePath: string) => {
    setSelectedFile(filePath);
    loadFileDiff(filePath);
  }, [loadFileDiff]);

  // Handle file expand/collapse
  const handleFileExpand = useCallback((filePath: string) => {
    setExpandedFiles(prev => {
      const next = new Set(prev);
      if (next.has(filePath)) {
        next.delete(filePath);
      } else {
        next.add(filePath);
      }
      return next;
    });
  }, []);

  // Filter and search files
  const filteredFiles = useMemo(() => {
    if (!diff) return [];

    let files = diff.files;

    // Apply status filter
    if (statusFilter !== 'all') {
      files = files.filter(file => file.status === statusFilter);
    }

    // Apply search filter
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      files = files.filter(file =>
        file.path.toLowerCase().includes(query)
      );
    }

    return files;
  }, [diff, statusFilter, searchQuery]);

  // Expand/collapse all directories
  const expandAll = useCallback(() => {
    // This would expand all directories in tree view
    // For now, just expand all files
    if (!diff) return;
    setExpandedFiles(new Set(diff.files.map(f => f.path)));
  }, [diff]);

  const collapseAll = useCallback(() => {
    setExpandedFiles(new Set());
  }, []);

  // Retry loading diff
  const retryLoadDiff = useCallback(() => {
    loadDiff();
  }, [loadDiff]);

  // Clear search
  const clearSearch = useCallback(() => {
    setSearchQuery('');
  }, []);

  useEffect(() => {
    loadDiff();
  }, [loadDiff]);

  // Loading state
  if (loading) {
    return (
      <div data-testid="files-panel" className="files-panel">
        <div data-testid="files-panel-loading" className="files-panel-loading">
          <div className="loading-spinner" />
          <span>Loading file changes...</span>
        </div>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div data-testid="files-panel" className="files-panel">
        <div data-testid="files-panel-error" className="files-panel-error">
          <Icon name="alert-circle" size={24} />
          <span>{error}</span>
          <Button
            data-testid="retry-load-diff"
            variant="secondary"
            size="sm"
            onClick={retryLoadDiff}
          >
            Retry
          </Button>
        </div>
      </div>
    );
  }

  // Empty state
  if (!diff || diff.files.length === 0) {
    return (
      <div data-testid="files-panel" className="files-panel">
        <div data-testid="files-panel-empty" className="files-panel-empty">
          <Icon name="file-text" size={32} />
          <span>No files changed</span>
        </div>
      </div>
    );
  }

  // Search no results
  if (filteredFiles.length === 0 && (statusFilter !== 'all' || searchQuery.trim())) {
    return (
      <div data-testid="files-panel" className="files-panel">
        <div data-testid="files-panel-toolbar" className="files-panel-toolbar">
          {/* Toolbar content */}
          <div className="toolbar-left">
            <input
              data-testid="file-search"
              type="text"
              placeholder="Search files..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="search-input"
            />
            {searchQuery && (
              <Button
                data-testid="clear-search"
                variant="ghost"
                size="sm"
                onClick={clearSearch}
              >
                <Icon name="x" size={14} />
              </Button>
            )}
          </div>
        </div>
        <div data-testid="search-no-results" className="search-no-results">
          <Icon name="search" size={32} />
          <span>No files match your search</span>
        </div>
      </div>
    );
  }

  const isFileLoading = selectedFile ? fileLoadingStates.get(selectedFile) || false : false;

  return (
    <div
      data-testid="files-panel"
      className={`files-panel ${layoutMode === 'full' ? 'full-width' : ''}`}
    >
      {/* Toolbar */}
      <div data-testid="files-panel-toolbar" className="files-panel-toolbar">
        <div className="toolbar-left">
          {/* Search */}
          <input
            data-testid="file-search"
            type="text"
            placeholder="Search files..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="search-input"
          />
          {searchQuery && (
            <Button
              data-testid="clear-search"
              variant="ghost"
              size="sm"
              onClick={clearSearch}
            >
              <Icon name="x" size={14} />
            </Button>
          )}

          {/* Status Filter */}
          <select
            data-testid="status-filter"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as typeof statusFilter)}
            className="status-filter"
          >
            <option value="all">All files</option>
            <option value="added">Added</option>
            <option value="modified">Modified</option>
            <option value="deleted">Deleted</option>
          </select>

          {/* View Mode Toggle */}
          <div data-testid="view-mode-toggle" className="view-mode-toggle">
            <Button
              variant={viewMode === 'list' ? 'primary' : 'ghost'}
              size="sm"
              className={viewMode === 'list' ? 'active' : ''}
              onClick={() => setViewMode('list')}
            >
              List
            </Button>
            <Button
              variant={viewMode === 'tree' ? 'primary' : 'ghost'}
              size="sm"
              className={viewMode === 'tree' ? 'active' : ''}
              onClick={() => setViewMode('tree')}
            >
              Tree
            </Button>
          </div>

          {/* Expand/Collapse All (for tree mode) */}
          {viewMode === 'tree' && (
            <>
              <Button
                data-testid="expand-all"
                variant="ghost"
                size="sm"
                onClick={expandAll}
              >
                Expand All
              </Button>
              <Button
                data-testid="collapse-all"
                variant="ghost"
                size="sm"
                onClick={collapseAll}
              >
                Collapse All
              </Button>
            </>
          )}
        </div>

        <div className="toolbar-right">
          {/* File count and stats */}
          <div className="file-stats">
            <span>{filteredFiles.length} files</span>
            {diff.stats && (
              <>
                <span className="additions">+{diff.stats.additions}</span>
                <span className="deletions">-{diff.stats.deletions}</span>
              </>
            )}
          </div>

          {/* Layout Toggle */}
          <Button
            data-testid="layout-toggle"
            variant="ghost"
            size="sm"
            onClick={() => setLayoutMode(prev => prev === 'split' ? 'full' : 'split')}
          >
            <Icon name={layoutMode === 'split' ? 'maximize' : 'minimize-2'} size={14} />
          </Button>

          {/* Diff Mode Toggle */}
          <div data-testid="diff-mode-toggle" className="diff-mode-toggle">
            <Button
              variant={diffMode === 'split' ? 'primary' : 'ghost'}
              size="sm"
              className={diffMode === 'split' ? 'active' : ''}
              onClick={() => setDiffMode('split')}
            >
              Split
            </Button>
            <Button
              variant={diffMode === 'unified' ? 'primary' : 'ghost'}
              size="sm"
              className={diffMode === 'unified' ? 'active' : ''}
              onClick={() => setDiffMode('unified')}
            >
              Unified
            </Button>
          </div>
        </div>
      </div>

      {/* Panel Content */}
      <div data-testid="files-panel-content" className="files-panel-content">
        {/* Files List Section */}
        <div data-testid="files-list-section" className="files-list-section">
          <FileList
            files={filteredFiles}
            stats={diff.stats!}
            loading={false}
            onFileSelect={handleFileSelect}
            onFileExpand={handleFileExpand}
            selectedFile={selectedFile}
            expandedFiles={expandedFiles}
            viewMode={viewMode}
            statusFilter={statusFilter}
            showFilter={false} // Filter is in toolbar
          />
        </div>

        {/* Diff View Section */}
        <div data-testid="diff-view-section" className="diff-view-section">
          {isFileLoading ? (
            <div data-testid="file-diff-loading" className="file-diff-loading">
              <div className="loading-spinner" />
              <span>Loading diff...</span>
            </div>
          ) : selectedFile && selectedFileData ? (
            <div data-testid="diff-view-content" className="diff-view-content">
              <DiffFile
                file={selectedFileData}
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
              />
            </div>
          ) : selectedFile ? (
            <div data-testid="file-load-error" className="file-load-error">
              <Icon name="alert-circle" size={24} />
              <span>Failed to load file diff</span>
            </div>
          ) : (
            <div className="diff-view-placeholder">
              <Icon name="file-text" size={32} />
              <span>Select a file to view its changes</span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}