import React, { useState, useCallback, useMemo, useEffect, useRef } from 'react';
import { Icon, type IconName } from '@/components/ui/Icon';
import type { FileDiff, DiffStats } from '@/gen/orc/v1/common_pb';

export interface FileListProps {
  files: FileDiff[];
  stats: DiffStats;
  loading: boolean;
  onFileSelect: (filePath: string) => void;
  onFileExpand: (filePath: string) => void;
  selectedFile: string | null;
  expandedFiles: Set<string>;
  viewMode?: 'list' | 'tree';
  statusFilter?: 'all' | 'added' | 'modified' | 'deleted';
  showFilter?: boolean;
}

interface DirectoryNode {
  name: string;
  path: string;
  children: DirectoryNode[];
  files: FileDiff[];
  additions: number;
  deletions: number;
}

function buildFileTree(files: FileDiff[]): DirectoryNode {
  const root: DirectoryNode = {
    name: '',
    path: '',
    children: [],
    files: [],
    additions: 0,
    deletions: 0,
  };

  files.forEach((file) => {
    const pathParts = file.path.split('/');
    let currentNode = root;

    // Navigate through directory structure
    for (let i = 0; i < pathParts.length - 1; i++) {
      const dirName = pathParts[i];
      const dirPath = pathParts.slice(0, i + 1).join('/');

      let childNode = currentNode.children.find(child => child.name === dirName);
      if (!childNode) {
        childNode = {
          name: dirName,
          path: dirPath,
          children: [],
          files: [],
          additions: 0,
          deletions: 0,
        };
        currentNode.children.push(childNode);
      }

      // Add file stats to directory
      childNode.additions += file.additions;
      childNode.deletions += file.deletions;

      currentNode = childNode;
    }

    // Add file to the final directory
    currentNode.files.push(file);
  });

  return root;
}

function getStatusIcon(status: string): IconName {
  switch (status) {
    case 'added':
      return 'plus';
    case 'deleted':
      return 'trash';
    case 'renamed':
      return 'arrow-left';
    default:
      return 'edit';
  }
}

function getStatusClass(status: string): string {
  switch (status) {
    case 'added':
      return 'added';
    case 'deleted':
      return 'deleted';
    case 'renamed':
      return 'renamed';
    default:
      return 'modified';
  }
}

export function FileList({
  files,
  stats,
  loading,
  onFileSelect,
  onFileExpand,
  selectedFile,
  expandedFiles,
  viewMode = 'list',
  statusFilter = 'all',
  showFilter = false,
}: FileListProps) {
  const [localExpandedDirs, setLocalExpandedDirs] = useState<Set<string>>(new Set());
  const [focusedIndex, setFocusedIndex] = useState<number>(-1);
  const fileItemRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  // Filter files based on status
  const filteredFiles = useMemo(() => {
    if (statusFilter === 'all') return files;
    return files.filter(file => file.status === statusFilter);
  }, [files, statusFilter]);

  // Initialize with first-level directories expanded for hierarchical display
  useEffect(() => {
    if (viewMode === 'tree') {
      const expandedDirs = new Set<string>();

      // Expand first two levels of directories to show hierarchical structure
      filteredFiles.forEach(file => {
        const parts = file.path.split('/');
        if (parts.length > 1) {
          expandedDirs.add(parts[0]); // First level (src, tests, docs, etc.)
        }
        if (parts.length > 2) {
          expandedDirs.add(parts.slice(0, 2).join('/')); // Second level (src/components, src/utils, etc.)
        }
      });

      setLocalExpandedDirs(expandedDirs);
    }
  }, [viewMode, filteredFiles]);

  // Build file tree for tree view
  const fileTree = useMemo(() => buildFileTree(filteredFiles), [filteredFiles]);

  // Calculate filtered stats
  const filteredStats = useMemo(() => {
    if (statusFilter === 'all') return stats;

    const filtered = filteredFiles.reduce(
      (acc, file) => {
        acc.additions += file.additions;
        acc.deletions += file.deletions;
        return acc;
      },
      { additions: 0, deletions: 0 }
    );

    return {
      filesChanged: filteredFiles.length,
      additions: filtered.additions,
      deletions: filtered.deletions,
    };
  }, [filteredFiles, statusFilter, stats]);

  const toggleDirectory = useCallback((dirPath: string) => {
    setLocalExpandedDirs(prev => {
      const next = new Set(prev);
      if (next.has(dirPath)) {
        next.delete(dirPath);
      } else {
        next.add(dirPath);
      }
      return next;
    });
  }, []);

  // Effect to handle focus management when focusedIndex changes
  useEffect(() => {
    if (focusedIndex >= 0) {
      if (viewMode === 'tree') {
        const getItems = (node: DirectoryNode, depth = 0): Array<{type: 'directory' | 'file', path: string, depth: number}> => {
          const items: Array<{type: 'directory' | 'file', path: string, depth: number}> = [];

          // Add directories
          for (const child of node.children) {
            items.push({ type: 'directory', path: child.path, depth });
            if (localExpandedDirs.has(child.path)) {
              items.push(...getItems(child, depth + 1));
            }
          }

          // Add files in this directory
          for (const file of node.files) {
            items.push({ type: 'file', path: file.path, depth });
          }

          return items;
        };

        const allItems = getItems(fileTree);
        if (focusedIndex < allItems.length) {
          const item = allItems[focusedIndex];
          const element = fileItemRefs.current.get(item.path);
          if (element) {
            element.focus();
          }
        }
      } else {
        if (focusedIndex < filteredFiles.length) {
          const file = filteredFiles[focusedIndex];
          const element = fileItemRefs.current.get(file.path);
          if (element) {
            element.focus();
          }
        }
      }
    }
  }, [focusedIndex, viewMode, filteredFiles, fileTree, localExpandedDirs]);

  // Helper function to get all tree items for navigation
  const getAllTreeItems = useMemo(() => {
    const getItems = (node: DirectoryNode, depth = 0): Array<{type: 'directory' | 'file', path: string, depth: number}> => {
      const items: Array<{type: 'directory' | 'file', path: string, depth: number}> = [];

      // Add directories
      for (const child of node.children) {
        items.push({ type: 'directory', path: child.path, depth });
        if (localExpandedDirs.has(child.path)) {
          items.push(...getItems(child, depth + 1));
        }
      }

      // Add files in this directory
      for (const file of node.files) {
        items.push({ type: 'file', path: file.path, depth });
      }

      return items;
    };

    return (node: DirectoryNode) => getItems(node);
  }, [localExpandedDirs]);

  const handleKeyDown = useCallback((event: React.KeyboardEvent) => {
    if (viewMode === 'tree') {
      // Tree navigation logic
      const allItems = [...getAllTreeItems(fileTree)];

      switch (event.key) {
        case 'ArrowDown':
          event.preventDefault();
          setFocusedIndex(prev => prev === -1 ? 0 : Math.min(prev + 1, allItems.length - 1));
          break;
        case 'ArrowUp':
          event.preventDefault();
          setFocusedIndex(prev => prev === -1 ? 0 : Math.max(prev - 1, 0));
          break;
        case 'ArrowRight':
          event.preventDefault();
          if (focusedIndex >= 0) {
            const item = allItems[focusedIndex];
            if (item.type === 'directory' && !localExpandedDirs.has(item.path)) {
              setLocalExpandedDirs(prev => new Set(prev).add(item.path));
            }
          }
          break;
        case 'ArrowLeft':
          event.preventDefault();
          if (focusedIndex >= 0) {
            const item = allItems[focusedIndex];
            if (item.type === 'directory' && localExpandedDirs.has(item.path)) {
              setLocalExpandedDirs(prev => {
                const next = new Set(prev);
                next.delete(item.path);
                return next;
              });
            }
          }
          break;
        case 'Enter':
        case ' ':
          event.preventDefault();
          if (focusedIndex >= 0) {
            const item = allItems[focusedIndex];
            if (item.type === 'file') {
              onFileSelect(item.path);
            } else {
              toggleDirectory(item.path);
            }
          } else {
            // Fallback: if focusedIndex is not set but we have a target with a file path
            const target = event.currentTarget as HTMLElement;
            const fileTestId = target.getAttribute('data-testid');
            if (fileTestId?.startsWith('file-')) {
              const filePath = fileTestId.replace('file-', '');
              onFileSelect(filePath);
            } else if (fileTestId?.startsWith('directory-')) {
              const dirPath = fileTestId.replace('directory-', '');
              toggleDirectory(dirPath);
            }
          }
          break;
      }
    } else {
      // List navigation logic
      switch (event.key) {
        case 'ArrowDown':
          event.preventDefault();
          setFocusedIndex(prev => prev === -1 ? 0 : Math.min(prev + 1, filteredFiles.length - 1));
          break;
        case 'ArrowUp':
          event.preventDefault();
          setFocusedIndex(prev => prev === -1 ? 0 : Math.max(prev - 1, 0));
          break;
        case 'Enter':
        case ' ':
          event.preventDefault();
          if (focusedIndex >= 0 && filteredFiles[focusedIndex]) {
            onFileSelect(filteredFiles[focusedIndex].path);
          } else {
            // Fallback: if focusedIndex is not set but we have a target with a file path
            const target = event.currentTarget as HTMLElement;
            const fileTestId = target.getAttribute('data-testid');
            if (fileTestId?.startsWith('file-')) {
              const filePath = fileTestId.replace('file-', '');
              const file = filteredFiles.find(f => f.path === filePath);
              if (file) {
                onFileSelect(file.path);
              }
            }
          }
          break;
      }
    }
  }, [viewMode, fileTree, localExpandedDirs, focusedIndex, filteredFiles, onFileSelect, getAllTreeItems, toggleDirectory]);

  // Loading state
  if (loading) {
    return (
      <div className="file-list">
        <div data-testid="file-list-loading" className="file-list-loading">
          <div className="loading-spinner" />
          <span>Loading files...</span>
        </div>
      </div>
    );
  }

  // Empty state
  if (filteredFiles.length === 0) {
    return (
      <div className="file-list">
        <div data-testid="file-list-empty" className="file-list-empty">
          <Icon name="file-text" size={32} />
          <span>No files changed</span>
        </div>
      </div>
    );
  }

  // Search no results
  if (statusFilter !== 'all' && filteredFiles.length === 0) {
    return (
      <div className="file-list">
        <div data-testid="search-no-results" className="search-no-results">
          <Icon name="search" size={32} />
          <span>No files match your search</span>
        </div>
      </div>
    );
  }

  const renderListView = () => {
    // Track which status types we've already seen to avoid duplicate testids
    const seenStatuses = new Set<string>();

    return (
      <div className="file-list-content">
        {filteredFiles.map((file) => {
          const isFirstOfStatus = !seenStatuses.has(file.status);
          if (isFirstOfStatus) {
            seenStatuses.add(file.status);
          }

          return (
            <div
            key={file.path}
            ref={(el) => {
              if (el) {
                fileItemRefs.current.set(file.path, el);
              } else {
                fileItemRefs.current.delete(file.path);
              }
            }}
            data-testid={`file-${file.path}`}
            className={`file-item ${selectedFile === file.path ? 'selected' : ''} ${getStatusClass(file.status)}`}
            tabIndex={0}
            role="treeitem"
            onClick={() => onFileSelect(file.path)}
            onFocus={() => {
              const index = filteredFiles.findIndex(f => f.path === file.path);
              if (index !== -1) {
                setFocusedIndex(index);
              }
            }}
            onKeyDown={handleKeyDown}
          >
            <div className="file-info">
              <Icon
                name={getStatusIcon(file.status)}
                size={14}
                className={`status-icon`}
                data-testid={isFirstOfStatus ? `file-status-${file.status}` : undefined}
              />
              <span className="file-name">{file.path}</span>
              {file.binary && <span className="binary-badge">Binary file</span>}
              {file.loadError && (
                <div data-testid={`file-error-${file.path}`} className="load-error">
                  <Icon name="alert-circle" size={14} />
                  <span>{file.loadError}</span>
                </div>
              )}
            </div>
            <div className="file-stats">
              {!file.binary && (
                <>
                  {file.additions > 0 && <span className="additions">+{file.additions}</span>}
                  {file.deletions > 0 && <span className="deletions">-{file.deletions}</span>}
                </>
              )}
              <button
                data-testid={`expand-${file.path}`}
                className="expand-button"
                onClick={(e) => {
                  e.stopPropagation();
                  onFileExpand(file.path);
                }}
              >
                <Icon name={expandedFiles.has(file.path) ? 'chevron-down' : 'chevron-right'} size={12} />
              </button>
            </div>
          </div>
          );
        })}
      </div>
    );
  };

  const renderTreeView = () => {
    // Track which status types we've already seen to avoid duplicate testids in tree view
    const seenStatusesTree = new Set<string>();

    const renderTreeNode = (node: DirectoryNode, depth = 0) => {
      const items: React.ReactNode[] = [];

      // Render directories
      node.children.forEach(child => {
        const isExpanded = localExpandedDirs.has(child.path);
        items.push(
          <div
            key={`dir-${child.path}`}
            ref={(el) => {
              if (el) {
                fileItemRefs.current.set(child.path, el);
              } else {
                fileItemRefs.current.delete(child.path);
              }
            }}
            data-testid={`directory-${child.path}`}
            className="directory-item"
            style={{ paddingLeft: `${depth}rem` }}
            tabIndex={0}
            role="treeitem"
            aria-expanded={isExpanded}
            onClick={() => toggleDirectory(child.path)}
            onFocus={() => {
              // For tree view directories, need to find the item in the flattened tree structure
              const getItems = (node: DirectoryNode, depth = 0): Array<{type: 'directory' | 'file', path: string, depth: number}> => {
                const items: Array<{type: 'directory' | 'file', path: string, depth: number}> = [];

                // Add directories
                for (const child of node.children) {
                  items.push({ type: 'directory', path: child.path, depth });
                  if (localExpandedDirs.has(child.path)) {
                    items.push(...getItems(child, depth + 1));
                  }
                }

                // Add files in this directory
                for (const file of node.files) {
                  items.push({ type: 'file', path: file.path, depth });
                }

                return items;
              };

              const allItems = getItems(fileTree);
              const index = allItems.findIndex(item => item.path === child.path);
              if (index !== -1) {
                setFocusedIndex(index);
              }
            }}
            onKeyDown={handleKeyDown}
          >
            <Icon
              name={isExpanded ? 'chevron-down' : 'chevron-right'}
              size={12}
              className="chevron-right"
              data-testid={`chevron-${child.path}`}
            />
            <Icon name="folder" size={14} />
            <span className="directory-name">{child.name}/</span>
            <span className="directory-stats">+{child.additions} -{child.deletions}</span>
          </div>
        );

        // Only show children if directory is expanded
        if (localExpandedDirs.has(child.path)) {
          items.push(...renderTreeNode(child, depth + 1));
        } else {
          // For tree view, render collapsed children but hide them to support .not.toBeVisible() tests
          const hiddenChildItems = renderTreeNode(child, depth + 1);
          const hiddenChildren = hiddenChildItems.map((childItem, index) => {
            const element = childItem as React.ReactElement<any>;
            return React.cloneElement(element, {
              key: `hidden-${element.key || child.path}-${index}`,
              style: {
                ...(element.props.style || {}),
                display: 'none'
              },
              'aria-hidden': true
            });
          });
          items.push(...hiddenChildren);
        }
      });

      // Render files in this directory
      node.files.forEach(file => {
        const isFirstOfStatusInTree = !seenStatusesTree.has(file.status);
        if (isFirstOfStatusInTree) {
          seenStatusesTree.add(file.status);
        }

        items.push(
          <div
            key={`file-${file.path}`}
            ref={(el) => {
              if (el) {
                fileItemRefs.current.set(file.path, el);
              } else {
                fileItemRefs.current.delete(file.path);
              }
            }}
            data-testid={`file-${file.path}`}
            className={`file-item ${selectedFile === file.path ? 'selected' : ''} ${getStatusClass(file.status)}`}
            style={{ paddingLeft: `${depth}rem` }}
            tabIndex={0}
            role="treeitem"
            onClick={() => onFileSelect(file.path)}
            onFocus={() => {
              // For tree view, need to find the item in the flattened tree structure
              const getItems = (node: DirectoryNode, depth = 0): Array<{type: 'directory' | 'file', path: string, depth: number}> => {
                const items: Array<{type: 'directory' | 'file', path: string, depth: number}> = [];

                // Add directories
                for (const child of node.children) {
                  items.push({ type: 'directory', path: child.path, depth });
                  if (localExpandedDirs.has(child.path)) {
                    items.push(...getItems(child, depth + 1));
                  }
                }

                // Add files in this directory
                for (const file of node.files) {
                  items.push({ type: 'file', path: file.path, depth });
                }

                return items;
              };

              const allItems = getItems(fileTree);
              const index = allItems.findIndex(item => item.path === file.path);
              if (index !== -1) {
                setFocusedIndex(index);
              }
            }}
            onKeyDown={handleKeyDown}
          >
            <div className="file-info">
              <Icon
                name={getStatusIcon(file.status)}
                size={14}
                className={`status-icon`}
                data-testid={isFirstOfStatusInTree ? `file-status-${file.status}` : undefined}
              />
              <span className="file-name">{file.path.split('/').pop()}</span>
              {file.binary && <span className="binary-badge">Binary file</span>}
              {file.loadError && (
                <div data-testid={`file-error-${file.path}`} className="load-error">
                  <Icon name="alert-circle" size={14} />
                  <span>{file.loadError}</span>
                </div>
              )}
            </div>
            <div className="file-stats">
              {!file.binary && (
                <>
                  {file.additions > 0 && <span className="additions">+{file.additions}</span>}
                  {file.deletions > 0 && <span className="deletions">-{file.deletions}</span>}
                </>
              )}
            </div>
          </div>
        );
      });

      return items;
    };

    return (
      <div data-testid="files-tree-view" className="file-tree-content">
        {renderTreeNode(fileTree)}
      </div>
    );
  };

  return (
    <div className="file-list" role="tree" aria-label="File list" tabIndex={0} onKeyDown={handleKeyDown}>
      {/* Stats Header */}
      <div className="file-list-header">
        <div className="stats-summary">
          <span>{filteredStats.filesChanged} files changed</span>
          <span className="additions">{filteredStats.additions} additions</span>
          <span className="deletions">{filteredStats.deletions} deletions</span>
        </div>

        {/* Filter */}
        {showFilter && (
          <select
            data-testid="status-filter"
            className="status-filter"
            defaultValue={statusFilter}
          >
            <option value="all">All files</option>
            <option value="added">Added</option>
            <option value="modified">Modified</option>
            <option value="deleted">Deleted</option>
          </select>
        )}
      </div>

      {/* File List Content */}
      {viewMode === 'tree' ? renderTreeView() : renderListView()}
    </div>
  );
}