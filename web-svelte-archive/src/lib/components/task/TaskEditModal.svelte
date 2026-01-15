<script lang="ts">
	import type { Task, TaskWeight, TaskQueue, TaskPriority, TaskCategory } from '$lib/types';
	import { CATEGORY_CONFIG } from '$lib/types';
	import Modal from '$lib/components/overlays/Modal.svelte';

	interface Props {
		task: Task;
		open: boolean;
		onClose: () => void;
		onSave: (update: { title?: string; description?: string; weight?: TaskWeight; queue?: TaskQueue; priority?: TaskPriority; category?: TaskCategory }) => Promise<void>;
	}

	let { task, open, onClose, onSave }: Props = $props();

	// Form state - initialized from task props (intentionally captured once at mount)
	// svelte-ignore state_referenced_locally
	let title = $state(task.title);
	// svelte-ignore state_referenced_locally
	let description = $state(task.description ?? '');
	// svelte-ignore state_referenced_locally
	let weight = $state<TaskWeight>(task.weight);
	// svelte-ignore state_referenced_locally
	let queue = $state<TaskQueue>(task.queue ?? 'active');
	// svelte-ignore state_referenced_locally
	let priority = $state<TaskPriority>(task.priority ?? 'normal');
	// svelte-ignore state_referenced_locally
	let category = $state<TaskCategory>(task.category ?? 'feature');
	let isLoading = $state(false);
	let error = $state<string | null>(null);

	// Reset form when task changes or modal opens
	$effect(() => {
		if (open) {
			title = task.title;
			description = task.description ?? '';
			weight = task.weight;
			queue = task.queue ?? 'active';
			priority = task.priority ?? 'normal';
			category = task.category ?? 'feature';
			error = null;
		}
	});

	// Platform detection for keyboard hints
	const isMac = $derived(
		typeof navigator !== 'undefined' && /Mac|iPhone|iPad|iPod/.test(navigator.platform)
	);
	const modifierKey = $derived(isMac ? 'Cmd' : 'Ctrl');

	const weightOptions: { value: TaskWeight; label: string; description: string }[] = [
		{ value: 'trivial', label: 'Trivial', description: 'One-liner fix' },
		{ value: 'small', label: 'Small', description: 'Bug fix, small feature' },
		{ value: 'medium', label: 'Medium', description: 'Feature with tests' },
		{ value: 'large', label: 'Large', description: 'Complex feature' },
		{ value: 'greenfield', label: 'Greenfield', description: 'New system' }
	];

	const queueOptions: { value: TaskQueue; label: string; description: string }[] = [
		{ value: 'active', label: 'Active', description: 'Current work queue' },
		{ value: 'backlog', label: 'Backlog', description: 'Someday/maybe items' }
	];

	const priorityOptions: { value: TaskPriority; label: string; description: string; color: string }[] = [
		{ value: 'critical', label: 'Critical', description: 'Needs immediate attention', color: 'var(--status-error)' },
		{ value: 'high', label: 'High', description: 'Should be done soon', color: 'var(--status-warning)' },
		{ value: 'normal', label: 'Normal', description: 'Regular priority', color: 'var(--text-muted)' },
		{ value: 'low', label: 'Low', description: 'Can wait', color: 'var(--text-disabled)' }
	];

	const categoryOptions: { value: TaskCategory; label: string; icon: string; color: string }[] = [
		{ value: 'feature', label: CATEGORY_CONFIG.feature.label, icon: CATEGORY_CONFIG.feature.icon, color: CATEGORY_CONFIG.feature.color },
		{ value: 'bug', label: CATEGORY_CONFIG.bug.label, icon: CATEGORY_CONFIG.bug.icon, color: CATEGORY_CONFIG.bug.color },
		{ value: 'refactor', label: CATEGORY_CONFIG.refactor.label, icon: CATEGORY_CONFIG.refactor.icon, color: CATEGORY_CONFIG.refactor.color },
		{ value: 'chore', label: CATEGORY_CONFIG.chore.label, icon: CATEGORY_CONFIG.chore.icon, color: CATEGORY_CONFIG.chore.color },
		{ value: 'docs', label: CATEGORY_CONFIG.docs.label, icon: CATEGORY_CONFIG.docs.icon, color: CATEGORY_CONFIG.docs.color },
		{ value: 'test', label: CATEGORY_CONFIG.test.label, icon: CATEGORY_CONFIG.test.icon, color: CATEGORY_CONFIG.test.color }
	];

	const hasChanges = $derived(
		title !== task.title ||
		description !== (task.description ?? '') ||
		weight !== task.weight ||
		queue !== (task.queue ?? 'active') ||
		priority !== (task.priority ?? 'normal') ||
		category !== (task.category ?? 'feature')
	);

	const canSubmit = $derived(title.trim().length > 0 && hasChanges && !isLoading);

	async function handleSubmit(event: Event) {
		event.preventDefault();
		if (!canSubmit) return;

		isLoading = true;
		error = null;

		try {
			const update: { title?: string; description?: string; weight?: TaskWeight; queue?: TaskQueue; priority?: TaskPriority; category?: TaskCategory } = {};

			if (title !== task.title) {
				update.title = title.trim();
			}
			if (description !== (task.description ?? '')) {
				update.description = description.trim();
			}
			if (weight !== task.weight) {
				update.weight = weight;
			}
			if (queue !== (task.queue ?? 'active')) {
				update.queue = queue;
			}
			if (priority !== (task.priority ?? 'normal')) {
				update.priority = priority;
			}
			if (category !== (task.category ?? 'feature')) {
				update.category = category;
			}

			await onSave(update);
			onClose();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to update task';
		} finally {
			isLoading = false;
		}
	}

	function handleKeyDown(event: KeyboardEvent) {
		if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
			handleSubmit(event);
		}
	}
</script>

<Modal {open} {onClose} title="Edit Task" size="md">
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<form class="edit-form" onsubmit={handleSubmit} onkeydown={handleKeyDown}>
		{#if error}
			<div class="error-banner">
				{error}
			</div>
		{/if}

		<div class="form-field">
			<label for="task-title">Title</label>
			<input
				id="task-title"
				type="text"
				bind:value={title}
				placeholder="Task title"
				disabled={isLoading}
				required
			/>
		</div>

		<div class="form-field">
			<label for="task-description">Description</label>
			<textarea
				id="task-description"
				bind:value={description}
				placeholder="Describe what needs to be done..."
				rows="4"
				disabled={isLoading}
			></textarea>
		</div>

		<div class="form-field">
			<!-- svelte-ignore a11y_label_has_associated_control -->
			<label id="weight-label">Weight</label>
			<div class="weight-options" role="radiogroup" aria-labelledby="weight-label">
				{#each weightOptions as option}
					<label
						class="weight-option"
						class:selected={weight === option.value}
						class:trivial={option.value === 'trivial'}
						class:small={option.value === 'small'}
						class:medium={option.value === 'medium'}
						class:large={option.value === 'large'}
						class:greenfield={option.value === 'greenfield'}
					>
						<input
							type="radio"
							name="weight"
							value={option.value}
							bind:group={weight}
							disabled={isLoading}
						/>
						<span class="weight-label">{option.label}</span>
						<span class="weight-desc">{option.description}</span>
					</label>
				{/each}
			</div>
		</div>

		<div class="form-field">
			<!-- svelte-ignore a11y_label_has_associated_control -->
			<label id="category-label">Category</label>
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
							disabled={isLoading}
						/>
						<span class="category-icon">{option.icon}</span>
						<span class="category-label">{option.label}</span>
					</label>
				{/each}
			</div>
		</div>

		<div class="form-row">
			<div class="form-field flex-1">
				<!-- svelte-ignore a11y_label_has_associated_control -->
				<label id="queue-label">Queue</label>
				<div class="toggle-options" role="radiogroup" aria-labelledby="queue-label">
					{#each queueOptions as option}
						<label
							class="toggle-option"
							class:selected={queue === option.value}
							class:backlog={option.value === 'backlog' && queue === option.value}
						>
							<input
								type="radio"
								name="queue"
								value={option.value}
								bind:group={queue}
								disabled={isLoading}
							/>
							<span class="toggle-label">{option.label}</span>
						</label>
					{/each}
				</div>
			</div>

			<div class="form-field flex-1">
				<!-- svelte-ignore a11y_label_has_associated_control -->
				<label id="priority-label">Priority</label>
				<div class="priority-options" role="radiogroup" aria-labelledby="priority-label">
					{#each priorityOptions as option}
						<label
							class="priority-option"
							class:selected={priority === option.value}
							style:--priority-color={option.color}
						>
							<input
								type="radio"
								name="priority"
								value={option.value}
								bind:group={priority}
								disabled={isLoading}
							/>
							<span class="priority-indicator" style:background={option.color}></span>
							<span class="priority-label">{option.label}</span>
						</label>
					{/each}
				</div>
			</div>
		</div>

		<div class="form-actions">
			<button type="button" class="ghost" onclick={onClose} disabled={isLoading}>Cancel</button>
			<button type="submit" class="primary" disabled={!canSubmit}>
				{#if isLoading}
					Saving...
				{:else}
					Save Changes
				{/if}
			</button>
		</div>

		<div class="keyboard-hint">
			<kbd>{modifierKey}</kbd> + <kbd>Enter</kbd> to save
		</div>
	</form>
</Modal>

<style>
	.edit-form {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.error-banner {
		padding: var(--space-3);
		background: var(--status-danger-bg);
		border: 1px solid var(--status-danger);
		border-radius: var(--radius-md);
		color: var(--status-danger);
		font-size: var(--text-sm);
	}

	.form-field {
		display: flex;
		flex-direction: column;
		gap: var(--space-1-5);
	}

	.form-field label {
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
	}

	.form-field input,
	.form-field textarea {
		font-size: var(--text-sm);
		padding: var(--space-2) var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-primary);
		transition:
			border-color var(--duration-fast) var(--ease-out),
			box-shadow var(--duration-fast) var(--ease-out);
	}

	.form-field input:focus,
	.form-field textarea:focus {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.form-field input::placeholder,
	.form-field textarea::placeholder {
		color: var(--text-muted);
	}

	.form-field input:disabled,
	.form-field textarea:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.form-field textarea {
		resize: vertical;
		min-height: 80px;
		font-family: var(--font-body);
	}

	.weight-options {
		display: grid;
		grid-template-columns: repeat(5, 1fr);
		gap: var(--space-2);
	}

	.weight-option {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-0-5);
		padding: var(--space-2);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		cursor: pointer;
		text-align: center;
		transition:
			border-color var(--duration-fast) var(--ease-out),
			background var(--duration-fast) var(--ease-out);
	}

	.weight-option input {
		position: absolute;
		opacity: 0;
		pointer-events: none;
	}

	.weight-option:hover {
		border-color: var(--border-strong);
	}

	.weight-option.selected {
		border-width: 2px;
	}

	.weight-option.selected.trivial {
		border-color: var(--weight-trivial);
		background: rgba(107, 114, 128, 0.15);
	}

	.weight-option.selected.small {
		border-color: var(--weight-small);
		background: var(--status-success-bg);
	}

	.weight-option.selected.medium {
		border-color: var(--weight-medium);
		background: var(--status-info-bg);
	}

	.weight-option.selected.large {
		border-color: var(--weight-large);
		background: var(--status-warning-bg);
	}

	.weight-option.selected.greenfield {
		border-color: var(--weight-greenfield);
		background: var(--accent-subtle);
	}

	.weight-label {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.weight-desc {
		font-size: var(--text-2xs);
		color: var(--text-muted);
		line-height: var(--leading-tight);
	}

	.form-actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
		padding-top: var(--space-2);
	}

	.keyboard-hint {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-1);
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.keyboard-hint kbd {
		padding: var(--space-0-5) var(--space-1);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		font-family: var(--font-mono);
		font-size: var(--text-2xs);
	}

	/* Queue and Priority row */
	.form-row {
		display: flex;
		gap: var(--space-4);
	}

	.flex-1 {
		flex: 1;
	}

	/* Toggle options (Queue) */
	.toggle-options {
		display: flex;
		gap: var(--space-2);
	}

	.toggle-option {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: var(--space-2);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		cursor: pointer;
		transition:
			border-color var(--duration-fast) var(--ease-out),
			background var(--duration-fast) var(--ease-out);
	}

	.toggle-option input {
		position: absolute;
		opacity: 0;
		pointer-events: none;
	}

	.toggle-option:hover {
		border-color: var(--border-strong);
	}

	.toggle-option.selected {
		border-width: 2px;
		border-color: var(--accent-primary);
		background: var(--accent-subtle);
	}

	.toggle-option.selected.backlog {
		border-color: var(--text-muted);
		background: var(--bg-secondary);
	}

	.toggle-label {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	/* Priority options */
	.priority-options {
		display: flex;
		gap: var(--space-1);
	}

	.priority-option {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-2) var(--space-1);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		cursor: pointer;
		transition:
			border-color var(--duration-fast) var(--ease-out),
			background var(--duration-fast) var(--ease-out);
	}

	.priority-option input {
		position: absolute;
		opacity: 0;
		pointer-events: none;
	}

	.priority-option:hover {
		border-color: var(--border-strong);
	}

	.priority-option.selected {
		border-width: 2px;
		border-color: var(--priority-color);
		background: color-mix(in srgb, var(--priority-color) 10%, transparent);
	}

	.priority-indicator {
		width: 8px;
		height: 8px;
		border-radius: var(--radius-full);
	}

	.priority-label {
		font-size: var(--text-2xs);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
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
		.weight-options {
			grid-template-columns: repeat(2, 1fr);
		}

		.weight-option:last-child {
			grid-column: span 2;
		}

		.form-row {
			flex-direction: column;
		}

		.priority-options {
			flex-wrap: wrap;
		}

		.priority-option {
			flex: 1 1 calc(50% - var(--space-1));
		}

		.category-option {
			flex: 1 1 calc(50% - var(--space-2));
		}
	}
</style>
