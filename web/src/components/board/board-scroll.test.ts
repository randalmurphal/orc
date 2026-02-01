/**
 * TDD tests for board scroll overflow fix.
 *
 * Root cause: CSS flex/grid children default to `min-height: auto`, which
 * resolves to `min-content` when `overflow: visible`. This prevents nested
 * elements with `overflow-y: auto` from activating their scrollbars because
 * the intermediate containers refuse to shrink below their content size.
 *
 * The fix requires `min-height: 0` on three elements in the layout chain:
 *   1. `.board-view`      (CSS grid container in BoardView.css)
 *   2. `.queue-column`    (flex child inside `.board-view__queue` in QueueColumn.css)
 *   3. `.running-column`  (flex child inside `.board-view__running` in RunningColumn.css)
 *
 * jsdom does not compute CSS layouts, so these tests read the raw CSS source
 * files and verify the required properties exist in the correct rule blocks.
 */

import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const __dirname = dirname(fileURLToPath(import.meta.url));

/**
 * Extract the CSS declaration block for an exact selector.
 *
 * Uses a negative lookahead for word characters, underscores, and hyphens
 * so that `.board-view` does NOT match `.board-view__queue` or
 * `.board-view--loading`.
 *
 * Returns the full matched block (selector + braces + declarations),
 * or an empty string if no match is found.
 */
function extractCSSBlock(css: string, selector: string): string {
  // Escape CSS special chars for regex (the dot in class selectors)
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

  // Match the exact selector followed by a block, but NOT followed by
  // additional word chars, underscores, or hyphens (BEM suffixes).
  const pattern = new RegExp(
    escaped + '(?![\\w_-])' + '\\s*\\{[^}]*\\}',
  );

  const match = css.match(pattern);
  return match ? match[0] : '';
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function readCSS(filename: string): string {
  return readFileSync(resolve(__dirname, filename), 'utf-8');
}

function hasMinHeightZero(block: string): boolean {
  // Match `min-height` followed by `:`, optional whitespace, then `0`
  // (with optional unit like `0px`), then `;` or `}`.
  return /min-height\s*:\s*0(px)?\s*[;}]/.test(block);
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('Board scroll overflow fix', () => {
  describe('extractCSSBlock helper', () => {
    it('extracts an exact selector block', () => {
      const css = `.foo { color: red; }\n.foo-bar { color: blue; }`;
      const block = extractCSSBlock(css, '.foo');
      expect(block).toContain('color: red');
      expect(block).not.toContain('color: blue');
    });

    it('does not match BEM-extended selectors', () => {
      const css = `.board-view__queue { display: flex; }\n.board-view { display: grid; }`;
      const block = extractCSSBlock(css, '.board-view');
      expect(block).toContain('display: grid');
      expect(block).not.toContain('display: flex');
    });

    it('returns empty string when selector is not found', () => {
      const css = `.other { margin: 0; }`;
      expect(extractCSSBlock(css, '.missing')).toBe('');
    });
  });

  describe('.board-view (BoardView.css)', () => {
    const css = readCSS('BoardView.css');
    const block = extractCSSBlock(css, '.board-view');

    it('finds the .board-view rule block', () => {
      expect(block).not.toBe('');
    });

    it('contains min-height: 0 to allow grid children to shrink', () => {
      expect(hasMinHeightZero(block)).toBe(true);
    });
  });

  describe('.queue-column (QueueColumn.css)', () => {
    const css = readCSS('QueueColumn.css');
    const block = extractCSSBlock(css, '.queue-column');

    it('finds the .queue-column rule block', () => {
      expect(block).not.toBe('');
    });

    it('contains min-height: 0 to allow flex children to shrink', () => {
      expect(hasMinHeightZero(block)).toBe(true);
    });
  });

  describe('.running-column (RunningColumn.css)', () => {
    const css = readCSS('RunningColumn.css');
    const block = extractCSSBlock(css, '.running-column');

    it('finds the .running-column rule block', () => {
      expect(block).not.toBe('');
    });

    it('contains min-height: 0 to allow flex children to shrink', () => {
      expect(hasMinHeightZero(block)).toBe(true);
    });
  });
});
