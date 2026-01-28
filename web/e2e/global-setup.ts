/**
 * Playwright Global Setup - E2E Test Sandbox
 *
 * CRITICAL: E2E tests MUST run against an isolated sandbox project, NOT the
 * real orc project. Tests perform real actions (drag-drop, clicks, API calls)
 * that modify task statuses. Running against production data will corrupt
 * real task states.
 *
 * This setup creates:
 * 1. A temporary project in /tmp/orc-e2e-sandbox-{timestamp}
 * 2. Test tasks with various statuses for testing
 * 3. Test initiatives for filtering tests
 *
 * The sandbox is cleaned up by global-teardown.ts after tests complete.
 */

import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';
import * as yaml from 'js-yaml';

// ES module compatible __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// State file to pass sandbox info to tests and teardown
const STATE_FILE = '/tmp/orc-e2e-state.json';

// Orc binary path (relative to web/ directory, tests run from there)
const ORC_BIN = path.resolve(__dirname, '../../bin/orc');

interface SandboxState {
	projectId: string;
	projectPath: string;
	projectName: string;
	createdAt: string;
}

export default async function globalSetup() {
	console.log('\n🧪 Setting up E2E test sandbox...');

	// Verify orc binary exists
	if (!fs.existsSync(ORC_BIN)) {
		throw new Error(
			`Orc binary not found at ${ORC_BIN}. Run 'make build' first.`
		);
	}

	// Create unique sandbox directory
	const timestamp = Date.now();
	const sandboxPath = `/tmp/orc-e2e-sandbox-${timestamp}`;
	const sandboxName = `e2e-sandbox-${timestamp}`;

	console.log(`   Creating sandbox at ${sandboxPath}`);
	fs.mkdirSync(sandboxPath, { recursive: true });

	// Initialize as git repo (orc requires git)
	execSync('git init', { cwd: sandboxPath, stdio: 'pipe' });
	execSync('git config user.email "test@example.com"', {
		cwd: sandboxPath,
		stdio: 'pipe',
	});
	execSync('git config user.name "E2E Test"', {
		cwd: sandboxPath,
		stdio: 'pipe',
	});

	// Create a dummy file and initial commit (orc needs a commit)
	fs.writeFileSync(path.join(sandboxPath, 'README.md'), '# E2E Test Sandbox\n');
	execSync('git add . && git commit -m "Initial commit"', {
		cwd: sandboxPath,
		stdio: 'pipe',
	});

	// Initialize orc in sandbox
	console.log('   Initializing orc...');
	execSync(`${ORC_BIN} init --yes`, { cwd: sandboxPath, stdio: 'pipe' });

	// Get the project ID from registry
	const homeDir = process.env.HOME || '/home/runner';
	const registryPath = path.join(homeDir, '.orc', 'projects.yaml');
	const registryContent = fs.readFileSync(registryPath, 'utf-8');
	const registry = yaml.load(registryContent) as {
		projects: Array<{ id: string; path: string; name: string }>;
	};

	const project = registry.projects.find((p) => p.path === sandboxPath);
	if (!project) {
		throw new Error('Failed to find sandbox project in registry');
	}

	console.log(`   Project ID: ${project.id}`);

	// Create test tasks using orc CLI (data goes to SQLite, not YAML files)
	console.log('   Creating test tasks...');

	// Helper to run orc commands in sandbox
	const runOrc = (cmd: string) => {
		execSync(`${ORC_BIN} ${cmd}`, { cwd: sandboxPath, stdio: 'pipe' });
	};

	// Task 1: Planned task (normal priority, feature)
	runOrc('new "E2E Test: Planned Task" -d "A planned task for testing board rendering" -w medium -p normal -c feature');

	// Task 2: High priority bug
	runOrc('new "E2E Test: High Priority Task" -d "A high priority task for testing sorting" -w small -p high -c bug');

	// Task 3: Low priority refactor task
	runOrc('new "E2E Test: Refactor Task" -d "A refactoring task for testing different categories" -w large -p low -c refactor');

	// Task 4: Task to mark as completed
	runOrc('new "E2E Test: Completed Task" -d "A completed task for testing Done column" -w small -p normal -c feature');
	// Mark task 4 as completed
	runOrc('edit TASK-004 --status completed');

	// Task 5: Task to mark as paused
	runOrc('new "E2E Test: Paused Task" -d "A paused task for testing resume functionality" -w medium -p normal -c feature');
	// Mark task 5 as paused
	runOrc('edit TASK-005 --status paused');

	// Task 6: Critical priority bug
	runOrc('new "E2E Test: Critical Task" -d "A critical priority task for testing priority display" -w medium -p critical -c bug');

	// Create test initiatives using orc CLI
	console.log('   Creating test initiatives...');

	// Initiative 1: Active with some tasks
	runOrc('initiative new "E2E Test Initiative" --vision "An initiative for testing filtering"');

	// Initiative 2: Another active initiative
	runOrc('initiative new "Second Test Initiative" --vision "Another initiative for testing swimlanes"');

	// Link tasks to initiatives
	runOrc('edit TASK-001 --initiative INIT-001');
	runOrc('edit TASK-002 --initiative INIT-001');
	runOrc('edit TASK-003 --initiative INIT-002');

	// Save state for tests and teardown
	const state: SandboxState = {
		projectId: project.id,
		projectPath: sandboxPath,
		projectName: sandboxName,
		createdAt: new Date().toISOString(),
	};

	fs.writeFileSync(STATE_FILE, JSON.stringify(state, null, 2));
	console.log(`   State saved to ${STATE_FILE}`);

	// Set environment variable for tests
	process.env.ORC_E2E_PROJECT_ID = project.id;
	process.env.ORC_E2E_PROJECT_PATH = sandboxPath;

	console.log('✅ E2E test sandbox ready\n');
}

// Note: Tasks and initiatives are created via orc CLI which stores data in SQLite.
// The YAML helper functions were removed as they are no longer needed.
