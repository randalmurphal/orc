<script lang="ts">
	import type { CommentSeverity, CreateCommentRequest } from '$lib/types';
	import Icon from '$lib/components/ui/Icon.svelte';

	interface Props {
		initialFilePath?: string;
		initialLineNumber?: number;
		onSubmit: (comment: CreateCommentRequest) => void;
		onCancel: () => void;
		isLoading?: boolean;
	}

	let { initialFilePath, initialLineNumber, onSubmit, onCancel, isLoading = false }: Props = $props();

	let filePath = $state(initialFilePath ?? '');
	let lineNumber = $state<number | undefined>(initialLineNumber);
	let content = $state('');
	let severity = $state<CommentSeverity>('issue');

	const severityOptions: { value: CommentSeverity; label: string; description: string }[] = [
		{ value: 'suggestion', label: 'Suggestion', description: 'Optional improvement' },
		{ value: 'issue', label: 'Issue', description: 'Should be fixed' },
		{ value: 'blocker', label: 'Blocker', description: 'Must be fixed before merge' }
	];

	const canSubmit = $derived(content.trim().length > 0 && !isLoading);

	function handleSubmit(event: Event) {
		event.preventDefault();
		if (!canSubmit) return;

		const comment: CreateCommentRequest = {
			content: content.trim(),
			severity
		};

		if (filePath.trim()) {
			comment.file_path = filePath.trim();
		}

		if (lineNumber !== undefined && lineNumber > 0) {
			comment.line_number = lineNumber;
		}

		onSubmit(comment);
	}

	function handleKeyDown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			onCancel();
		} else if (event.key === 'Enter' && event.metaKey) {
			handleSubmit(event);
		}
	}
</script>

<form class="comment-form" onsubmit={handleSubmit} onkeydown={handleKeyDown}>
	<div class="form-header">
		<h3>Add Comment</h3>
		<button type="button" class="close-btn" onclick={onCancel} title="Cancel">
			<Icon name="close" size={16} />
		</button>
	</div>

	<div class="form-row location-row">
		<div class="form-field file-field">
			<label for="file-path">File (optional)</label>
			<input
				id="file-path"
				type="text"
				bind:value={filePath}
				placeholder="path/to/file.ts"
				disabled={isLoading}
			/>
		</div>
		<div class="form-field line-field">
			<label for="line-number">Line</label>
			<input
				id="line-number"
				type="number"
				bind:value={lineNumber}
				placeholder="#"
				min="1"
				disabled={isLoading}
			/>
		</div>
	</div>

	<div class="form-field">
		<label for="severity">Severity</label>
		<div class="severity-options">
			{#each severityOptions as option}
				<label
					class="severity-option"
					class:selected={severity === option.value}
					class:suggestion={option.value === 'suggestion'}
					class:issue={option.value === 'issue'}
					class:blocker={option.value === 'blocker'}
				>
					<input
						type="radio"
						name="severity"
						value={option.value}
						bind:group={severity}
						disabled={isLoading}
					/>
					<span class="severity-label">{option.label}</span>
					<span class="severity-desc">{option.description}</span>
				</label>
			{/each}
		</div>
	</div>

	<div class="form-field">
		<label for="content">Comment</label>
		<textarea
			id="content"
			bind:value={content}
			placeholder="Describe the issue or suggestion..."
			rows="4"
			disabled={isLoading}
			autofocus
		></textarea>
	</div>

	<div class="form-actions">
		<button type="button" class="ghost" onclick={onCancel} disabled={isLoading}>
			Cancel
		</button>
		<button type="submit" class="primary" disabled={!canSubmit}>
			{#if isLoading}
				Adding...
			{:else}
				Add Comment
			{/if}
		</button>
	</div>

	<div class="keyboard-hint">
		<kbd>Cmd</kbd> + <kbd>Enter</kbd> to submit
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

	.file-field {
		flex: 1;
	}

	.line-field {
		width: 80px;
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

	.severity-options {
		display: flex;
		gap: var(--space-2);
	}

	.severity-option {
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

	.severity-option input {
		position: absolute;
		opacity: 0;
		pointer-events: none;
	}

	.severity-option:hover {
		border-color: var(--border-strong);
	}

	.severity-option.selected {
		border-width: 2px;
	}

	.severity-option.selected.suggestion {
		border-color: var(--status-info);
		background: var(--status-info-bg);
	}

	.severity-option.selected.issue {
		border-color: var(--status-warning);
		background: var(--status-warning-bg);
	}

	.severity-option.selected.blocker {
		border-color: var(--status-danger);
		background: var(--status-danger-bg);
	}

	.severity-label {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.severity-desc {
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
