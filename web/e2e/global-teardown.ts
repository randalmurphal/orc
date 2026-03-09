/**
 * Playwright Global Teardown - E2E Test Sandbox Cleanup
 *
 * Cleans up the test sandbox created by global-setup.ts:
 * 1. Removes the project from ~/.orc/projects.yaml
 * 2. Deletes the temporary sandbox directory
 *
 * This ensures no test data pollutes the global project registry
 * and no temporary files are left behind.
 */

import * as fs from 'fs';
import { execSync } from 'child_process';
import * as path from 'path';
import { fileURLToPath } from 'url';

const STATE_FILE = '/tmp/orc-e2e-state.json';
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const ORC_BIN = path.resolve(__dirname, '../../bin/orc');

interface SandboxState {
	projectId: string;
	projectPath: string;
	projectName: string;
	createdAt: string;
}

export default async function globalTeardown() {
	console.log('\n🧹 Cleaning up E2E test sandbox...');

	// Load state
	if (!fs.existsSync(STATE_FILE)) {
		console.log('   No state file found, nothing to clean up');
		return;
	}

	let state: SandboxState;
	try {
		state = JSON.parse(fs.readFileSync(STATE_FILE, 'utf-8'));
	} catch (error) {
		console.log('   Failed to parse state file:', error);
		return;
	}

	console.log(`   Project ID: ${state.projectId}`);
	console.log(`   Project Path: ${state.projectPath}`);

	if (fs.existsSync(ORC_BIN)) {
		try {
			execSync(`${ORC_BIN} projects remove ${state.projectPath}`, {
				stdio: 'pipe',
			});
			console.log('   ✓ Removed from project registry');
		} catch (error) {
			console.log('   Failed to remove project from registry:', error);
		}
	}

	// Remove sandbox directory
	if (fs.existsSync(state.projectPath)) {
		try {
			fs.rmSync(state.projectPath, { recursive: true, force: true });
			console.log('   ✓ Removed sandbox directory');
		} catch (error) {
			console.log('   Failed to remove sandbox directory:', error);
		}
	} else {
		console.log('   Sandbox directory not found (already removed?)');
	}

	// Remove state file
	try {
		fs.unlinkSync(STATE_FILE);
		console.log('   ✓ Removed state file');
	} catch (_error) {
		// Ignore - file might not exist
	}

	console.log('✅ E2E test sandbox cleaned up\n');
}
