/**
 * NewTaskModal - Create a new task
 *
 * Features:
 * - Title input (required)
 * - Description textarea
 * - Weight selector
 * - Category selector
 * - Priority selector
 * - Initiative selector
 * - Attachment upload with drag-drop and preview
 * - Keyboard shortcut: Cmd/Ctrl+Enter to submit
 */

import { useState, useEffect, useRef, useCallback, type ChangeEvent, type DragEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { Modal } from './Modal';
import { Icon } from '@/components/ui/Icon';
import { createProjectTask } from '@/lib/api';
import { useCurrentProjectId } from '@/stores';
import { useInitiatives, useCurrentInitiativeId } from '@/stores/initiativeStore';
import { useUIStore } from '@/stores/uiStore';
import type { TaskWeight, TaskCategory, TaskPriority } from '@/lib/types';
import './NewTaskModal.css';

interface NewTaskModalProps {
	open: boolean;
	onClose: () => void;
}

const WEIGHTS: { value: TaskWeight; label: string; description: string }[] = [
	{ value: 'trivial', label: 'Trivial', description: 'Quick fix, one-liner' },
	{ value: 'small', label: 'Small', description: 'Bug fix, minor change' },
	{ value: 'medium', label: 'Medium', description: 'Feature with tests' },
	{ value: 'large', label: 'Large', description: 'Complex feature' },
	{ value: 'greenfield', label: 'Greenfield', description: 'New system from scratch' },
];

const CATEGORIES: { value: TaskCategory; label: string; icon: string }[] = [
	{ value: 'feature', label: 'Feature', icon: '✨' },
	{ value: 'bug', label: 'Bug', icon: '🐛' },
	{ value: 'refactor', label: 'Refactor', icon: '♻️' },
	{ value: 'chore', label: 'Chore', icon: '🔧' },
	{ value: 'docs', label: 'Docs', icon: '📝' },
	{ value: 'test', label: 'Test', icon: '🧪' },
];

const PRIORITIES: { value: TaskPriority; label: string }[] = [
	{ value: 'critical', label: 'Critical' },
	{ value: 'high', label: 'High' },
	{ value: 'normal', label: 'Normal' },
	{ value: 'low', label: 'Low' },
];

// Accepted file types
const ACCEPTED_TYPES = [
	'image/*',
	'application/pdf',
	'text/markdown',
	'text/plain',
	'application/json',
];

function formatFileSize(bytes: number): string {
	if (bytes < 1024) return `${bytes} B`;
	if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
	return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function isImageFile(file: File): boolean {
	return file.type.startsWith('image/');
}

export function NewTaskModal({ open, onClose }: NewTaskModalProps) {
	const navigate = useNavigate();
	const projectId = useCurrentProjectId();
	const currentInitiativeId = useCurrentInitiativeId();
	const initiatives = useInitiatives();
	const toast = useUIStore((s) => s.toast);

	// Form state
	const [title, setTitle] = useState('');
	const [description, setDescription] = useState('');
	const [weight, setWeight] = useState<TaskWeight>('medium');
	const [category, setCategory] = useState<TaskCategory>('feature');
	const [priority, setPriority] = useState<TaskPriority>('normal');
	const [initiativeId, setInitiativeId] = useState<string>('');
	const [attachments, setAttachments] = useState<File[]>([]);

	// UI state
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [isDragOver, setIsDragOver] = useState(false);

	// Refs
	const titleInputRef = useRef<HTMLInputElement>(null);
	const fileInputRef = useRef<HTMLInputElement>(null);

	// Reset form when modal opens/closes
	useEffect(() => {
		if (open) {
			setTitle('');
			setDescription('');
			setWeight('medium');
			setCategory('feature');
			setPriority('normal');
			// Pre-select current initiative filter if set
			setInitiativeId(currentInitiativeId && currentInitiativeId !== '__unassigned__' ? currentInitiativeId : '');
			setAttachments([]);
			setError(null);
			// Focus title input
			setTimeout(() => titleInputRef.current?.focus(), 50);
		}
	}, [open, currentInitiativeId]);

	// Handle form submission
	const handleSubmit = useCallback(async () => {
		if (!title.trim()) {
			setError('Title is required');
			return;
		}

		if (!projectId) {
			setError('No project selected');
			return;
		}

		setLoading(true);
		setError(null);

		try {
			const task = await createProjectTask(
				projectId,
				title.trim(),
				description.trim() || undefined,
				weight,
				category,
				attachments.length > 0 ? attachments : undefined
			);

			toast.success(`Task ${task.id} created`);
			onClose();
			navigate(`/tasks/${task.id}`);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to create task');
		} finally {
			setLoading(false);
		}
	}, [title, description, weight, category, projectId, attachments, onClose, navigate]);

	// Handle Cmd/Ctrl+Enter shortcut
	const handleKeyDown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
				e.preventDefault();
				handleSubmit();
			}
		},
		[handleSubmit]
	);

	// Handle file selection
	const handleFileSelect = (e: ChangeEvent<HTMLInputElement>) => {
		const files = Array.from(e.target.files || []);
		setAttachments((prev) => [...prev, ...files]);
		// Reset input so same file can be selected again
		if (fileInputRef.current) {
			fileInputRef.current.value = '';
		}
	};

	// Handle drag events
	const handleDragOver = (e: DragEvent) => {
		e.preventDefault();
		setIsDragOver(true);
	};

	const handleDragLeave = (e: DragEvent) => {
		e.preventDefault();
		setIsDragOver(false);
	};

	const handleDrop = (e: DragEvent) => {
		e.preventDefault();
		setIsDragOver(false);
		const files = Array.from(e.dataTransfer.files);
		setAttachments((prev) => [...prev, ...files]);
	};

	// Remove attachment
	const removeAttachment = (index: number) => {
		setAttachments((prev) => prev.filter((_, i) => i !== index));
	};

	// Active initiatives for selector
	const activeInitiatives = initiatives.filter((i) => i.status !== 'archived');

	return (
		<Modal open={open} onClose={onClose} title="Create New Task" size="lg">
			<div className="new-task-modal" onKeyDown={handleKeyDown}>
				{error && (
					<div className="error-banner" role="alert">
						<Icon name="close" size={16} />
						<span>{error}</span>
					</div>
				)}

				<form onSubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
					{/* Title */}
					<div className="form-group">
						<label htmlFor="task-title" className="form-label">
							Title <span className="required">*</span>
						</label>
						<input
							ref={titleInputRef}
							id="task-title"
							type="text"
							className="form-input"
							value={title}
							onChange={(e) => setTitle(e.target.value)}
							placeholder="What needs to be done?"
							disabled={loading}
							autoComplete="off"
						/>
					</div>

					{/* Description */}
					<div className="form-group">
						<label htmlFor="task-description" className="form-label">
							Description
						</label>
						<textarea
							id="task-description"
							className="form-textarea"
							value={description}
							onChange={(e) => setDescription(e.target.value)}
							placeholder="Additional context, requirements, or notes..."
							rows={3}
							disabled={loading}
						/>
					</div>

					{/* Weight and Category row */}
					<div className="form-row">
						{/* Weight */}
						<div className="form-group">
							<label className="form-label">Weight</label>
							<div className="weight-selector">
								{WEIGHTS.map((w) => (
									<button
										key={w.value}
										type="button"
										className={`weight-option ${weight === w.value ? 'selected' : ''}`}
										onClick={() => setWeight(w.value)}
										disabled={loading}
										title={w.description}
									>
										{w.label}
									</button>
								))}
							</div>
						</div>

						{/* Priority */}
						<div className="form-group">
							<label htmlFor="task-priority" className="form-label">
								Priority
							</label>
							<select
								id="task-priority"
								className="form-select"
								value={priority}
								onChange={(e) => setPriority(e.target.value as TaskPriority)}
								disabled={loading}
							>
								{PRIORITIES.map((p) => (
									<option key={p.value} value={p.value}>
										{p.label}
									</option>
								))}
							</select>
						</div>
					</div>

					{/* Category */}
					<div className="form-group">
						<label className="form-label">Category</label>
						<div className="category-selector">
							{CATEGORIES.map((c) => (
								<label
									key={c.value}
									className={`category-option ${category === c.value ? 'selected' : ''}`}
								>
									<input
										type="radio"
										name="category"
										value={c.value}
										checked={category === c.value}
										onChange={() => setCategory(c.value)}
										disabled={loading}
									/>
									<span className="category-icon">{c.icon}</span>
									<span className="category-label">{c.label}</span>
								</label>
							))}
						</div>
					</div>

					{/* Initiative */}
					{activeInitiatives.length > 0 && (
						<div className="form-group">
							<label htmlFor="task-initiative" className="form-label">
								Initiative
							</label>
							<select
								id="task-initiative"
								className="form-select"
								value={initiativeId}
								onChange={(e) => setInitiativeId(e.target.value)}
								disabled={loading}
							>
								<option value="">None</option>
								{activeInitiatives.map((i) => (
									<option key={i.id} value={i.id}>
										{i.id}: {i.title}
									</option>
								))}
							</select>
						</div>
					)}

					{/* Attachments */}
					<div className="form-group">
						<label className="form-label">Attachments</label>
						<div
							className={`drop-zone ${isDragOver ? 'drag-over' : ''}`}
							onDragOver={handleDragOver}
							onDragLeave={handleDragLeave}
							onDrop={handleDrop}
							onClick={() => fileInputRef.current?.click()}
						>
							<Icon name="file" size={24} />
							<p>Drop files here or click to browse</p>
							<span className="drop-zone-hint">
								Images, PDFs, Markdown, JSON, log files
							</span>
							<input
								ref={fileInputRef}
								type="file"
								multiple
								accept={ACCEPTED_TYPES.join(',')}
								onChange={handleFileSelect}
								style={{ display: 'none' }}
								disabled={loading}
							/>
						</div>

						{attachments.length > 0 && (
							<ul className="attachment-list">
								{attachments.map((file, index) => (
									<li key={`${file.name}-${index}`} className="attachment-item">
										{isImageFile(file) ? (
											<img
												src={URL.createObjectURL(file)}
												alt={file.name}
												className="attachment-thumbnail"
											/>
										) : (
											<div className="attachment-icon">
												<Icon name="file" size={20} />
											</div>
										)}
										<div className="attachment-info">
											<span className="attachment-name">{file.name}</span>
											<span className="attachment-size">
												{formatFileSize(file.size)}
											</span>
										</div>
										<button
											type="button"
											className="attachment-remove"
											onClick={(e) => {
												e.stopPropagation();
												removeAttachment(index);
											}}
											aria-label={`Remove ${file.name}`}
											disabled={loading}
										>
											<Icon name="close" size={14} />
										</button>
									</li>
								))}
							</ul>
						)}
					</div>

					{/* Actions */}
					<div className="modal-actions">
						<button
							type="button"
							className="btn-secondary"
							onClick={onClose}
							disabled={loading}
						>
							Cancel
						</button>
						<button
							type="submit"
							className="btn-primary"
							disabled={loading || !title.trim()}
						>
							{loading ? (
								<>
									<span className="spinner" />
									Creating...
								</>
							) : (
								<>
									<Icon name="plus" size={16} />
									Create Task
								</>
							)}
						</button>
					</div>

					<div className="keyboard-hint">
						<kbd>⌘</kbd>+<kbd>Enter</kbd> to create
					</div>
				</form>
			</div>
		</Modal>
	);
}
