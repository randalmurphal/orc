/**
 * NewInitiativeModal - Minimal modal for creating a new initiative.
 * Uses the same Modal component and form styles as NewTaskModal.
 */

import { useState, useCallback } from 'react';
import { Modal } from './Modal';
import { Button } from '@/components/ui/Button';
import { initiativeClient } from '@/lib/client';
import { create } from '@bufbuild/protobuf';
import { toast } from '@/stores/uiStore';
import {
	CreateInitiativeRequestSchema,
	type Initiative,
} from '@/gen/orc/v1/initiative_pb';
import '../task-detail/TaskEditModal.css';

interface NewInitiativeModalProps {
	open: boolean;
	onClose: () => void;
	onCreate?: (initiative: Initiative) => void;
}

export function NewInitiativeModal({ open, onClose, onCreate }: NewInitiativeModalProps) {
	const [title, setTitle] = useState('');
	const [vision, setVision] = useState('');
	const [saving, setSaving] = useState(false);

	const handleClose = useCallback(() => {
		setTitle('');
		setVision('');
		onClose();
	}, [onClose]);

	const handleCreate = useCallback(async () => {
		if (!title.trim()) return;

		setSaving(true);
		try {
			const response = await initiativeClient.createInitiative(
				create(CreateInitiativeRequestSchema, {
					title: title.trim(),
					vision: vision.trim() || undefined,
				})
			);

			if (response.initiative) {
				toast.success(`Initiative ${response.initiative.id} created`);
				onCreate?.(response.initiative);
			}
			handleClose();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to create initiative');
		} finally {
			setSaving(false);
		}
	}, [title, vision, handleClose, onCreate]);

	return (
		<Modal open={open} onClose={handleClose} title="New Initiative" size="md">
			<div className="task-edit-form">
				<div className="form-group">
					<label htmlFor="initiative-title">Title</label>
					<input
						id="initiative-title"
						type="text"
						value={title}
						onChange={(e) => setTitle(e.target.value)}
						placeholder="e.g. User Authentication System"
						autoFocus
						onKeyDown={(e) => {
							if (e.key === 'Enter' && title.trim()) {
								handleCreate();
							}
						}}
					/>
				</div>

				<div className="form-group">
					<label htmlFor="initiative-vision">Vision (optional)</label>
					<textarea
						id="initiative-vision"
						value={vision}
						onChange={(e) => setVision(e.target.value)}
						placeholder="Describe the initiative's goals and scope..."
						rows={3}
					/>
					<span className="form-hint">
						The vision flows into all linked task prompts, keeping work aligned.
					</span>
				</div>

				<div className="form-actions">
					<Button variant="secondary" onClick={handleClose} disabled={saving}>
						Cancel
					</Button>
					<Button
						variant="primary"
						onClick={handleCreate}
						disabled={!title.trim() || saving}
						loading={saving}
					>
						Create Initiative
					</Button>
				</div>
			</div>
		</Modal>
	);
}
