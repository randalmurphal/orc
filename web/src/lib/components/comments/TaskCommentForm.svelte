<script lang="ts">
	import type { TaskCommentAuthorType, CreateTaskCommentRequest } from '$lib/types';
	import Icon from '$lib/components/ui/Icon.svelte';

	interface Props {
		initialPhase?: string;
		phases?: string[];
		onSubmit: (comment: CreateTaskCommentRequest) => void;
		onCancel: () => void;
		isLoading?: boolean;
		editMode?: boolean;
		initialContent?: string;
	}

	let { initialPhase, phases = [], onSubmit, onCancel, isLoading = false, editMode = false, initialContent = '' }: Props = $props();

	// svelte-ignore state_referenced_locally
	let content = $state(initialContent);
	// svelte-ignore state_referenced_locally
	let phase = $state(initialPhase ?? '');
	let authorType = $state<TaskCommentAuthorType>('human');
	let author = $state('');
	let textareaEl = $state<HTMLTextAreaElement | null>(null);

	// Focus textarea on mount
	$effect(() => {
		if (textareaEl) {
			textareaEl.focus();
		}
	});

	// Platform detection for keyboard hints
	const isMac = $derived(typeof navigator !== 'undefined' && /Mac|iPhone|iPad|iPod/.test(navigator.platform));
	const modifierKey = $derived(isMac ? 'Cmd' : 'Ctrl');

	const authorTypeOptions: { value: TaskCommentAuthorType; label: string; description: string }[] = [
		{ value: 'human', label: 'Human', description: 'Manual note or feedback' },
		{ value: 'agent', label: 'Agent', description: 'Note from Claude/AI' },
		{ value: 'system', label: 'System', description: 'Automated system note' }
	];

	const canSubmit = $derived(content.trim().length > 0 && !isLoading);

	function handleSubmit(event: Event) {
		event.preventDefault();
		if (!canSubmit) return;

		const comment: CreateTaskCommentRequest = {
			content: content.trim(),
			author_type: authorType
		};

		if (author.trim()) {
			comment.author = author.trim();
		}

		if (phase) {
			comment.phase = phase;
		}

		onSubmit(comment);
	}

	function handleKeyDown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			onCancel();
		} else if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
			handleSubmit(event);
		}
	}
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<form class="comment-form" onsubmit={handleSubmit} onkeydown={handleKeyDown}>
	<div class="form-header">
		<h3>{editMode ? 'Edit Comment' : 'Add Comment'}</h3>
		<button type="button" class="close-btn" onclick={onCancel} title="Cancel">
			<Icon name="close" size={16} />
		</button>
	</div>

	{#if !editMode}
		<div class="form-row">
			<div class="form-field author-field">
				<label for="author">Author (optional)</label>
				<input
					id="author"
					type="text"
					bind:value={author}
					placeholder="Your name"
					disabled={isLoading}
				/>
			</div>
			{#if phases.length > 0}
				<div class="form-field phase-field">
					<label for="phase">Phase (optional)</label>
					<select id="phase" bind:value={phase} disabled={isLoading}>
						<option value="">No phase</option>
						{#each phases as p}
							<option value={p}>{p}</option>
						{/each}
					</select>
				</div>
			{/if}
		</div>

		<div class="form-field">
			<label for="author-type">Type</label>
			<div class="author-type-options">
				{#each authorTypeOptions as option}
					<label
						class="author-type-option"
						class:selected={authorType === option.value}
						class:human={option.value === 'human'}
						class:agent={option.value === 'agent'}
						class:system={option.value === 'system'}
					>
						<input
							type="radio"
							name="author-type"
							value={option.value}
							bind:group={authorType}
							disabled={isLoading}
						/>
						<span class="author-type-label">{option.label}</span>
						<span class="author-type-desc">{option.description}</span>
					</label>
				{/each}
			</div>
		</div>
	{/if}

	<div class="form-field">
		<label for="content">Comment</label>
		<textarea
			id="content"
			bind:this={textareaEl}
			bind:value={content}
			placeholder="Add a note, feedback, or context..."
			rows="4"
			disabled={isLoading}
		></textarea>
	</div>

	<div class="form-actions">
		<button type="button" class="ghost" onclick={onCancel} disabled={isLoading}>
			Cancel
		</button>
		<button type="submit" class="primary" disabled={!canSubmit}>
			{#if isLoading}
				{editMode ? 'Saving...' : 'Adding...'}
			{:else}
				{editMode ? 'Save Changes' : 'Add Comment'}
			{/if}
		</button>
	</div>

	<div class="keyboard-hint">
		<kbd>{modifierKey}</kbd> + <kbd>Enter</kbd> to submit
	</div>
</form>

<style>
	.comment-form {
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-lg);
		padding: var(--space-4);
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.form-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.form-header h3 {
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
	}

	.close-btn {
		padding: var(--space-1);
		background: transparent;
		border: none;
		color: var(--text-muted);
		cursor: pointer;
		border-radius: var(--radius-sm);
		transition: color var(--duration-fast) var(--ease-out);
	}

	.close-btn:hover {
		color: var(--text-primary);
	}

	.form-row {
		display: flex;
		gap: var(--space-3);
	}

	.form-field {
		display: flex;
		flex-direction: column;
		gap: var(--space-1-5);
	}

	.author-field {
		flex: 1;
	}

	.phase-field {
		width: 140px;
	}

	.form-field label {
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
	}

	.form-field input,
	.form-field textarea,
	.form-field select {
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
	.form-field textarea:focus,
	.form-field select:focus {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.form-field input::placeholder,
	.form-field textarea::placeholder {
		color: var(--text-muted);
	}

	.form-field input:disabled,
	.form-field textarea:disabled,
	.form-field select:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.form-field textarea {
		resize: vertical;
		min-height: 80px;
		font-family: var(--font-body);
	}

	.author-type-options {
		display: flex;
		gap: var(--space-2);
	}

	.author-type-option {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: var(--space-0-5);
		padding: var(--space-2);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		cursor: pointer;
		transition:
			border-color var(--duration-fast) var(--ease-out),
			background var(--duration-fast) var(--ease-out);
	}

	.author-type-option input {
		position: absolute;
		opacity: 0;
		pointer-events: none;
	}

	.author-type-option:hover {
		border-color: var(--border-strong);
	}

	.author-type-option.selected {
		border-width: 2px;
	}

	.author-type-option.selected.human {
		border-color: var(--status-info);
		background: var(--status-info-bg);
	}

	.author-type-option.selected.agent {
		border-color: var(--accent-primary);
		background: var(--accent-glow);
	}

	.author-type-option.selected.system {
		border-color: var(--text-muted);
		background: var(--bg-tertiary);
	}

	.author-type-label {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.author-type-desc {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.form-actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
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
</style>
