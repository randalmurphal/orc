<!--
	NewTaskModal - Global modal for creating new tasks

	Lives in +layout.svelte so it can be triggered from any page via:
	  window.dispatchEvent(new CustomEvent('orc:new-task'))

	Also triggered by Cmd+N keyboard shortcut.
-->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { createTask, createProjectTask } from '$lib/api';
	import { currentProjectId } from '$lib/stores/project';
	import { addTask } from '$lib/stores/tasks';
	import { toast } from '$lib/stores/toast.svelte';
	import type { TaskCategory } from '$lib/types';
	import { CATEGORY_CONFIG } from '$lib/types';
	import Modal from './Modal.svelte';

	interface Props {
		open: boolean;
		onClose: () => void;
	}

	let { open, onClose }: Props = $props();

	let title = $state('');
	let description = $state('');
	let category = $state<TaskCategory>('feature');
	let creating = $state(false);
	let error = $state<string | null>(null);
	let titleInputRef: HTMLInputElement;

	const categoryOptions: { value: TaskCategory; label: string; icon: string; color: string }[] = [
		{ value: 'feature', label: CATEGORY_CONFIG.feature.label, icon: CATEGORY_CONFIG.feature.icon, color: CATEGORY_CONFIG.feature.color },
		{ value: 'bug', label: CATEGORY_CONFIG.bug.label, icon: CATEGORY_CONFIG.bug.icon, color: CATEGORY_CONFIG.bug.color },
		{ value: 'refactor', label: CATEGORY_CONFIG.refactor.label, icon: CATEGORY_CONFIG.refactor.icon, color: CATEGORY_CONFIG.refactor.color },
		{ value: 'chore', label: CATEGORY_CONFIG.chore.label, icon: CATEGORY_CONFIG.chore.icon, color: CATEGORY_CONFIG.chore.color },
		{ value: 'docs', label: CATEGORY_CONFIG.docs.label, icon: CATEGORY_CONFIG.docs.icon, color: CATEGORY_CONFIG.docs.color },
		{ value: 'test', label: CATEGORY_CONFIG.test.label, icon: CATEGORY_CONFIG.test.icon, color: CATEGORY_CONFIG.test.color }
	];

	// Focus input when modal opens
	$effect(() => {
		if (open && titleInputRef) {
			// Small delay to ensure modal is rendered
			setTimeout(() => titleInputRef?.focus(), 50);
		}
	});

	// Reset form when modal closes
	$effect(() => {
		if (!open) {
			title = '';
			description = '';
			category = 'feature';
			error = null;
			creating = false;
		}
	});

	async function handleSubmit() {
		if (!title.trim() || creating) return;

		creating = true;
		error = null;

		try {
			const projectId = $currentProjectId;
			let newTask;

			if (projectId) {
				newTask = await createProjectTask(projectId, title.trim(), description.trim() || undefined, undefined, category);
			} else {
				newTask = await createTask(title.trim(), description.trim() || undefined, undefined, category);
			}

			// Add to store
			addTask(newTask);

			// Show success and close
			toast.success(`Created task ${newTask.id}`, { title: 'Task Created' });
			onClose();

			// Navigate to the new task
			goto(`/tasks/${newTask.id}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create task';
			toast.error(error);
		} finally {
			creating = false;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
			handleSubmit();
		}
	}
</script>

<Modal {open} {onClose} size="md" title="Create New Task">
	<form class="new-task-form" onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
		{#if error}
			<div class="error-message">
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<circle cx="12" cy="12" r="10" />
					<line x1="12" y1="8" x2="12" y2="12" />
					<line x1="12" y1="16" x2="12.01" y2="16" />
				</svg>
				<span>{error}</span>
			</div>
		{/if}

		<label class="form-label">
			Task Title
			<input
				bind:this={titleInputRef}
				type="text"
				placeholder="What needs to be done?"
				bind:value={title}
				onkeydown={handleKeydown}
				class="form-input"
				disabled={creating}
			/>
		</label>

		<label class="form-label">
			Description <span class="optional">(optional)</span>
			<textarea
				placeholder="Provide additional context, acceptance criteria, or implementation details..."
				bind:value={description}
				onkeydown={handleKeydown}
				class="form-textarea"
				rows="4"
				disabled={creating}
			></textarea>
		</label>

		<div class="form-field">
			<!-- svelte-ignore a11y_label_has_associated_control -->
			<label class="form-label" id="category-label">Category</label>
			<div class="category-options" role="radiogroup" aria-labelledby="category-label">
				{#each categoryOptions as option}
					<label
						class="category-option"
						class:selected={category === option.value}
						style:--category-color={option.color}
					>
						<input
							type="radio"
							name="category"
							value={option.value}
							bind:group={category}
							disabled={creating}
						/>
						<span class="category-icon">{option.icon}</span>
						<span class="category-label">{option.label}</span>
					</label>
				{/each}
			</div>
		</div>

		<p class="form-hint">
			Orc will classify the weight and create a plan automatically based on the title and description.
		</p>

		<div class="form-actions">
			<button type="button" onclick={onClose} disabled={creating}>
				Cancel
			</button>
			<button type="submit" class="primary" disabled={!title.trim() || creating}>
				{#if creating}
					<span class="spinner"></span>
					Creating...
				{:else}
					Create Task
				{/if}
			</button>
		</div>
	</form>
</Modal>

<style>
	.new-task-form {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.error-message {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-3);
		background: var(--status-danger-bg);
		border: 1px solid var(--status-danger);
		border-radius: var(--radius-md);
		color: var(--status-danger);
		font-size: var(--text-sm);
	}

	.form-label {
		display: block;
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
	}

	.form-input {
		width: 100%;
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		font-size: var(--text-base);
		color: var(--text-primary);
		margin-top: var(--space-2);
		transition: all var(--duration-fast) var(--ease-out);
	}

	.form-input:focus {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.form-input:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.form-input::placeholder {
		color: var(--text-muted);
	}

	.form-textarea {
		width: 100%;
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		color: var(--text-primary);
		margin-top: var(--space-2);
		resize: vertical;
		min-height: 80px;
		font-family: inherit;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.form-textarea:focus {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.form-textarea:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.form-textarea::placeholder {
		color: var(--text-muted);
	}

	.optional {
		font-weight: var(--font-normal);
		color: var(--text-muted);
	}

	.form-hint {
		font-size: var(--text-xs);
		color: var(--text-muted);
		margin: 0;
	}

	.form-field {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	/* Category options */
	.category-options {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2);
	}

	.category-option {
		flex: 1 1 calc(33.333% - var(--space-2));
		min-width: 80px;
		display: flex;
		align-items: center;
		gap: var(--space-1-5);
		padding: var(--space-2);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		cursor: pointer;
		transition:
			border-color var(--duration-fast) var(--ease-out),
			background var(--duration-fast) var(--ease-out);
	}

	.category-option input {
		position: absolute;
		opacity: 0;
		pointer-events: none;
	}

	.category-option:hover {
		border-color: var(--border-strong);
	}

	.category-option.selected {
		border-width: 2px;
		border-color: var(--category-color);
		background: color-mix(in srgb, var(--category-color) 10%, transparent);
	}

	.category-icon {
		font-size: var(--text-base);
	}

	.category-label {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	@media (max-width: 640px) {
		.category-option {
			flex: 1 1 calc(50% - var(--space-2));
		}
	}

	.form-actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-3);
		margin-top: var(--space-2);
	}

	.spinner {
		display: inline-block;
		width: 14px;
		height: 14px;
		border: 2px solid currentColor;
		border-top-color: transparent;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
