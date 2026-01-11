<script lang="ts">
	import Column from './Column.svelte';
	import ConfirmModal from './ConfirmModal.svelte';
	import type { Task } from '$lib/types';

	interface Props {
		tasks: Task[];
		onAction: (taskId: string, action: 'run' | 'pause' | 'resume') => Promise<void>;
		onRefresh: () => Promise<void>;
	}

	let { tasks, onAction, onRefresh }: Props = $props();

	const columns = [
		{ id: 'todo', title: 'To Do', statuses: ['created', 'classifying', 'planned'] },
		{ id: 'running', title: 'In Progress', statuses: ['running'] },
		{ id: 'review', title: 'In Review', statuses: ['paused', 'blocked'] },
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

		if (columnId === 'running' && task.status !== 'running') {
			// Moving to In Progress - run or resume
			if (task.status === 'paused') {
				action = 'resume';
			} else if (['created', 'classifying', 'planned'].includes(task.status)) {
				action = 'run';
			}
		} else if (columnId === 'review' && task.status === 'running') {
			// Moving from running to review - pause
			action = 'pause';
		}

		if (action) {
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

	function cancelAction() {
		confirmModal = null;
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

<style>
	.board {
		display: flex;
		gap: var(--space-4);
		flex: 1;
		min-height: 0;
		overflow-x: auto;
		padding-bottom: var(--space-2);
	}
</style>
