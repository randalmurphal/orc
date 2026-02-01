/**
 * TDD Tests for edge CSS styles
 *
 * Tests for TASK-693: Visual editor - edge drawing, deletion, and type badges
 *
 * Success Criteria Coverage:
 * - SC-1: ConditionalEdge has dotted line style in CSS
 * - SC-3: DependencyEdge has solid line style (no stroke-dasharray) in CSS
 * - SC-4: Type badge label styles exist for dependency and conditional
 *
 * These tests verify the CSS file contains correct style rules by reading
 * the raw file content. They will FAIL until edges.css is updated.
 */

import { describe, it, expect, beforeAll } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

let cssContent: string;

beforeAll(() => {
	const cssPath = resolve(__dirname, 'edges.css');
	cssContent = readFileSync(cssPath, 'utf-8');
});

describe('edges.css', () => {
	describe('SC-1: ConditionalEdge dotted line style', () => {
		it('defines .edge-conditional class', () => {
			expect(cssContent).toContain('.edge-conditional');
		});

		it('uses stroke-dasharray for conditional edges (dotted pattern)', () => {
			// Conditional edges should have a short dash pattern (dotted)
			const conditionalMatch = cssContent.match(
				/\.edge-conditional[\s\S]*?stroke-dasharray:\s*([^;]+);/
			);
			expect(conditionalMatch).not.toBeNull();
		});
	});

	describe('SC-3: DependencyEdge solid line style', () => {
		it('defines .edge-dependency class', () => {
			expect(cssContent).toContain('.edge-dependency');
		});

		it('does NOT use stroke-dasharray for dependency edges (solid line)', () => {
			// Extract the dependency edge rule block
			const depRuleMatch = cssContent.match(
				/\.edge-dependency\s+\.react-flow__edge-path\s*\{([^}]*)\}/
			);
			expect(depRuleMatch).not.toBeNull();

			// The rule should NOT contain stroke-dasharray (solid line)
			const ruleBody = depRuleMatch![1];
			expect(ruleBody).not.toContain('stroke-dasharray');
		});
	});

	describe('SC-4: Type badge label styles', () => {
		it('defines .edge-label-conditional class', () => {
			expect(cssContent).toContain('.edge-label-conditional');
		});

		it('defines .edge-label-dependency class', () => {
			expect(cssContent).toContain('.edge-label-dependency');
		});
	});
});
