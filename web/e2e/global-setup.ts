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
	execSync(`${ORC_BIN} init`, { cwd: sandboxPath, stdio: 'pipe' });

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

	// Create test tasks with various statuses
	console.log('   Creating test tasks...');

	const tasksDir = path.join(sandboxPath, '.orc', 'tasks');

	// Task 1: Planned task in Queued column
	createTestTask(tasksDir, 'TASK-001', {
		id: 'TASK-001',
		title: 'E2E Test: Planned Task',
		description: 'A planned task for testing board rendering',
		status: 'planned',
		weight: 'medium',
		queue: 'active',
		priority: 'normal',
		category: 'feature',
	});

	// Task 2: Another planned task with different priority
	createTestTask(tasksDir, 'TASK-002', {
		id: 'TASK-002',
		title: 'E2E Test: High Priority Task',
		description: 'A high priority task for testing sorting',
		status: 'planned',
		weight: 'small',
		queue: 'active',
		priority: 'high',
		category: 'bug',
	});

	// Task 3: Backlog task
	createTestTask(tasksDir, 'TASK-003', {
		id: 'TASK-003',
		title: 'E2E Test: Backlog Task',
		description: 'A task in the backlog queue',
		status: 'planned',
		weight: 'large',
		queue: 'backlog',
		priority: 'low',
		category: 'refactor',
	});

	// Task 4: Completed task
	createTestTask(tasksDir, 'TASK-004', {
		id: 'TASK-004',
		title: 'E2E Test: Completed Task',
		description: 'A completed task for testing Done column',
		status: 'completed',
		weight: 'small',
		queue: 'active',
		priority: 'normal',
		category: 'feature',
	});

	// Task 5: Paused task (for testing pause/resume)
	createTestTask(tasksDir, 'TASK-005', {
		id: 'TASK-005',
		title: 'E2E Test: Paused Task',
		description: 'A paused task for testing resume functionality',
		status: 'paused',
		weight: 'medium',
		queue: 'active',
		priority: 'normal',
		category: 'feature',
		current_phase: 'implement',
	});

	// Task 6: Critical priority task
	createTestTask(tasksDir, 'TASK-006', {
		id: 'TASK-006',
		title: 'E2E Test: Critical Task',
		description: 'A critical priority task for testing priority display',
		status: 'planned',
		weight: 'medium',
		queue: 'active',
		priority: 'critical',
		category: 'bug',
	});

	// Create test initiatives
	console.log('   Creating test initiatives...');
	const initiativesDir = path.join(sandboxPath, '.orc', 'initiatives');
	fs.mkdirSync(initiativesDir, { recursive: true });

	// Initiative 1: Active with some tasks
	createTestInitiative(initiativesDir, 'INIT-001', {
		id: 'INIT-001',
		title: 'E2E Test Initiative',
		description: 'An initiative for testing filtering',
		status: 'active',
		task_ids: ['TASK-001', 'TASK-002'],
	});

	// Link tasks to initiative
	linkTaskToInitiative(tasksDir, 'TASK-001', 'INIT-001');
	linkTaskToInitiative(tasksDir, 'TASK-002', 'INIT-001');

	// Initiative 2: Another active initiative
	createTestInitiative(initiativesDir, 'INIT-002', {
		id: 'INIT-002',
		title: 'Second Test Initiative',
		description: 'Another initiative for testing swimlanes',
		status: 'active',
		task_ids: ['TASK-003'],
	});

	linkTaskToInitiative(tasksDir, 'TASK-003', 'INIT-002');

	// Commit test data
	execSync('git add . && git commit -m "Add E2E test data"', {
		cwd: sandboxPath,
		stdio: 'pipe',
	});

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

function createTestTask(
	tasksDir: string,
	taskId: string,
	task: Record<string, unknown>
) {
	const taskDir = path.join(tasksDir, taskId);
	fs.mkdirSync(taskDir, { recursive: true });

	// Add timestamps
	const now = new Date().toISOString();
	const fullTask = {
		...task,
		created_at: now,
		updated_at: now,
	};

	// Write task.yaml
	fs.writeFileSync(path.join(taskDir, 'task.yaml'), yaml.dump(fullTask));

	// Write minimal plan.yaml
	const plan = {
		version: 1,
		task_id: taskId,
		weight: task.weight || 'medium',
		description: 'Test plan',
		phases: [
			{ id: 'implement', name: 'implement', gate: { type: 'auto' }, status: 'pending' },
			{ id: 'test', name: 'test', gate: { type: 'auto' }, status: 'pending' },
		],
	};
	fs.writeFileSync(path.join(taskDir, 'plan.yaml'), yaml.dump(plan));
}

function createTestInitiative(
	initiativesDir: string,
	initId: string,
	initiative: Record<string, unknown>
) {
	const now = new Date().toISOString();
	const fullInit = {
		...initiative,
		created_at: now,
		updated_at: now,
	};

	fs.writeFileSync(
		path.join(initiativesDir, `${initId}.yaml`),
		yaml.dump(fullInit)
	);
}

function linkTaskToInitiative(
	tasksDir: string,
	taskId: string,
	initiativeId: string
) {
	const taskPath = path.join(tasksDir, taskId, 'task.yaml');
	const taskContent = fs.readFileSync(taskPath, 'utf-8');
	const task = yaml.load(taskContent) as Record<string, unknown>;
	task.initiative_id = initiativeId;
	fs.writeFileSync(taskPath, yaml.dump(task));
}
