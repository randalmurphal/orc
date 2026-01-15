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
import * as path from 'path';
import * as yaml from 'js-yaml';

const STATE_FILE = '/tmp/orc-e2e-state.json';

interface SandboxState {
	projectId: string;
	projectPath: string;
	projectName: string;
	createdAt: string;
}

interface ProjectRegistry {
	projects: Array<{ id: string; path: string; name: string }>;
	default_project?: string;
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

	// Remove from project registry
	const homeDir = process.env.HOME || '/home/runner';
	const registryPath = path.join(homeDir, '.orc', 'projects.yaml');

	if (fs.existsSync(registryPath)) {
		try {
			const registryContent = fs.readFileSync(registryPath, 'utf-8');
			const registry = yaml.load(registryContent) as ProjectRegistry;

			const originalCount = registry.projects.length;
			registry.projects = registry.projects.filter(
				(p) => p.id !== state.projectId
			);

			if (registry.projects.length < originalCount) {
				// Clear default if it was the sandbox
				if (registry.default_project === state.projectId) {
					registry.default_project = undefined;
				}

				fs.writeFileSync(registryPath, yaml.dump(registry));
				console.log('   ✓ Removed from project registry');
			} else {
				console.log('   Project not found in registry (already removed?)');
			}
		} catch (error) {
			console.log('   Failed to update registry:', error);
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
	} catch (error) {
		// Ignore - file might not exist
	}

	console.log('✅ E2E test sandbox cleaned up\n');
}
