<script lang="ts">
	import Column from './Column.svelte';
	import ConfirmModal from './ConfirmModal.svelte';
	import type { Task } from '$lib/types';

	interface Props {
		tasks: Task[];
		onAction: (taskId: string, action: 'run' | 'pause' | 'resume') => Promise<void>;
		onEscalate?: (taskId: string, reason: string) => Promise<void>;
		onRefresh: () => Promise<void>;
	}

	let { tasks, onAction, onEscalate, onRefresh }: Props = $props();

	// Escalation modal state
	let showEscalateModal = $state(false);
	let escalateTask = $state<Task | null>(null);
	let escalateReason = $state('');

	const columns = [
		{ id: 'todo', title: 'To Do', statuses: ['created', 'classifying', 'planned'] },
		{ id: 'running', title: 'In Progress', statuses: ['running'] },
		{ id: 'review', title: 'Review', statuses: ['paused'] },
		{ id: 'qa', title: 'QA', statuses: ['blocked'] },
		{ id: 'done', title: 'Done', statuses: ['completed', 'failed'] }
	];

	let confirmModal = $state<{ task: Task; action: string; targetColumn: string } | null>(null);
	let actionLoading = $state(false);

	// Group tasks by column
	const tasksByColumn = $derived.by(() => {
		const grouped: Record<string, Task[]> = {};
		for (const col of columns) {
			grouped[col.id] = tasks.filter((t) => col.statuses.includes(t.status));
		}
		return grouped;
	});

	function getSourceColumn(task: Task): string {
		for (const col of columns) {
			if (col.statuses.includes(task.status)) {
				return col.id;
			}
		}
		return 'todo';
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

		if (columnId === 'running') {
			// Moving to In Progress - run or resume
			if (task.status === 'paused' || task.status === 'blocked') {
				action = 'resume';
			} else if (['created', 'classifying', 'planned'].includes(task.status)) {
				action = 'run';
			}
		} else if (columnId === 'review' && task.status === 'running') {
			// Moving from running to review - pause
			action = 'pause';
		} else if (columnId === 'qa' && task.status === 'running') {
			// Moving from running to QA - pause (will be marked as blocked by QA process)
			action = 'pause';
		} else if (columnId === 'todo' && (sourceColumnId === 'review' || sourceColumnId === 'qa')) {
			// Escalating from Review/QA back to To Do for re-implementation
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
			await onRefresh();
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

<div class="board">
	{#each columns as column (column.id)}
		<Column
			{column}
			tasks={tasksByColumn[column.id] || []}
			onDrop={(task) => handleDrop(column.id, task)}
			{onAction}
		/>
	{/each}
</div>

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
				<h3 id="escalate-title">Escalate to Implementation</h3>
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
					The task will be moved back to "To Do" and re-run with this context injected into the implementation phase.
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

<style>
	.board {
		display: flex;
		gap: var(--space-4);
		flex: 1;
		min-height: 0;
		overflow-x: auto;
		padding: 0 var(--space-4) var(--space-2);
		justify-content: center;
		max-width: 100%;
	}

	/* When there's not enough space, allow scrolling and left-align */
	@media (max-width: 1700px) {
		.board {
			justify-content: flex-start;
		}
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

	.escalate-modal {
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
</style>
