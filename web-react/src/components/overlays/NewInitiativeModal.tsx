/**
 * NewInitiativeModal - Create a new initiative
 *
 * Features:
 * - Title input (required)
 * - Vision textarea
 * - Keyboard shortcut: Cmd/Ctrl+Enter to submit
 */

import { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Modal } from './Modal';
import { Icon } from '@/components/ui/Icon';
import { createInitiative } from '@/lib/api';
import { useInitiativeStore, useUIStore } from '@/stores';
import './NewInitiativeModal.css';

interface NewInitiativeModalProps {
	open: boolean;
	onClose: () => void;
}

export function NewInitiativeModal({ open, onClose }: NewInitiativeModalProps) {
	const navigate = useNavigate();
	const addInitiative = useInitiativeStore((s) => s.addInitiative);
	const toast = useUIStore((s) => s.toast);

	// Form state
	const [title, setTitle] = useState('');
	const [vision, setVision] = useState('');

	// UI state
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);

	// Refs
	const titleInputRef = useRef<HTMLInputElement>(null);

	// Reset form when modal opens
	useEffect(() => {
		if (open) {
			setTitle('');
			setVision('');
			setError(null);
			setTimeout(() => titleInputRef.current?.focus(), 50);
		}
	}, [open]);

	// Handle form submission
	const handleSubmit = useCallback(async () => {
		if (!title.trim()) {
			setError('Title is required');
			return;
		}

		setLoading(true);
		setError(null);

		try {
			const initiative = await createInitiative({
				title: title.trim(),
				vision: vision.trim() || undefined,
			});

			// Add to store
			addInitiative(initiative);

			toast.success(`Initiative ${initiative.id} created`);
			onClose();
			navigate(`/initiatives/${initiative.id}`);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to create initiative');
		} finally {
			setLoading(false);
		}
	}, [title, vision, addInitiative, onClose, navigate]);

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

	return (
		<Modal open={open} onClose={onClose} title="Create New Initiative" size="md">
			<div className="new-initiative-modal" onKeyDown={handleKeyDown}>
				{error && (
					<div className="error-banner" role="alert">
						<Icon name="close" size={16} />
						<span>{error}</span>
					</div>
				)}

				<form onSubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
					{/* Title */}
					<div className="form-group">
						<label htmlFor="initiative-title" className="form-label">
							Title <span className="required">*</span>
						</label>
						<input
							ref={titleInputRef}
							id="initiative-title"
							type="text"
							className="form-input"
							value={title}
							onChange={(e) => setTitle(e.target.value)}
							placeholder="Initiative name..."
							disabled={loading}
							autoComplete="off"
						/>
					</div>

					{/* Vision */}
					<div className="form-group">
						<label htmlFor="initiative-vision" className="form-label">
							Vision
						</label>
						<textarea
							id="initiative-vision"
							className="form-textarea"
							value={vision}
							onChange={(e) => setVision(e.target.value)}
							placeholder="What's the end goal? What will success look like?"
							rows={4}
							disabled={loading}
						/>
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
									Create Initiative
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
