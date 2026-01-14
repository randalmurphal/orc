<!--
	NewInitiativeModal - Modal for creating new initiatives

	Lives in +layout.svelte so it can be triggered from sidebar.
-->
<script lang="ts">
	import { createNewInitiative, selectInitiative } from '$lib/stores/initiative';
	import { toast } from '$lib/stores/toast.svelte';
	import Modal from './Modal.svelte';

	interface Props {
		open: boolean;
		onClose: () => void;
	}

	let { open, onClose }: Props = $props();

	let title = $state('');
	let vision = $state('');
	let ownerInitials = $state('');
	let creating = $state(false);
	let error = $state<string | null>(null);
	let titleInputRef: HTMLInputElement;

	// Focus input when modal opens
	$effect(() => {
		if (open && titleInputRef) {
			setTimeout(() => titleInputRef?.focus(), 50);
		}
	});

	// Reset form when modal closes
	$effect(() => {
		if (!open) {
			title = '';
			vision = '';
			ownerInitials = '';
			error = null;
			creating = false;
		}
	});

	async function handleSubmit() {
		if (!title.trim() || creating) return;

		creating = true;
		error = null;

		try {
			const initiative = await createNewInitiative({
				title: title.trim(),
				vision: vision.trim() || undefined,
				owner: ownerInitials.trim() ? { initials: ownerInitials.trim() } : undefined
			});

			toast.success(`Created initiative ${initiative.id}`, { title: 'Initiative Created' });

			// Auto-select the new initiative
			selectInitiative(initiative.id);

			onClose();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create initiative';
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

<Modal {open} {onClose} size="sm" title="New Initiative">
	<form class="new-initiative-form" onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
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
			Title
			<input
				bind:this={titleInputRef}
				type="text"
				placeholder="e.g., Frontend Migration"
				bind:value={title}
				onkeydown={handleKeydown}
				class="form-input"
				disabled={creating}
			/>
		</label>

		<label class="form-label">
			Vision <span class="optional">(optional)</span>
			<textarea
				placeholder="What is the goal of this initiative?"
				bind:value={vision}
				onkeydown={handleKeydown}
				class="form-textarea"
				rows="3"
				disabled={creating}
			></textarea>
		</label>

		<label class="form-label">
			Owner <span class="optional">(optional)</span>
			<input
				type="text"
				placeholder="e.g., JD"
				bind:value={ownerInitials}
				onkeydown={handleKeydown}
				class="form-input"
				disabled={creating}
				maxlength="5"
			/>
			<span class="form-hint">Initials or short identifier</span>
		</label>

		<div class="form-actions">
			<button type="button" onclick={onClose} disabled={creating}>
				Cancel
			</button>
			<button type="submit" class="primary" disabled={!title.trim() || creating}>
				{#if creating}
					<span class="spinner"></span>
					Creating...
				{:else}
					Create Initiative
				{/if}
			</button>
		</div>
	</form>
</Modal>

<style>
	.new-initiative-form {
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
		min-height: 60px;
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
		display: block;
		font-size: var(--text-xs);
		color: var(--text-muted);
		margin-top: var(--space-1);
		font-weight: var(--font-normal);
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
