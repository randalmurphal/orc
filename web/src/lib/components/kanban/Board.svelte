<script lang="ts">
	import Column from './Column.svelte';
	import QueuedColumn from './QueuedColumn.svelte';
	import Swimlane from './Swimlane.svelte';
	import ConfirmModal from './ConfirmModal.svelte';
	import type { Task, TaskPriority, TaskQueue, Initiative } from '$lib/types';
	import { PRIORITY_ORDER } from '$lib/types';
	import { updateTask } from '$lib/api';
	import { updateTask as updateTaskInStore } from '$lib/stores/tasks';

	export type BoardViewMode = 'flat' | 'swimlane';

	interface Props {
		tasks: Task[];
		viewMode?: BoardViewMode;
		initiatives?: Initiative[];
		onAction: (taskId: string, action: 'run' | 'pause' | 'resume') => Promise<void>;
		onEscalate?: (taskId: string, reason: string) => Promise<void>;
		onRefresh?: () => Promise<void>;
		onTaskClick?: (task: Task) => void;
		onFinalizeClick?: (task: Task) => void;
	}

	let {
		tasks,
		viewMode = 'flat',
		initiatives = [],
		onAction,
		onEscalate,
		onRefresh,
		onTaskClick,
		onFinalizeClick
	}: Props = $props();

	// Escalation modal state
	let showEscalateModal = $state(false);
	let escalateTask = $state<Task | null>(null);
	let escalateReason = $state('');

	// Backlog visibility state (persisted in localStorage)
	let showBacklog = $state(false);

	// Collapsed swimlanes state (persisted in localStorage)
	let collapsedSwimlanes = $state<Set<string>>(new Set());

	// Initiative change confirmation modal
	let initiativeChangeModal = $state<{
		task: Task;
		targetInitiativeId: string | null;
		columnId: string;
	} | null>(null);

	// Initialize showBacklog from localStorage
	$effect(() => {
		if (typeof window !== 'undefined' && typeof localStorage !== 'undefined') {
			try {
				const stored = localStorage.getItem('orc-show-backlog');
				if (stored !== null) {
					showBacklog = stored === 'true';
				}
			} catch {
				// localStorage may not be available in some environments (e.g., tests)
			}
		}
	});

	// Initialize collapsed swimlanes from localStorage
	$effect(() => {
		if (typeof window !== 'undefined' && typeof localStorage !== 'undefined') {
			try {
				const stored = localStorage.getItem('orc-collapsed-swimlanes');
				if (stored !== null) {
					collapsedSwimlanes = new Set(JSON.parse(stored));
				}
			} catch {
				// localStorage may not be available in some environments (e.g., tests)
			}
		}
	});

	// Persist showBacklog to localStorage
	function toggleBacklog() {
		showBacklog = !showBacklog;
		if (typeof window !== 'undefined' && typeof localStorage !== 'undefined') {
			try {
				localStorage.setItem('orc-show-backlog', String(showBacklog));
			} catch {
				// localStorage may not be available in some environments (e.g., tests)
			}
		}
	}

	// Toggle swimlane collapse
	function toggleSwimlane(id: string) {
		const newCollapsed = new Set(collapsedSwimlanes);
		if (newCollapsed.has(id)) {
			newCollapsed.delete(id);
		} else {
			newCollapsed.add(id);
		}
		collapsedSwimlanes = newCollapsed;

		if (typeof window !== 'undefined' && typeof localStorage !== 'undefined') {
			try {
				localStorage.setItem(
					'orc-collapsed-swimlanes',
					JSON.stringify([...newCollapsed])
				);
			} catch {
				// localStorage may not be available
			}
		}
	}

	// Phase-based columns matching orchestration workflow
	const columns = [
		{ id: 'queued', title: 'Queued', phases: [] as string[] }, // No phase yet
		{ id: 'spec', title: 'Spec', phases: ['research', 'spec', 'design'] },
		{ id: 'implement', title: 'Implement', phases: ['implement'] },
		{ id: 'test', title: 'Test', phases: ['test'] },
		{ id: 'review', title: 'Review', phases: ['docs', 'validate', 'review'] },
		{ id: 'done', title: 'Done', phases: [] as string[] } // Terminal statuses
	];

	let confirmModal = $state<{ task: Task; action: string; targetColumn: string } | null>(null);
	let actionLoading = $state(false);

	// Sort tasks: running tasks first, then by priority (critical first, then high, normal, low)
	function sortTasks(taskList: Task[]): Task[] {
		return [...taskList].sort((a, b) => {
			// Running tasks always come first
			const aRunning = a.status === 'running' ? 0 : 1;
			const bRunning = b.status === 'running' ? 0 : 1;
			if (aRunning !== bRunning) {
				return aRunning - bRunning;
			}
			// Within same running status, sort by priority
			const priorityA = (a.priority || 'normal') as TaskPriority;
			const priorityB = (b.priority || 'normal') as TaskPriority;
			return PRIORITY_ORDER[priorityA] - PRIORITY_ORDER[priorityB];
		});
	}

	// Determine which column a task belongs to based on phase and status
	function getTaskColumn(task: Task): string {
		// Terminal/done statuses always go to Done
		if (task.status === 'finalizing' || task.status === 'completed' || task.status === 'finished' || task.status === 'failed') {
			return 'done';
		}

		// Tasks not yet started go to Queued
		// Note: "running" tasks without a phase are transitional - they're about to
		// start their first phase, so show them in "implement" instead of "queued"
		if (!task.current_phase) {
			if (task.status === 'running') {
				return 'implement';
			}
			return 'queued';
		}
		if (['created', 'classifying', 'planned'].includes(task.status)) {
			return 'queued';
		}

		// Running, paused, or blocked tasks go to their current phase column
		for (const col of columns) {
			if (col.phases.includes(task.current_phase)) {
				return col.id;
			}
		}

		// Default to implement if phase not recognized
		return 'implement';
	}

	// Get queue for a task (default to active for backward compatibility)
	function getTaskQueue(task: Task): TaskQueue {
		return (task.queue || 'active') as TaskQueue;
	}

	// Group tasks by column, sorted by priority
	const tasksByColumn = $derived.by(() => {
		const grouped: Record<string, Task[]> = {};
		for (const col of columns) {
			grouped[col.id] = [];
		}
		for (const task of tasks) {
			const colId = getTaskColumn(task);
			grouped[colId].push(task);
		}
		// Sort each column: running tasks first, then by priority
		for (const colId of Object.keys(grouped)) {
			grouped[colId] = sortTasks(grouped[colId]);
		}
		return grouped;
	});

	// Separate queued tasks into active and backlog
	const queuedActiveTasks = $derived(
		tasksByColumn['queued'].filter(t => getTaskQueue(t) === 'active')
	);
	const queuedBacklogTasks = $derived(
		tasksByColumn['queued'].filter(t => getTaskQueue(t) === 'backlog')
	);

	// Count of backlog tasks for the toggle button
	const backlogCount = $derived(queuedBacklogTasks.length);

	// Group tasks by initiative for swimlane view
	const tasksByInitiative = $derived.by(() => {
		const grouped: Map<string | null, Task[]> = new Map();

		// Initialize with all initiatives (even empty ones)
		for (const init of initiatives) {
			grouped.set(init.id, []);
		}
		// Add null for unassigned
		grouped.set(null, []);

		// Group tasks
		for (const task of tasks) {
			const initId = task.initiative_id ?? null;
			const existing = grouped.get(initId) || [];
			existing.push(task);
			grouped.set(initId, existing);
		}

		return grouped;
	});

	// Get tasks by column for a specific initiative
	function getTasksByColumnForInitiative(initiativeTasks: Task[]): Record<string, Task[]> {
		const grouped: Record<string, Task[]> = {};
		for (const col of columns) {
			grouped[col.id] = [];
		}
		for (const task of initiativeTasks) {
			const colId = getTaskColumn(task);
			grouped[colId].push(task);
		}
		// Sort each column
		for (const colId of Object.keys(grouped)) {
			grouped[colId] = sortTasks(grouped[colId]);
		}
		return grouped;
	}

	// Swimlane data: ordered list of initiatives with their tasks
	const swimlaneData = $derived.by(() => {
		const result: Array<{
			initiative: Initiative | null;
			tasks: Task[];
			tasksByColumn: Record<string, Task[]>;
			collapsed: boolean;
		}> = [];

		// Active initiatives first (sorted by title)
		const activeInits = initiatives
			.filter(i => i.status === 'active')
			.sort((a, b) => a.title.localeCompare(b.title));

		for (const init of activeInits) {
			const initTasks = tasksByInitiative.get(init.id) || [];
			if (initTasks.length > 0) {
				result.push({
					initiative: init,
					tasks: initTasks,
					tasksByColumn: getTasksByColumnForInitiative(initTasks),
					collapsed: collapsedSwimlanes.has(init.id)
				});
			}
		}

		// Other initiatives (draft, completed) that have tasks
		const otherInits = initiatives
			.filter(i => i.status !== 'active')
			.sort((a, b) => a.title.localeCompare(b.title));

		for (const init of otherInits) {
			const initTasks = tasksByInitiative.get(init.id) || [];
			if (initTasks.length > 0) {
				result.push({
					initiative: init,
					tasks: initTasks,
					tasksByColumn: getTasksByColumnForInitiative(initTasks),
					collapsed: collapsedSwimlanes.has(init.id)
				});
			}
		}

		// Unassigned tasks at the bottom
		const unassignedTasks = tasksByInitiative.get(null) || [];
		if (unassignedTasks.length > 0) {
			result.push({
				initiative: null,
				tasks: unassignedTasks,
				tasksByColumn: getTasksByColumnForInitiative(unassignedTasks),
				collapsed: collapsedSwimlanes.has('__unassigned__')
			});
		}

		return result;
	});

	function getSourceColumn(task: Task): string {
		return getTaskColumn(task);
	}

	function handleDrop(columnId: string, task: Task) {
		const sourceColumnId = getSourceColumn(task);

		// Don't show modal if dropping in the same column
		if (sourceColumnId === columnId) {
			return;
		}

		const column = columns.find((c) => c.id === columnId);
		if (!column) return;

		// Determine action based on current status and target column
		let action: string | null = null;

		// Moving from Queued to any phase column - start the task
		if (sourceColumnId === 'queued' && columnId !== 'done') {
			action = 'run';
		}
		// Moving a paused/blocked task to any phase column - resume
		else if ((task.status === 'paused' || task.status === 'blocked') && columnId !== 'done' && columnId !== 'queued') {
			action = 'resume';
		}
		// Moving a running task to Queued - pause and escalate
		else if (task.status === 'running' && columnId === 'queued') {
			action = 'escalate';
		}
		// Moving a running task backward (e.g., from Test to Implement) - escalate
		else if (task.status === 'running' && getColumnIndex(columnId) < getColumnIndex(sourceColumnId)) {
			action = 'escalate';
		}

		if (action === 'escalate') {
			// For escalation, show a special modal to capture the reason
			escalateTask = task;
			escalateReason = '';
			showEscalateModal = true;
		} else if (action) {
			confirmModal = { task, action, targetColumn: column.title };
		}
	}

	// Handle drop in swimlane view (includes initiative change detection)
	function handleSwimlaneDrop(
		columnId: string,
		task: Task,
		targetInitiativeId: string | null
	) {
		const sourceInitiativeId = task.initiative_id ?? null;

		// Check if initiative is changing
		if (sourceInitiativeId !== targetInitiativeId) {
			// Show confirmation for initiative change
			initiativeChangeModal = { task, targetInitiativeId, columnId };
			return;
		}

		// Otherwise handle as normal column drop
		handleDrop(columnId, task);
	}

	async function confirmInitiativeChange() {
		if (!initiativeChangeModal) return;

		const { task, targetInitiativeId, columnId } = initiativeChangeModal;

		actionLoading = true;
		try {
			// Update task's initiative
			const updated = await updateTask(task.id, {
				initiative_id: targetInitiativeId ?? ''
			});
			updateTaskInStore(task.id, updated);

			// Now handle the column change if any
			initiativeChangeModal = null;
			handleDrop(columnId, { ...task, initiative_id: targetInitiativeId ?? undefined });
		} catch (e) {
			console.error('Failed to change initiative:', e);
		} finally {
			actionLoading = false;
			initiativeChangeModal = null;
		}
	}

	function cancelInitiativeChange() {
		initiativeChangeModal = null;
	}

	function getColumnIndex(columnId: string): number {
		return columns.findIndex((c) => c.id === columnId);
	}

	async function confirmAction() {
		if (!confirmModal) return;

		actionLoading = true;
		try {
			const action = confirmModal.action as 'run' | 'pause' | 'resume';
			await onAction(confirmModal.task.id, action);
		} finally {
			actionLoading = false;
			confirmModal = null;
		}
	}

	async function confirmEscalate() {
		if (!escalateTask || !onEscalate) return;

		actionLoading = true;
		try {
			await onEscalate(escalateTask.id, escalateReason);
			// onRefresh is optional - WebSocket events will update the store
			if (onRefresh) await onRefresh();
		} finally {
			actionLoading = false;
			showEscalateModal = false;
			escalateTask = null;
			escalateReason = '';
		}
	}

	function cancelAction() {
		confirmModal = null;
	}

	function cancelEscalate() {
		showEscalateModal = false;
		escalateTask = null;
		escalateReason = '';
	}
</script>

{#if viewMode === 'flat'}
	<div class="board">
		{#each columns as column (column.id)}
			{#if column.id === 'queued'}
				<QueuedColumn
					{column}
					activeTasks={queuedActiveTasks}
					backlogTasks={queuedBacklogTasks}
					{showBacklog}
					onToggleBacklog={toggleBacklog}
					onDrop={(task) => handleDrop(column.id, task)}
					{onAction}
					{onTaskClick}
					{onFinalizeClick}
				/>
			{:else}
				<Column
					{column}
					tasks={tasksByColumn[column.id] || []}
					onDrop={(task) => handleDrop(column.id, task)}
					{onAction}
					{onTaskClick}
					{onFinalizeClick}
				/>
			{/if}
		{/each}
	</div>
{:else}
	<!-- Swimlane View -->
	<div class="swimlane-view">
		<!-- Column headers -->
		<div class="swimlane-headers">
			<div class="header-spacer"></div>
			{#each columns as column (column.id)}
				<div class="column-header">
					<span class="header-title">{column.title}</span>
				</div>
			{/each}
		</div>

		<!-- Swimlanes -->
		<div class="swimlanes">
			{#each swimlaneData as lane (lane.initiative?.id ?? '__unassigned__')}
				<Swimlane
					initiative={lane.initiative}
					tasks={lane.tasks}
					{columns}
					tasksByColumn={lane.tasksByColumn}
					collapsed={lane.collapsed}
					onToggleCollapse={() => toggleSwimlane(lane.initiative?.id ?? '__unassigned__')}
					onDrop={handleSwimlaneDrop}
					{onAction}
					{onTaskClick}
					{onFinalizeClick}
				/>
			{/each}

			{#if swimlaneData.length === 0}
				<div class="empty-swimlanes">
					<p>No tasks to display</p>
				</div>
			{/if}
		</div>
	</div>
{/if}

{#if confirmModal}
	<ConfirmModal
		title="{confirmModal.action === 'run' ? 'Run' : confirmModal.action === 'pause' ? 'Pause' : 'Resume'} Task?"
		message="Move '{confirmModal.task.title}' to {confirmModal.targetColumn}?"
		confirmLabel={confirmModal.action === 'run' ? 'Run Task' : confirmModal.action === 'pause' ? 'Pause Task' : 'Resume Task'}
		confirmVariant={confirmModal.action === 'pause' ? 'warning' : 'primary'}
		action={confirmModal.action as 'run' | 'pause' | 'resume'}
		loading={actionLoading}
		onConfirm={confirmAction}
		onCancel={cancelAction}
	/>
{/if}

{#if showEscalateModal && escalateTask}
	<div class="modal-backdrop" onclick={cancelEscalate} onkeydown={(e) => e.key === 'Escape' && cancelEscalate()} role="presentation">
		<div class="escalate-modal" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()} role="dialog" aria-labelledby="escalate-title" tabindex="-1">
			<div class="modal-header">
				<h3 id="escalate-title">Escalate Task</h3>
				<button class="close-btn" onclick={cancelEscalate} aria-label="Close">
					<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<line x1="18" y1="6" x2="6" y2="18" />
						<line x1="6" y1="6" x2="18" y2="18" />
					</svg>
				</button>
			</div>
			<div class="modal-body">
				<p class="task-info">
					<strong>{escalateTask.id}</strong>: {escalateTask.title}
				</p>
				<label class="reason-label">
					<span>Reason for escalation</span>
					<textarea
						bind:value={escalateReason}
						placeholder="Describe what needs to be fixed or changed. This context will be passed to the AI agent..."
						rows="4"
					></textarea>
				</label>
				<p class="hint">
					The task will be paused and moved back to Queued. When re-run, this context will be injected into the phase prompt.
				</p>
			</div>
			<div class="modal-footer">
				<button class="btn-secondary" onclick={cancelEscalate}>Cancel</button>
				<button
					class="btn-warning"
					onclick={confirmEscalate}
					disabled={!escalateReason.trim() || actionLoading}
				>
					{#if actionLoading}
						Escalating...
					{:else}
						Escalate & Re-run
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}

{#if initiativeChangeModal}
	<div class="modal-backdrop" onclick={cancelInitiativeChange} onkeydown={(e) => e.key === 'Escape' && cancelInitiativeChange()} role="presentation">
		<div class="initiative-change-modal" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()} role="dialog" aria-labelledby="init-change-title" tabindex="-1">
			<div class="modal-header">
				<h3 id="init-change-title">Change Initiative?</h3>
				<button class="close-btn" onclick={cancelInitiativeChange} aria-label="Close">
					<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<line x1="18" y1="6" x2="6" y2="18" />
						<line x1="6" y1="6" x2="18" y2="18" />
					</svg>
				</button>
			</div>
			<div class="modal-body">
				<p class="task-info">
					<strong>{initiativeChangeModal.task.id}</strong>: {initiativeChangeModal.task.title}
				</p>
				<p class="change-description">
					{#if initiativeChangeModal.targetInitiativeId}
						{@const targetInit = initiatives.find(i => i.id === initiativeChangeModal?.targetInitiativeId)}
						Move to initiative: <strong>{targetInit?.title ?? 'Unknown'}</strong>
					{:else}
						Remove from current initiative (unassigned)
					{/if}
				</p>
			</div>
			<div class="modal-footer">
				<button class="btn-secondary" onclick={cancelInitiativeChange}>Cancel</button>
				<button
					class="btn-primary"
					onclick={confirmInitiativeChange}
					disabled={actionLoading}
				>
					{#if actionLoading}
						Moving...
					{:else}
						Confirm Move
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.board {
		display: flex;
		gap: var(--space-3);
		flex: 1;
		min-height: 0;
		overflow-x: auto;
		padding-bottom: var(--space-2);
		/* Let columns fill available space, scroll if needed */
	}

	/* Ensure board doesn't overflow container */
	@media (min-width: 1400px) {
		.board {
			/* On larger screens, columns can be slightly wider */
			gap: var(--space-4);
		}
	}

	/* Swimlane View */
	.swimlane-view {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-height: 0;
		overflow: auto;
	}

	.swimlane-headers {
		display: flex;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-4);
		background: var(--bg-secondary);
		border-bottom: 1px solid var(--border-subtle);
		position: sticky;
		top: 0;
		z-index: 10;
	}

	.header-spacer {
		width: 200px;
		flex-shrink: 0;
	}

	.column-header {
		flex: 1;
		min-width: 150px;
		max-width: 280px;
		padding: var(--space-2);
		text-align: center;
	}

	.header-title {
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wide);
	}

	.swimlanes {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		padding: var(--space-3);
		flex: 1;
	}

	.empty-swimlanes {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: var(--space-12);
		color: var(--text-muted);
		font-size: var(--text-sm);
	}

	/* Escalation Modal */
	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.6);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
		backdrop-filter: blur(4px);
	}

	.escalate-modal,
	.initiative-change-modal {
		background: var(--bg-primary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-xl);
		width: 100%;
		max-width: 500px;
		box-shadow: var(--shadow-xl);
	}

	.modal-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-4) var(--space-5);
		border-bottom: 1px solid var(--border-subtle);
	}

	.modal-header h3 {
		margin: 0;
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
	}

	.close-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		color: var(--text-muted);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.close-btn:hover {
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}

	.modal-body {
		padding: var(--space-5);
	}

	.task-info {
		margin: 0 0 var(--space-4);
		padding: var(--space-3);
		background: var(--bg-secondary);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		color: var(--text-secondary);
	}

	.task-info strong {
		color: var(--text-primary);
		font-family: var(--font-mono);
	}

	.change-description {
		margin: 0;
		font-size: var(--text-sm);
		color: var(--text-secondary);
	}

	.change-description strong {
		color: var(--accent-primary);
	}

	.reason-label {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.reason-label span {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
	}

	.reason-label textarea {
		width: 100%;
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		color: var(--text-primary);
		resize: vertical;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.reason-label textarea:focus {
		outline: none;
		border-color: var(--status-warning);
		box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.2);
	}

	.reason-label textarea::placeholder {
		color: var(--text-muted);
	}

	.hint {
		margin: var(--space-3) 0 0;
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.modal-footer {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-3);
		padding: var(--space-4) var(--space-5);
		border-top: 1px solid var(--border-subtle);
	}

	.btn-secondary {
		padding: var(--space-2) var(--space-4);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.btn-secondary:hover {
		background: var(--bg-tertiary);
		border-color: var(--border-strong);
	}

	.btn-warning {
		padding: var(--space-2) var(--space-4);
		background: var(--status-warning);
		border: none;
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--bg-primary);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.btn-warning:hover:not(:disabled) {
		filter: brightness(1.1);
	}

	.btn-warning:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.btn-primary {
		padding: var(--space-2) var(--space-4);
		background: var(--accent-primary);
		border: none;
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-inverse);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.btn-primary:hover:not(:disabled) {
		background: var(--accent-hover);
	}

	.btn-primary:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
</style>
