// FileList component placeholder for TDD
// This component will be implemented after tests are written and failing

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

// Placeholder component that will fail tests until properly implemented
export function FileList(_props: FileListProps): never {
  // This is intentionally a non-functional placeholder
  // Tests should fail until the real implementation is created
  throw new Error('FileList component not yet implemented - this is expected in TDD phase');
}