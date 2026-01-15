<script lang="ts">
	import { onMount } from 'svelte';
	import type { Attachment } from '$lib/types';
	import {
		listAttachments,
		uploadAttachment,
		deleteAttachment,
		getAttachmentUrl
	} from '$lib/api';

	interface Props {
		taskId: string;
	}

	let { taskId }: Props = $props();

	let attachments = $state<Attachment[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let uploading = $state(false);
	let dragOver = $state(false);

	// For lightbox
	let lightboxImage = $state<string | null>(null);
	let lightboxFilename = $state<string | null>(null);

	onMount(async () => {
		await loadAttachments();
	});

	async function loadAttachments() {
		loading = true;
		error = null;

		try {
			attachments = await listAttachments(taskId);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load attachments';
		} finally {
			loading = false;
		}
	}

	async function handleUpload(files: FileList | null) {
		if (!files || files.length === 0) return;

		uploading = true;
		error = null;

		try {
			for (const file of files) {
				await uploadAttachment(taskId, file);
			}
			await loadAttachments();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Upload failed';
		} finally {
			uploading = false;
		}
	}

	async function handleDelete(filename: string) {
		if (!confirm(`Delete "${filename}"?`)) return;

		try {
			await deleteAttachment(taskId, filename);
			attachments = attachments.filter((a) => a.filename !== filename);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Delete failed';
		}
	}

	function handleDragOver(e: DragEvent) {
		e.preventDefault();
		dragOver = true;
	}

	function handleDragLeave() {
		dragOver = false;
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		dragOver = false;
		handleUpload(e.dataTransfer?.files ?? null);
	}

	function openLightbox(filename: string) {
		lightboxImage = getAttachmentUrl(taskId, filename);
		lightboxFilename = filename;
	}

	function closeLightbox() {
		lightboxImage = null;
		lightboxFilename = null;
	}

	function formatSize(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
	}

	function formatDate(dateStr: string): string {
		const date = new Date(dateStr);
		return date.toLocaleDateString(undefined, {
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	// Split attachments into images and files
	const images = $derived(attachments.filter((a) => a.is_image));
	const files = $derived(attachments.filter((a) => !a.is_image));
</script>

<div class="attachments-container">
	<!-- Upload area -->
	<div
		class="upload-area"
		class:drag-over={dragOver}
		ondragover={handleDragOver}
		ondragleave={handleDragLeave}
		ondrop={handleDrop}
		role="region"
		aria-label="File upload area"
	>
		<input
			type="file"
			id="file-upload"
			multiple
			onchange={(e) => handleUpload(e.currentTarget.files)}
			class="file-input"
		/>
		<label for="file-upload" class="upload-label">
			<svg
				xmlns="http://www.w3.org/2000/svg"
				width="24"
				height="24"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"
				stroke-linejoin="round"
			>
				<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
				<polyline points="17 8 12 3 7 8" />
				<line x1="12" y1="3" x2="12" y2="15" />
			</svg>
			{#if uploading}
				<span>Uploading...</span>
			{:else}
				<span>Drop files here or click to upload</span>
			{/if}
		</label>
	</div>

	{#if error}
		<div class="error-message">{error}</div>
	{/if}

	{#if loading}
		<div class="loading-state">
			<div class="spinner"></div>
			<span>Loading attachments...</span>
		</div>
	{:else if attachments.length === 0}
		<div class="empty-state">
			<p>No attachments yet</p>
		</div>
	{:else}
		<!-- Images gallery -->
		{#if images.length > 0}
			<div class="section">
				<h3 class="section-title">Images ({images.length})</h3>
				<div class="images-grid">
					{#each images as attachment}
						<div class="image-card">
							<button
								class="image-preview"
								onclick={() => openLightbox(attachment.filename)}
								title="Click to enlarge"
							>
								<img
									src={getAttachmentUrl(taskId, attachment.filename)}
									alt={attachment.filename}
									loading="lazy"
								/>
							</button>
							<div class="image-info">
								<span class="image-name" title={attachment.filename}>{attachment.filename}</span>
								<span class="image-meta">{formatSize(attachment.size)}</span>
							</div>
							<button
								class="delete-btn"
								onclick={() => handleDelete(attachment.filename)}
								title="Delete"
							>
								<svg
									xmlns="http://www.w3.org/2000/svg"
									width="14"
									height="14"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									stroke-width="2"
								>
									<polyline points="3 6 5 6 21 6" />
									<path
										d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"
									/>
								</svg>
							</button>
						</div>
					{/each}
				</div>
			</div>
		{/if}

		<!-- Files list -->
		{#if files.length > 0}
			<div class="section">
				<h3 class="section-title">Files ({files.length})</h3>
				<div class="files-list">
					{#each files as attachment}
						<div class="file-item">
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="16"
								height="16"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								class="file-icon"
							>
								<path
									d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"
								/>
								<polyline points="14 2 14 8 20 8" />
							</svg>
							<a
								href={getAttachmentUrl(taskId, attachment.filename)}
								class="file-name"
								target="_blank"
								rel="noopener"
							>
								{attachment.filename}
							</a>
							<span class="file-meta">{formatSize(attachment.size)}</span>
							<span class="file-date">{formatDate(attachment.created_at)}</span>
							<button
								class="delete-btn"
								onclick={() => handleDelete(attachment.filename)}
								title="Delete"
							>
								<svg
									xmlns="http://www.w3.org/2000/svg"
									width="14"
									height="14"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									stroke-width="2"
								>
									<polyline points="3 6 5 6 21 6" />
									<path
										d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"
									/>
								</svg>
							</button>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	{/if}
</div>

<!-- Lightbox modal -->
{#if lightboxImage}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div
		class="lightbox"
		onclick={closeLightbox}
		onkeydown={(e) => e.key === 'Escape' && closeLightbox()}
		role="dialog"
		aria-modal="true"
		tabindex="-1"
	>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="lightbox-content" onclick={(e) => e.stopPropagation()} role="presentation">
			<button class="lightbox-close" onclick={closeLightbox} aria-label="Close">
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="24"
					height="24"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
				>
					<line x1="18" y1="6" x2="6" y2="18" />
					<line x1="6" y1="6" x2="18" y2="18" />
				</svg>
			</button>
			<img src={lightboxImage} alt={lightboxFilename ?? 'Image'} />
			{#if lightboxFilename}
				<div class="lightbox-filename">{lightboxFilename}</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	.attachments-container {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	/* Upload area */
	.upload-area {
		border: 2px dashed var(--border-default);
		border-radius: var(--radius-lg);
		padding: var(--space-4);
		text-align: center;
		transition:
			border-color 0.2s,
			background 0.2s;
	}

	.upload-area.drag-over {
		border-color: var(--accent-primary);
		background: var(--accent-subtle);
	}

	.file-input {
		display: none;
	}

	.upload-label {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-2);
		cursor: pointer;
		color: var(--text-muted);
		font-size: var(--text-sm);
	}

	.upload-label:hover {
		color: var(--text-secondary);
	}

	.upload-label svg {
		opacity: 0.5;
	}

	/* Error message */
	.error-message {
		padding: var(--space-3);
		background: var(--status-danger-bg);
		border: 1px solid var(--status-danger);
		border-radius: var(--radius-md);
		color: var(--status-danger);
		font-size: var(--text-sm);
	}

	/* Loading state */
	.loading-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-3);
		padding: var(--space-8);
		color: var(--text-muted);
		font-size: var(--text-sm);
	}

	.spinner {
		width: 20px;
		height: 20px;
		border: 2px solid var(--border-default);
		border-top-color: var(--accent-primary);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	/* Empty state */
	.empty-state {
		padding: var(--space-8);
		text-align: center;
		color: var(--text-muted);
		font-size: var(--text-sm);
	}

	/* Section */
	.section {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	.section-title {
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		color: var(--text-muted);
	}

	/* Images grid */
	.images-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
		gap: var(--space-3);
	}

	.image-card {
		position: relative;
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		overflow: hidden;
	}

	.image-preview {
		display: block;
		width: 100%;
		aspect-ratio: 1;
		padding: 0;
		border: none;
		background: none;
		cursor: pointer;
	}

	.image-preview img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.image-info {
		padding: var(--space-2);
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.image-name {
		font-size: var(--text-xs);
		color: var(--text-primary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.image-meta {
		font-size: var(--text-2xs);
		color: var(--text-muted);
	}

	.image-card .delete-btn {
		position: absolute;
		top: var(--space-1);
		right: var(--space-1);
		padding: var(--space-1);
		background: rgba(0, 0, 0, 0.6);
		border: none;
		border-radius: var(--radius-sm);
		color: white;
		cursor: pointer;
		opacity: 0;
		transition: opacity 0.2s;
	}

	.image-card:hover .delete-btn {
		opacity: 1;
	}

	.image-card .delete-btn:hover {
		background: var(--status-danger);
	}

	/* Files list */
	.files-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.file-item {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
	}

	.file-icon {
		color: var(--text-muted);
		flex-shrink: 0;
	}

	.file-name {
		flex: 1;
		font-size: var(--text-sm);
		color: var(--accent-primary);
		text-decoration: none;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.file-name:hover {
		text-decoration: underline;
	}

	.file-meta,
	.file-date {
		font-size: var(--text-xs);
		color: var(--text-muted);
		flex-shrink: 0;
	}

	.file-item .delete-btn {
		padding: var(--space-1);
		background: none;
		border: none;
		color: var(--text-muted);
		cursor: pointer;
		border-radius: var(--radius-sm);
	}

	.file-item .delete-btn:hover {
		color: var(--status-danger);
		background: var(--status-danger-bg);
	}

	/* Lightbox */
	.lightbox {
		position: fixed;
		inset: 0;
		z-index: 1000;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(0, 0, 0, 0.9);
		padding: var(--space-4);
	}

	.lightbox-content {
		position: relative;
		max-width: 90vw;
		max-height: 90vh;
		display: flex;
		flex-direction: column;
		align-items: center;
	}

	.lightbox-content img {
		max-width: 100%;
		max-height: calc(90vh - 60px);
		object-fit: contain;
		border-radius: var(--radius-md);
	}

	.lightbox-close {
		position: absolute;
		top: -40px;
		right: 0;
		padding: var(--space-2);
		background: none;
		border: none;
		color: white;
		cursor: pointer;
		opacity: 0.7;
	}

	.lightbox-close:hover {
		opacity: 1;
	}

	.lightbox-filename {
		margin-top: var(--space-3);
		color: white;
		font-size: var(--text-sm);
		opacity: 0.7;
	}
</style>
