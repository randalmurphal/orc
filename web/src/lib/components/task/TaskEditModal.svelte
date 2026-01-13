<script lang="ts">
	import type { Task, TaskWeight } from '$lib/types';
	import Modal from '$lib/components/overlays/Modal.svelte';

	interface Props {
		task: Task;
		open: boolean;
		onClose: () => void;
		onSave: (update: { title?: string; description?: string; weight?: TaskWeight }) => Promise<void>;
	}

	let { task, open, onClose, onSave }: Props = $props();

	// Form state - initialized from task props (intentionally captured once at mount)
	// svelte-ignore state_referenced_locally
	let title = $state(task.title);
	// svelte-ignore state_referenced_locally
	let description = $state(task.description ?? '');
	// svelte-ignore state_referenced_locally
	let weight = $state<TaskWeight>(task.weight);
	let isLoading = $state(false);
	let error = $state<string | null>(null);

	// Reset form when task changes or modal opens
	$effect(() => {
		if (open) {
			title = task.title;
			description = task.description ?? '';
			weight = task.weight;
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

	const hasChanges = $derived(
		title !== task.title || description !== (task.description ?? '') || weight !== task.weight
	);

	const canSubmit = $derived(title.trim().length > 0 && hasChanges && !isLoading);

	async function handleSubmit(event: Event) {
		event.preventDefault();
		if (!canSubmit) return;

		isLoading = true;
		error = null;

		try {
			const update: { title?: string; description?: string; weight?: TaskWeight } = {};

			if (title !== task.title) {
				update.title = title.trim();
			}
			if (description !== (task.description ?? '')) {
				update.description = description.trim();
			}
			if (weight !== task.weight) {
				update.weight = weight;
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

	@media (max-width: 640px) {
		.weight-options {
			grid-template-columns: repeat(2, 1fr);
		}

		.weight-option:last-child {
			grid-column: span 2;
		}
	}
</style>
