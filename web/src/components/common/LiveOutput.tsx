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
  selectable = false
}: LiveOutputProps) {
  const [searchTerm, setSearchTerm] = useState('');
  const [searchIndex, setSearchIndex] = useState(0);
  const [selectedLines, setSelectedLines] = useState<number[]>([]);
  const containerRef = useRef<HTMLDivElement>(null);

  const { scrollRef, isAtBottom, scrollToBottom } = useAutoScroll({
    enabled: autoScroll,
    smooth: true
  });

  const parseOutputLine = (line: string, _index: number): OutputLine => {
    const timestamp = formatTime();

    // Determine line type based on content
    let level: OutputLine['level'] = 'default';
    if (line.includes('✓') || line.toLowerCase().includes('success')) {
      level = 'success';
    } else if (line.includes('→') || line.toLowerCase().includes('info')) {
      level = 'info';
    } else if (line.includes('✗') || line.toLowerCase().includes('error')) {
      level = 'error';
    } else if (line.includes('⚠') || line.toLowerCase().includes('warning')) {
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

  const filteredLines = useMemo(() => {
    const levelOrder: Record<string, number> = {
      'debug': 0,
      'info': 1,
      'warning': 2,
      'error': 3
    };

    let lines = parsedLines;

    // Apply level filtering
    if (filterByLevel && minLevel) {
      const minLevelValue = levelOrder[minLevel.toLowerCase()] || 0;
      lines = lines.filter(line => {
        const lineLevelValue = levelOrder[line.level] || 1;
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
  }, [parsedLines, filterByLevel, minLevel, searchable, searchTerm, maxLines]);

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
        <span data-testid="empty-output-message">No output available</span>
      </div>
    );
  }

  return (
    <div data-testid="live-output" data-task-id={taskId} className="space-y-2">
      {/* Controls */}
      {(searchable || allowCopy || filterByLevel) && (
        <div className="flex items-center gap-2 text-sm">
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
            </div>
          )}

          {filterByLevel && (
            <select
              data-testid="level-filter"
              value={minLevel}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="px-2 py-1 border border-gray-300 rounded text-sm"
            >
              <option value="debug">Debug+</option>
              <option value="info">Info+</option>
              <option value="warning">Warning+</option>
              <option value="error">Error only</option>
            </select>
          )}

          {allowCopy && (
            <button
              onClick={handleCopyToClipboard}
              data-testid="copy-output-btn"
              className="px-2 py-1 text-xs bg-blue-100 text-blue-800 rounded hover:bg-blue-200"
            >
              Copy {selectedLines.length > 0 ? 'Selected' : 'All'}
            </button>
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
          // Assign to both refs
          (scrollRef as React.MutableRefObject<HTMLDivElement | null>).current = el;
          (containerRef as React.MutableRefObject<HTMLDivElement | null>).current = el;
        }}
        data-testid="output-container"
        className="bg-gray-900 text-gray-100 p-3 rounded font-mono text-sm overflow-y-auto max-h-64"
        style={{ scrollBehavior: 'smooth' }}
      >
        {filteredLines.map((line, index) => {
          const isSelected = selectedLines.includes(index);
          const isSearchMatch = searchTerm && line.content.toLowerCase().includes(searchTerm.toLowerCase());
          const isCurrentSearchMatch = searchMatches[searchIndex] === index;

          return (
            <div
              key={`${taskId}-line-${index}`}
              data-testid={isSearchMatch ? 'highlighted-line' : 'output-line'}
              className={`flex items-start gap-2 py-0.5 hover:bg-gray-800 cursor-pointer ${getLineColor(line.level)} ${
                isSelected ? 'bg-blue-50 border-l-4 border-blue-400' : ''
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
              <span className="break-all">
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
