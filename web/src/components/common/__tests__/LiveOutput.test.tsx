import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { LiveOutput } from '../LiveOutput';
import { useAutoScroll } from '../../../hooks/useAutoScroll';

// Mock the auto-scroll hook
vi.mock('../../../hooks/useAutoScroll', () => ({
  useAutoScroll: vi.fn().mockReturnValue({
    scrollRef: { current: null },
    isAtBottom: true,
    scrollToBottom: vi.fn(),
  }),
}));

const mockUseAutoScroll = vi.mocked(useAutoScroll);

describe('LiveOutput Real-Time Transcript Streaming', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('SC-4.1: displays live output lines with color coding', () => {
    const outputLines = [
      '✓ Reading file: src/main.go',
      '→ Analyzing function signatures...',
      '✗ Error: missing import statement',
      '⚠ Warning: deprecated function used',
      'Normal output line without prefix'
    ];

    render(
      <LiveOutput
        taskId="TASK-001"
        outputLines={outputLines}
        maxLines={50}
        showTimestamps={true}
      />
    );

    const outputContainer = screen.getByTestId('live-output');
    expect(outputContainer).toHaveAttribute('data-task-id', 'TASK-001');

    const lines = screen.getAllByTestId('output-line');
    expect(lines).toHaveLength(5);

    // Check color coding for different line types
    expect(lines[0]).toHaveClass('text-green-600'); // Success (✓)
    expect(lines[1]).toHaveClass('text-blue-600');  // Info (→)
    expect(lines[2]).toHaveClass('text-red-600');   // Error (✗)
    expect(lines[3]).toHaveClass('text-yellow-600'); // Warning (⚠)
    expect(lines[4]).toHaveClass('text-gray-700');  // Default

    // Check content
    expect(lines[0]).toHaveTextContent('✓ Reading file: src/main.go');
    expect(lines[2]).toHaveTextContent('✗ Error: missing import statement');
  });

  it('SC-4.2: auto-scrolls to bottom when new lines arrive', async () => {
    const mockScrollToBottom = vi.fn();

    mockUseAutoScroll.mockReturnValue({
      scrollRef: { current: document.createElement('div') },
      isAtBottom: true,
      scrollToBottom: mockScrollToBottom,
      enableAutoScroll: vi.fn(),
      disableAutoScroll: vi.fn(),
      isAutoScrollEnabled: true,
    });

    let outputLines = ['Line 1', 'Line 2'];

    const { rerender } = render(
      <LiveOutput
        taskId="TASK-002"
        outputLines={outputLines}
        maxLines={50}
        autoScroll={true}
      />
    );

    // Add new lines
    outputLines = ['Line 1', 'Line 2', 'Line 3', 'Line 4'];

    rerender(
      <LiveOutput
        taskId="TASK-002"
        outputLines={outputLines}
        maxLines={50}
        autoScroll={true}
      />
    );

    // Should auto-scroll to bottom when new lines arrive
    await waitFor(() => {
      expect(mockScrollToBottom).toHaveBeenCalled();
    });
  });

  it('SC-4.3: limits display to maxLines and shows truncation indicator', () => {
    const outputLines = Array.from({ length: 100 }, (_, i) => `Line ${i + 1}`);

    render(
      <LiveOutput
        taskId="TASK-003"
        outputLines={outputLines}
        maxLines={50}
        showTimestamps={false}
      />
    );

    // Should only show last 50 lines
    const lines = screen.getAllByTestId('output-line');
    expect(lines).toHaveLength(50);

    // Should show lines 51-100 (last 50)
    expect(lines[0]).toHaveTextContent('Line 51');
    expect(lines[49]).toHaveTextContent('Line 100');

    // Should show truncation indicator
    expect(screen.getByTestId('truncation-indicator')).toBeInTheDocument();
    expect(screen.getByTestId('truncation-indicator')).toHaveTextContent('... 50 earlier lines hidden');
  });

  it('SC-4.4: shows timestamps when enabled', () => {
    const outputLines = [
      'First line',
      'Second line'
    ];

    render(
      <LiveOutput
        taskId="TASK-004"
        outputLines={outputLines}
        maxLines={50}
        showTimestamps={true}
      />
    );

    const timestampElements = screen.getAllByTestId('output-timestamp');
    expect(timestampElements).toHaveLength(2);

    // Timestamps should be in HH:MM:SS format
    timestampElements.forEach(timestamp => {
      expect(timestamp).toHaveTextContent(/^\d{2}:\d{2}:\d{2}$/);
      expect(timestamp).toHaveClass('text-gray-500', 'text-xs');
    });
  });

  it('SC-4.5: allows manual scrolling and disables auto-scroll when not at bottom', () => {
    mockUseAutoScroll.mockReturnValue({
      scrollRef: { current: document.createElement('div') },
      isAtBottom: false, // User scrolled up
      scrollToBottom: vi.fn(),
      enableAutoScroll: vi.fn(),
      disableAutoScroll: vi.fn(),
      isAutoScrollEnabled: false,
    });

    const outputLines = Array.from({ length: 20 }, (_, i) => `Line ${i + 1}`);

    render(
      <LiveOutput
        taskId="TASK-005"
        outputLines={outputLines}
        maxLines={50}
        autoScroll={true}
      />
    );

    const outputContainer = screen.getByTestId('output-container');

    // Simulate user scrolling up
    fireEvent.scroll(outputContainer, { target: { scrollTop: 100 } });

    // Should show scroll-to-bottom button when not at bottom
    expect(screen.getByTestId('scroll-to-bottom-btn')).toBeInTheDocument();
  });

  it('SC-4.6: provides search functionality for output lines', () => {
    const outputLines = [
      'Starting implementation...',
      'Reading configuration file',
      'Error: configuration not found',
      'Retrying with default config',
      'Implementation completed successfully'
    ];

    render(
      <LiveOutput
        taskId="TASK-006"
        outputLines={outputLines}
        maxLines={50}
        searchable={true}
      />
    );

    const searchInput = screen.getByTestId('output-search');
    fireEvent.change(searchInput, { target: { value: 'config' } });

    // Should highlight matching lines
    const highlightedLines = screen.getAllByTestId('highlighted-line');
    expect(highlightedLines).toHaveLength(3); // Lines containing 'config'

    const searchResults = screen.getByTestId('search-results');
    expect(searchResults).toHaveTextContent('3 of 5 lines match');

    // Should show search navigation
    expect(screen.getByTestId('search-prev-btn')).toBeInTheDocument();
    expect(screen.getByTestId('search-next-btn')).toBeInTheDocument();
  });

  it('SC-4.7: handles empty output gracefully', () => {
    render(
      <LiveOutput
        taskId="TASK-007"
        outputLines={[]}
        maxLines={50}
        showTimestamps={true}
      />
    );

    const outputContainer = screen.getByTestId('live-output');
    expect(outputContainer).toBeInTheDocument();

    // Should show empty state message
    expect(screen.getByTestId('empty-output-message')).toHaveTextContent('No output yet...');
    expect(screen.queryByTestId('output-line')).toBeNull();
  });

  it('SC-4.8: supports copy-to-clipboard functionality', () => {
    const outputLines = [
      'Line 1: Important data',
      'Line 2: More data',
      'Line 3: Critical information'
    ];

    // Mock clipboard API
    Object.assign(navigator, {
      clipboard: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
    });

    render(
      <LiveOutput
        taskId="TASK-008"
        outputLines={outputLines}
        maxLines={50}
        allowCopy={true}
      />
    );

    const copyButton = screen.getByTestId('copy-output-btn');
    fireEvent.click(copyButton);

    // Should copy all lines to clipboard
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      'Line 1: Important data\nLine 2: More data\nLine 3: Critical information'
    );

    // Should show success feedback
    expect(screen.getByTestId('copy-success-msg')).toHaveTextContent('Output copied to clipboard');
  });

  it('SC-4.9: filters output by log level when enabled', () => {
    const outputLines = [
      '[DEBUG] Debug message',
      '[INFO] Information message',
      '[WARN] Warning message',
      '[ERROR] Error message',
      'Plain text message'
    ];

    render(
      <LiveOutput
        taskId="TASK-009"
        outputLines={outputLines}
        maxLines={50}
        filterByLevel={true}
        minLevel="WARN"
      />
    );

    const visibleLines = screen.getAllByTestId('output-line');
    // Should only show WARN, ERROR, and plain text (no level specified)
    expect(visibleLines).toHaveLength(3);

    expect(visibleLines[0]).toHaveTextContent('[WARN] Warning message');
    expect(visibleLines[1]).toHaveTextContent('[ERROR] Error message');
    expect(visibleLines[2]).toHaveTextContent('Plain text message');

    // Should show filter indicator
    const filterIndicator = screen.getByTestId('filter-indicator');
    expect(filterIndicator).toHaveTextContent('Showing WARN+ (3 of 5 lines)');
  });

  it('SC-4.10: supports line selection and contextual actions', () => {
    const outputLines = [
      'src/main.go:42: undefined variable',
      'src/utils.go:15: function not used',
      'tests/main_test.go:8: test failed'
    ];

    render(
      <LiveOutput
        taskId="TASK-010"
        outputLines={outputLines}
        maxLines={50}
        selectable={true}
      />
    );

    const firstLine = screen.getAllByTestId('output-line')[0];
    fireEvent.click(firstLine);

    // Should highlight selected line
    expect(firstLine).toHaveClass('bg-blue-50', 'border-l-4', 'border-blue-500');

    // Should show contextual actions for file references
    expect(screen.getByTestId('line-actions')).toBeInTheDocument();
    expect(screen.getByTestId('open-file-btn')).toHaveTextContent('Open src/main.go:42');
    expect(screen.getByTestId('copy-line-btn')).toHaveTextContent('Copy line');
  });
});