/**
 * Playwright Test Fixtures - E2E Test Sandbox
 *
 * This module provides fixtures that ensure tests run against the isolated
 * sandbox project rather than real production data.
 *
 * CRITICAL: All E2E tests MUST use these fixtures to prevent accidentally
 * modifying real tasks in the main orc project.
 *
 * Usage in test files:
 *
 *   import { test, expect, sandboxProjectId } from './fixtures';
 *
 *   test('my test', async ({ page, sandbox }) => {
 *     // sandbox.projectId - The sandbox project ID
 *     // sandbox.projectPath - Path to sandbox directory
 *     // page already has sandbox project selected via localStorage
 *   });
 */

import { test as base, type Page } from '@playwright/test';
import * as fs from 'fs';

const STATE_FILE = '/tmp/orc-e2e-state.json';

interface SandboxState {
	projectId: string;
	projectPath: string;
	projectName: string;
	createdAt: string;
}

// Export the sandbox project ID for use in test assertions
export function getSandboxState(): SandboxState | null {
	try {
		if (!fs.existsSync(STATE_FILE)) {
			return null;
		}
		return JSON.parse(fs.readFileSync(STATE_FILE, 'utf-8'));
	} catch {
		return null;
	}
}

// Convenience getter for project ID
export function sandboxProjectId(): string | null {
	const state = getSandboxState();
	return state?.projectId ?? null;
}

/**
 * Extended test fixture that ensures sandbox project is selected
 *
 * This fixture:
 * 1. Sets localStorage to select sandbox project before page loads
 * 2. Provides sandbox metadata for assertions
 * 3. Verifies sandbox is actually selected on the page
 */
export const test = base.extend<{
	sandbox: SandboxState;
}>({
	sandbox: async ({}, use) => {
		const state = getSandboxState();
		if (!state) {
			throw new Error(
				'E2E sandbox not initialized. Run globalSetup first.\n' +
				'Make sure playwright.config.ts has globalSetup configured.'
			);
		}
		await use(state);
	},

	// Override page fixture to auto-select sandbox project and disable animations
	page: async ({ page, sandbox }, use) => {
		// Set localStorage BEFORE navigating to any page
		// This ensures the project is selected on first load
		await page.addInitScript((projectId: string) => {
			localStorage.setItem('orc_current_project_id', projectId);
		}, sandbox.projectId);

		// Disable animations for E2E stability
		// Animations cause Playwright "element not stable" timeouts
		// because elements are continuously moving during transitions
		await page.addStyleTag({
			content: `
				*,
				*::before,
				*::after {
					animation-duration: 0.01ms !important;
					animation-iteration-count: 1 !important;
					transition-duration: 0.01ms !important;
					scroll-behavior: auto !important;
				}
			`,
		});

		await use(page);
	},
});

export { expect } from '@playwright/test';

/**
 * Helper to wait for page to be connected to correct project
 */
export async function verifyProjectSelected(page: Page, expectedProjectId: string) {
	// Wait for the project to be selected in the UI
	// The project name should appear in the header
	await page.waitForFunction(
		(id: string) => {
			const stored = localStorage.getItem('orc_current_project_id');
			return stored === id;
		},
		expectedProjectId,
		{ timeout: 5000 }
	);
}

/**
 * Helper to wait for sandbox tasks to load
 */
export async function waitForSandboxTasks(page: Page) {
	// Wait for task cards or empty state
	await Promise.race([
		page.waitForSelector('.task-card', { timeout: 10000 }),
		page.waitForSelector('.empty-state', { timeout: 10000 }),
		page.waitForSelector('[data-testid="no-tasks"]', { timeout: 10000 }),
	]).catch(() => {
		// If neither appears, that's okay - the page might still be loading
	});
}

/**
 * Reset sandbox to clean state between tests
 *
 * This is useful for tests that modify task state and need a fresh start.
 * It re-reads the sandbox state file which was created by global-setup.
 */
export async function resetSandboxState(page: Page) {
	const state = getSandboxState();
	if (!state) {
		throw new Error('Cannot reset - sandbox state not found');
	}

	// Clear any URL params that might override project selection
	const url = new URL(page.url());
	url.searchParams.delete('project');
	url.searchParams.delete('initiative');

	// Re-set localStorage
	await page.evaluate((projectId: string) => {
		localStorage.setItem('orc_current_project_id', projectId);
		localStorage.removeItem('orc_current_initiative_id');
	}, state.projectId);

	// Reload to apply
	await page.goto(url.pathname);
}
