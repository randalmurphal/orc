import { useState, useEffect, useCallback, useMemo, DragEvent, ChangeEvent } from 'react';
import { taskClient } from '@/lib/client';
import { create } from '@bufbuild/protobuf';
import {
	type Attachment,
	ListAttachmentsRequestSchema,
	DeleteAttachmentRequestSchema,
} from '@/gen/orc/v1/task_pb';
import { timestampToDate } from '@/lib/time';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { toast } from '@/stores/uiStore';
import { useCurrentProjectId } from '@/stores';
import './AttachmentsTab.css';

interface AttachmentsTabProps {
	taskId: string;
}

function formatSize(bytes: number | bigint): string {
	const numBytes = typeof bytes === 'bigint' ? Number(bytes) : bytes;
	if (numBytes < 1024) return `${numBytes} B`;
	if (numBytes < 1024 * 1024) return `${(numBytes / 1024).toFixed(1)} KB`;
	return `${(numBytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(date: Date | null): string {
	if (!date) return 'N/A';
	return date.toLocaleDateString(undefined, {
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
	});
}

// Get attachment URL (uses /files/ endpoint for binary file serving)
function getAttachmentUrl(taskId: string, filename: string, projectId?: string | null): string {
	const base = `/files/tasks/${taskId}/attachments/${encodeURIComponent(filename)}`;
	return projectId ? `${base}?project=${encodeURIComponent(projectId)}` : base;
}

export function AttachmentsTab({ taskId }: AttachmentsTabProps) {
	const projectId = useCurrentProjectId();
	const [attachments, setAttachments] = useState<Attachment[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [uploading, setUploading] = useState(false);
	const [dragOver, setDragOver] = useState(false);
	const [lightboxImage, setLightboxImage] = useState<string | null>(null);
	const [lightboxFilename, setLightboxFilename] = useState<string | null>(null);

	const loadAttachments = useCallback(async () => {
		if (!projectId) return;
		setLoading(true);
		setError(null);

		try {
			const response = await taskClient.listAttachments(
				create(ListAttachmentsRequestSchema, { projectId, taskId })
			);
			setAttachments(response.attachments);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load attachments');
		} finally {
			setLoading(false);
		}
	}, [projectId, taskId]);

	useEffect(() => {
		loadAttachments();
	}, [loadAttachments]);

	const handleUpload = useCallback(
		async (files: FileList | null) => {
			if (!files || files.length === 0) return;

			setUploading(true);
			setError(null);

			try {
				// Upload uses /files/ endpoint for multipart file upload
				for (const file of files) {
					const formData = new FormData();
					formData.append('file', file);
					const uploadUrl = projectId
						? `/files/tasks/${taskId}/attachments?project=${encodeURIComponent(projectId)}`
						: `/files/tasks/${taskId}/attachments`;
					const res = await fetch(uploadUrl, {
						method: 'POST',
						body: formData,
					});
					if (!res.ok) {
						throw new Error(`Upload failed: ${res.statusText}`);
					}
				}
				await loadAttachments();
				toast.success(`${files.length} file${files.length > 1 ? 's' : ''} uploaded`);
			} catch (e) {
				setError(e instanceof Error ? e.message : 'Upload failed');
				toast.error('Upload failed');
			} finally {
				setUploading(false);
			}
		},
		[projectId, taskId, loadAttachments]
	);

	const handleDelete = useCallback(
		async (filename: string) => {
			if (!projectId || !confirm(`Delete "${filename}"?`)) return;

			try {
				await taskClient.deleteAttachment(
					create(DeleteAttachmentRequestSchema, { projectId, taskId, filename })
				);
				setAttachments((prev) => prev.filter((a) => a.filename !== filename));
				toast.success('Attachment deleted');
			} catch (e) {
				setError(e instanceof Error ? e.message : 'Delete failed');
				toast.error('Delete failed');
			}
		},
		[projectId, taskId]
	);

	const handleDragOver = useCallback((e: DragEvent<HTMLDivElement>) => {
		e.preventDefault();
		setDragOver(true);
	}, []);

	const handleDragLeave = useCallback(() => {
		setDragOver(false);
	}, []);

	const handleDrop = useCallback(
		(e: DragEvent<HTMLDivElement>) => {
			e.preventDefault();
			setDragOver(false);
			handleUpload(e.dataTransfer?.files ?? null);
		},
		[handleUpload]
	);

	const handleFileInputChange = useCallback(
		(e: ChangeEvent<HTMLInputElement>) => {
			handleUpload(e.currentTarget.files);
			// Reset the input so the same file can be uploaded again
			e.currentTarget.value = '';
		},
		[handleUpload]
	);

	const openLightbox = useCallback(
		(filename: string) => {
			setLightboxImage(getAttachmentUrl(taskId, filename, projectId));
			setLightboxFilename(filename);
		},
		[projectId, taskId]
	);

	const closeLightbox = useCallback(() => {
		setLightboxImage(null);
		setLightboxFilename(null);
	}, []);

	// Handle escape key for lightbox
	useEffect(() => {
		const handleKeyDown = (e: KeyboardEvent) => {
			if (e.key === 'Escape' && lightboxImage) {
				closeLightbox();
			}
		};
		document.addEventListener('keydown', handleKeyDown);
		return () => document.removeEventListener('keydown', handleKeyDown);
	}, [lightboxImage, closeLightbox]);

	// Split attachments into images and files
	const images = useMemo(() => attachments.filter((a) => a.isImage), [attachments]);
	const files = useMemo(() => attachments.filter((a) => !a.isImage), [attachments]);

	return (
		<div className="attachments-container">
			{/* Upload area */}
			<div
				className={`upload-area ${dragOver ? 'drag-over' : ''}`}
				onDragOver={handleDragOver}
				onDragLeave={handleDragLeave}
				onDrop={handleDrop}
				role="region"
				aria-label="File upload area"
			>
				<input
					type="file"
					id="file-upload"
					multiple
					onChange={handleFileInputChange}
					className="file-input"
				/>
				<label htmlFor="file-upload" className="upload-label">
					<Icon name="upload" size={24} />
					{uploading ? <span>Uploading...</span> : <span>Drop files here or click to upload</span>}
				</label>
			</div>

			{error && <div className="error-message">{error}</div>}

			{loading ? (
				<div className="loading-state">
					<div className="spinner" />
					<span>Loading attachments...</span>
				</div>
			) : attachments.length === 0 ? (
				<div className="empty-state">
					<p>No attachments yet</p>
				</div>
			) : (
				<>
					{/* Images gallery */}
					{images.length > 0 && (
						<div className="section">
							<h3 className="section-title">Images ({images.length})</h3>
							<div className="images-grid">
								{images.map((attachment) => (
									<div key={attachment.filename} className="image-card">
										<Button
											variant="ghost"
											className="image-preview"
											onClick={() => openLightbox(attachment.filename)}
											title="Click to enlarge"
											aria-label={`Preview ${attachment.filename}`}
										>
											<img
												src={getAttachmentUrl(taskId, attachment.filename, projectId)}
												alt={attachment.filename}
												loading="lazy"
											/>
										</Button>
										<div className="image-info">
											<span className="image-name" title={attachment.filename}>
												{attachment.filename}
											</span>
											<span className="image-meta">{formatSize(attachment.size)}</span>
										</div>
										<Button
											variant="ghost"
											iconOnly
											size="sm"
											className="delete-btn"
											onClick={() => handleDelete(attachment.filename)}
											title="Delete"
											aria-label="Delete attachment"
										>
											<Icon name="trash" size={14} />
										</Button>
									</div>
								))}
							</div>
						</div>
					)}

					{/* Files list */}
					{files.length > 0 && (
						<div className="section">
							<h3 className="section-title">Files ({files.length})</h3>
							<div className="files-list">
								{files.map((attachment) => (
									<div key={attachment.filename} className="file-item">
										<Icon name="file" size={16} className="file-icon" />
										<a
											href={getAttachmentUrl(taskId, attachment.filename, projectId)}
											className="file-name"
											target="_blank"
											rel="noopener noreferrer"
										>
											{attachment.filename}
										</a>
										<span className="file-meta">{formatSize(attachment.size)}</span>
										<span className="file-date">{formatDate(timestampToDate(attachment.createdAt))}</span>
										<Button
											variant="ghost"
											iconOnly
											size="sm"
											className="delete-btn"
											onClick={() => handleDelete(attachment.filename)}
											title="Delete"
											aria-label="Delete file"
										>
											<Icon name="trash" size={14} />
										</Button>
									</div>
								))}
							</div>
						</div>
					)}
				</>
			)}

			{/* Lightbox modal */}
			{lightboxImage && (
				<div
					className="lightbox"
					onClick={closeLightbox}
					role="dialog"
					aria-modal="true"
					tabIndex={-1}
				>
					<div className="lightbox-content" onClick={(e) => e.stopPropagation()}>
						<Button variant="ghost" iconOnly className="lightbox-close" onClick={closeLightbox} aria-label="Close">
							<Icon name="x" size={24} />
						</Button>
						<img src={lightboxImage} alt={lightboxFilename ?? 'Image'} />
						{lightboxFilename && <div className="lightbox-filename">{lightboxFilename}</div>}
					</div>
				</div>
			)}
		</div>
	);
}
