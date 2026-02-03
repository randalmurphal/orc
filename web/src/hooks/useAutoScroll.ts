import { useEffect, useRef, useState } from 'react';

export interface UseAutoScrollOptions {
  enabled?: boolean;
  threshold?: number; // Distance from bottom to consider "at bottom"
  smooth?: boolean;
}

export interface UseAutoScrollResult {
  scrollRef: React.RefObject<HTMLDivElement | null>;
  isAtBottom: boolean;
  scrollToBottom: () => void;
  enableAutoScroll: () => void;
  disableAutoScroll: () => void;
  isAutoScrollEnabled: boolean;
}

/**
 * Hook for managing auto-scroll behavior in scrollable containers
 * Automatically scrolls to bottom when new content is added, but disables
 * auto-scroll when user manually scrolls up
 */
export function useAutoScroll(options: UseAutoScrollOptions = {}): UseAutoScrollResult {
  const {
    enabled = true,
    threshold = 50,
    smooth = true
  } = options;

  const scrollRef = useRef<HTMLDivElement>(null);
  const [isAtBottom, setIsAtBottom] = useState(true);
  const [isAutoScrollEnabled, setIsAutoScrollEnabled] = useState(enabled);
  const userScrolledRef = useRef(false);

  const scrollToBottom = () => {
    const element = scrollRef.current;
    if (!element) return;

    element.scrollTo({
      top: element.scrollHeight,
      behavior: smooth ? 'smooth' : 'instant'
    });
  };

  const checkIfAtBottom = () => {
    const element = scrollRef.current;
    if (!element) return false;

    const { scrollTop, scrollHeight, clientHeight } = element;
    const distanceFromBottom = scrollHeight - (scrollTop + clientHeight);
    return distanceFromBottom <= threshold;
  };

  const handleScroll = () => {
    const element = scrollRef.current;
    if (!element) return;

    const atBottom = checkIfAtBottom();
    setIsAtBottom(atBottom);

    // If user scrolls up manually, disable auto-scroll
    if (!atBottom && !userScrolledRef.current) {
      userScrolledRef.current = true;
      setIsAutoScrollEnabled(false);
    }

    // If user scrolls back to bottom, re-enable auto-scroll
    if (atBottom && userScrolledRef.current) {
      userScrolledRef.current = false;
      setIsAutoScrollEnabled(true);
    }
  };

  // Auto-scroll when content changes and auto-scroll is enabled
  useEffect(() => {
    if (isAutoScrollEnabled && scrollRef.current) {
      const observer = new MutationObserver(() => {
        if (isAtBottom || !userScrolledRef.current) {
          scrollToBottom();
        }
      });

      observer.observe(scrollRef.current, {
        childList: true,
        subtree: true
      });

      return () => observer.disconnect();
    }
  }, [isAutoScrollEnabled, isAtBottom]);

  // Set up scroll event listener
  useEffect(() => {
    const element = scrollRef.current;
    if (!element) return;

    element.addEventListener('scroll', handleScroll, { passive: true });
    return () => element.removeEventListener('scroll', handleScroll);
  }, [threshold]);

  const enableAutoScroll = () => {
    setIsAutoScrollEnabled(true);
    userScrolledRef.current = false;
    scrollToBottom();
  };

  const disableAutoScroll = () => {
    setIsAutoScrollEnabled(false);
    userScrolledRef.current = true;
  };

  return {
    scrollRef,
    isAtBottom,
    scrollToBottom,
    enableAutoScroll,
    disableAutoScroll,
    isAutoScrollEnabled
  };
}