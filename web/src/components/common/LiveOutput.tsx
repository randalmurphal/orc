import { useState, useEffect, useMemo, useRef } from 'react';
import { useAutoScroll } from '../../hooks/useAutoScroll';

export interface LiveOutputProps {
  taskId: string;
  outputLines: string[];
  maxLines: number;
  showTimestamps?: boolean;
  autoScroll?: boolean;
  searchable?: boolean;
  allowCopy?: boolean;
  filterByLevel?: boolean;
  minLevel?: string;
  selectable?: boolean;
  onOpenFile?: (filePath: string, line?: number) => void;
}

interface OutputLine {
  content: string;
  timestamp: string;
  level: 'success' | 'info' | 'error' | 'warning' | 'default';
}

// Format time as HH:MM:SS
function formatTime(): string {
  const now = new Date();
  const hours = String(now.getHours()).padStart(2, '0');
  const minutes = String(now.getMinutes()).padStart(2, '0');
  const seconds = String(now.getSeconds()).padStart(2, '0');
  return `${hours}:${minutes}:${seconds}`;
}

// Parse file reference from line (e.g., "src/main.go:42: error message")
function parseFileReference(content: string): { filePath: string; line?: number } | null {
  const match = content.match(/^([^\s:]+):(\d+):/);
  if (match) {
    return { filePath: match[1], line: parseInt(match[2], 10) };
  }
  return null;
}

export function LiveOutput({
  taskId,
  outputLines,
  maxLines,
  showTimestamps = false,
  autoScroll = true,
  searchable = false,
  allowCopy = false,
  filterByLevel = false,
  minLevel = 'info',
  selectable = false,
  onOpenFile
}: LiveOutputProps) {
  const [searchTerm, setSearchTerm] = useState('');
  const [searchIndex, setSearchIndex] = useState(0);
  const [selectedLines, setSelectedLines] = useState<number[]>([]);
  const [copySuccess, setCopySuccess] = useState(false);
  const [internalMinLevel, setInternalMinLevel] = useState(minLevel);
  const containerRef = useRef<HTMLDivElement>(null);

  const { scrollRef, isAtBottom, scrollToBottom } = useAutoScroll({
    enabled: autoScroll,
    smooth: true
  });

  const parseOutputLine = (line: string, _index: number): OutputLine => {
    const timestamp = formatTime();

    // Determine line type based on content
    let level: OutputLine['level'] = 'default';

    // First, check for [LEVEL] prefix format (e.g., "[DEBUG]", "[INFO]", "[WARN]", "[ERROR]")
    const levelMatch = line.match(/^\[(DEBUG|INFO|WARN|WARNING|ERROR)\]/i);
    if (levelMatch) {
      const parsedLevel = levelMatch[1].toUpperCase();
      if (parsedLevel === 'DEBUG') {
        level = 'info'; // Map debug to info for display
      } else if (parsedLevel === 'INFO') {
        level = 'info';
      } else if (parsedLevel === 'WARN' || parsedLevel === 'WARNING') {
        level = 'warning';
      } else if (parsedLevel === 'ERROR') {
        level = 'error';
      }
    } else if (line.includes('✓') || line.toLowerCase().includes('success')) {
      level = 'success';
    } else if (line.includes('→')) {
      level = 'info';
    } else if (line.includes('✗')) {
      level = 'error';
    } else if (line.includes('⚠')) {
      level = 'warning';
    }

    return {
      content: line,
      timestamp,
      level
    };
  };

  const parsedLines = useMemo(() => {
    return outputLines.map(parseOutputLine);
  }, [outputLines]);

  // Calculate how many lines are hidden due to truncation
  const hiddenLinesCount = useMemo(() => {
    if (parsedLines.length > maxLines) {
      return parsedLines.length - maxLines;
    }
    return 0;
  }, [parsedLines.length, maxLines]);

  // Parse level from original line content (for filtering)
  const getLineFilterLevel = (content: string): string => {
    const levelMatch = content.match(/^\[(DEBUG|INFO|WARN|WARNING|ERROR)\]/i);
    if (levelMatch) {
      const level = levelMatch[1].toUpperCase();
      if (level === 'WARN' || level === 'WARNING') return 'warn';
      return level.toLowerCase();
    }
    return 'default'; // Lines without [LEVEL] prefix
  };

  const filteredLines = useMemo(() => {
    const levelOrder: Record<string, number> = {
      'debug': 0,
      'info': 1,
      'warn': 2,
      'warning': 2,
      'error': 3,
      'default': 1 // Plain text treated as info level
    };

    let lines = parsedLines;

    // Apply level filtering based on [LEVEL] prefix in original content
    if (filterByLevel && internalMinLevel) {
      const minLevelValue = levelOrder[internalMinLevel.toLowerCase()] || 0;
      lines = lines.filter(line => {
        const lineLevel = getLineFilterLevel(line.content);
        // "default" lines (no prefix) pass through all filters
        if (lineLevel === 'default') return true;
        const lineLevelValue = levelOrder[lineLevel] || 1;
        return lineLevelValue >= minLevelValue;
      });
    }

    // Apply search filtering
    if (searchable && searchTerm) {
      lines = lines.filter(line =>
        line.content.toLowerCase().includes(searchTerm.toLowerCase())
      );
    }

    // Apply max lines limit
    if (lines.length > maxLines) {
      return lines.slice(-maxLines);
    }

    return lines;
  }, [parsedLines, filterByLevel, internalMinLevel, searchable, searchTerm, maxLines]);

  const searchMatches = useMemo(() => {
    if (!searchTerm) return [];
    return filteredLines.map((line, index) =>
      line.content.toLowerCase().includes(searchTerm.toLowerCase()) ? index : -1
    ).filter(index => index !== -1);
  }, [filteredLines, searchTerm]);

  const getLineColor = (level: OutputLine['level']): string => {
    switch (level) {
      case 'success':
        return 'text-green-600';
      case 'info':
        return 'text-blue-600';
      case 'error':
        return 'text-red-600';
      case 'warning':
        return 'text-yellow-600';
      default:
        return 'text-gray-700';
    }
  };

  const handleCopyToClipboard = () => {
    const content = selectedLines.length > 0
      ? selectedLines.map(i => filteredLines[i]?.content).join('\n')
      : filteredLines.map(line => line.content).join('\n');

    navigator.clipboard.writeText(content);
    setCopySuccess(true);
    setTimeout(() => setCopySuccess(false), 2000);
  };

  const handleCopyLine = (lineContent: string) => {
    navigator.clipboard.writeText(lineContent);
    setCopySuccess(true);
    setTimeout(() => setCopySuccess(false), 2000);
  };

  const handleLineSelect = (index: number) => {
    if (!selectable) return;

    setSelectedLines(prev => {
      if (prev.includes(index)) {
        return prev.filter(i => i !== index);
      } else {
        return [...prev, index];
      }
    });
  };

  const handleSearchNext = () => {
    if (searchMatches.length === 0) return;
    setSearchIndex(prev => (prev + 1) % searchMatches.length);
  };

  const handleSearchPrev = () => {
    if (searchMatches.length === 0) return;
    setSearchIndex(prev => prev === 0 ? searchMatches.length - 1 : prev - 1);
  };

  // Auto-scroll to bottom when new lines are added
  useEffect(() => {
    if (autoScroll) {
      scrollToBottom();
    }
  }, [outputLines.length, autoScroll, scrollToBottom]);

  if (outputLines.length === 0) {
    return (
      <div data-testid="live-output" data-task-id={taskId} className="flex items-center justify-center h-32 text-gray-500 bg-gray-50 rounded">
        <span data-testid="empty-output-message">No output yet...</span>
      </div>
    );
  }

  // Count total lines and matching lines for filter indicator
  const totalLinesCount = parsedLines.length;
  const filteredLinesCount = filteredLines.length;

  return (
    <div data-testid="live-output" data-task-id={taskId} className="space-y-2">
      {/* Controls */}
      {(searchable || allowCopy || filterByLevel) && (
        <div className="flex items-center gap-2 text-sm flex-wrap">
          {searchable && (
            <div className="flex items-center gap-1 flex-1">
              <input
                type="text"
                data-testid="output-search"
                placeholder="Search output..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="px-2 py-1 border border-gray-300 rounded text-sm flex-1"
              />
              {searchTerm && (
                <>
                  <span data-testid="search-results" className="text-xs text-gray-600">
                    {searchMatches.length} of {parsedLines.length} lines match
                  </span>
                  {searchMatches.length > 0 && (
                    <div className="flex items-center gap-1">
                      <span data-testid="search-count" className="text-xs text-gray-600">
                        {searchIndex + 1} of {searchMatches.length}
                      </span>
                      <button
                        onClick={handleSearchPrev}
                        data-testid="search-prev-btn"
                        className="px-1 py-0.5 text-xs border rounded hover:bg-gray-50"
                      >
                        ↑
                      </button>
                      <button
                        onClick={handleSearchNext}
                        data-testid="search-next-btn"
                        className="px-1 py-0.5 text-xs border rounded hover:bg-gray-50"
                      >
                        ↓
                      </button>
                    </div>
                  )}
                </>
              )}
            </div>
          )}

          {filterByLevel && (
            <>
              <select
                data-testid="level-filter"
                value={internalMinLevel}
                onChange={(e) => setInternalMinLevel(e.target.value)}
                className="px-2 py-1 border border-gray-300 rounded text-sm"
              >
                <option value="debug">Debug+</option>
                <option value="info">Info+</option>
                <option value="warn">Warning+</option>
                <option value="error">Error only</option>
              </select>
              <span data-testid="filter-indicator" className="text-xs text-gray-600">
                Showing {internalMinLevel.toUpperCase()}+ ({filteredLinesCount} of {totalLinesCount} lines)
              </span>
            </>
          )}

          {allowCopy && (
            <>
              <button
                onClick={handleCopyToClipboard}
                data-testid="copy-output-btn"
                className="px-2 py-1 text-xs bg-blue-100 text-blue-800 rounded hover:bg-blue-200"
              >
                Copy {selectedLines.length > 0 ? 'Selected' : 'All'}
              </button>
              {copySuccess && (
                <span data-testid="copy-success-msg" className="text-xs text-green-600">
                  Output copied to clipboard
                </span>
              )}
            </>
          )}
        </div>
      )}

      {/* Truncation indicator - shown before output when lines are hidden */}
      {hiddenLinesCount > 0 && (
        <div
          data-testid="truncation-indicator"
          className="text-center text-gray-500 text-xs py-1 bg-gray-100 rounded"
        >
          ... {hiddenLinesCount} earlier lines hidden
        </div>
      )}

      {/* Output container */}
      <div
        ref={(el) => {
          // Assign to both refs using type assertion
          if (scrollRef && 'current' in scrollRef) {
            (scrollRef as { current: HTMLDivElement | null }).current = el;
          }
          if (containerRef && 'current' in containerRef) {
            (containerRef as { current: HTMLDivElement | null }).current = el;
          }
        }}
        data-testid="output-container"
        className="bg-gray-900 text-gray-100 p-3 rounded font-mono text-sm overflow-y-auto max-h-64"
        style={{ scrollBehavior: 'smooth' }}
      >
        {filteredLines.map((line, index) => {
          const isSelected = selectedLines.includes(index);
          const isSearchMatch = searchTerm && line.content.toLowerCase().includes(searchTerm.toLowerCase());
          const isCurrentSearchMatch = searchMatches[searchIndex] === index;
          const fileRef = selectable ? parseFileReference(line.content) : null;

          return (
            <div
              key={`${taskId}-line-${index}`}
              data-testid={isSearchMatch ? 'highlighted-line' : 'output-line'}
              className={`flex items-start gap-2 py-0.5 hover:bg-gray-800 cursor-pointer ${getLineColor(line.level)} ${
                isSelected ? 'bg-blue-50 border-l-4 border-blue-500' : ''
              } ${
                isCurrentSearchMatch ? 'bg-yellow-900' : ''
              }`}
              onClick={() => handleLineSelect(index)}
            >
              {showTimestamps && (
                <span data-testid="output-timestamp" className="text-gray-500 text-xs shrink-0 w-16">
                  {line.timestamp}
                </span>
              )}
              <span className="break-all flex-1">
                {isSearchMatch && searchTerm ? (
                  <span
                    dangerouslySetInnerHTML={{
                      __html: line.content.replace(
                        new RegExp(searchTerm, 'gi'),
                        '<mark class="bg-yellow-300 text-black">$&</mark>'
                      )
                    }}
                  />
                ) : (
                  line.content.length > 2000 ? line.content.substring(0, 2000) + '...' : line.content
                )}
              </span>
              {/* Contextual actions for selected lines with file references */}
              {isSelected && selectable && (
                <div data-testid="line-actions" className="flex items-center gap-1 shrink-0">
                  {fileRef && (
                    <button
                      data-testid="open-file-btn"
                      onClick={(e) => {
                        e.stopPropagation();
                        onOpenFile?.(fileRef.filePath, fileRef.line);
                      }}
                      className="px-1 py-0.5 text-xs bg-blue-600 text-white rounded hover:bg-blue-700"
                    >
                      Open {fileRef.filePath}:{fileRef.line}
                    </button>
                  )}
                  <button
                    data-testid="copy-line-btn"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleCopyLine(line.content);
                    }}
                    className="px-1 py-0.5 text-xs bg-gray-600 text-white rounded hover:bg-gray-700"
                  >
                    Copy line
                  </button>
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Scroll to bottom button - shown when user has scrolled up */}
      {autoScroll && !isAtBottom && (
        <button
          data-testid="scroll-to-bottom-btn"
          onClick={scrollToBottom}
          className="w-full py-1 text-xs text-center bg-blue-100 text-blue-800 rounded hover:bg-blue-200"
        >
          ↓ Scroll to bottom
        </button>
      )}

      {selectedLines.length > 0 && selectable && (
        <div className="text-xs text-gray-600">
          {selectedLines.length} lines selected
        </div>
      )}
    </div>
  );
}
