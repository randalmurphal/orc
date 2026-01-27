/**
 * NewCommandModal - Modal for creating new slash commands.
 *
 * Features:
 * - Name input (required)
 * - Description input (optional)
 * - Scope selector (project/global)
 * - Creates skill via configClient.createSkill()
 */

import { useState, useCallback, useEffect } from 'react';
import { Modal } from '@/components/overlays/Modal';
import { Button } from '@/components/ui/Button';
import { configClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import type { Skill } from '@/gen/orc/v1/config_pb';
import { SettingsScope } from '@/gen/orc/v1/config_pb';
import '../task-detail/TaskEditModal.css';

interface NewCommandModalProps {
	open: boolean;
	onClose: () => void;
	onCreate?: (skill: Skill) => void;
}

const SCOPE_OPTIONS = [
	{ value: SettingsScope.GLOBAL, label: 'Global (~/.claude/commands/)' },
	{ value: SettingsScope.PROJECT, label: 'Project (.claude/commands/)' },
] as const;

export function NewCommandModal({ open, onClose, onCreate }: NewCommandModalProps) {
	const [name, setName] = useState('');
	const [description, setDescription] = useState('');
	const [scope, setScope] = useState<SettingsScope>(SettingsScope.GLOBAL);
	const [saving, setSaving] = useState(false);

	// Reset form when modal opens
	useEffect(() => {
		if (open) {
			setName('');
			setDescription('');
			setScope(SettingsScope.GLOBAL);
		}
	}, [open]);

	const handleCreate = useCallback(async () => {
		if (!name.trim()) {
			toast.error('Name is required');
			return;
		}

		setSaving(true);
		try {
			const response = await configClient.createSkill({
				name: name.trim(),
				description: description.trim(),
				content: `# ${name.trim()}\n\n<!-- Command content here -->`,
				userInvocable: true,
				scope,
			});

			if (response.skill) {
				toast.success(`Command /${response.skill.name} created`);
				onCreate?.(response.skill);
			}
			onClose();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to create command');
		} finally {
			setSaving(false);
		}
	}, [name, description, scope, onCreate, onClose]);

	// Handle Enter key to submit
	const handleKeyDown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' && !e.shiftKey && name.trim()) {
				e.preventDefault();
				handleCreate();
			}
		},
		[handleCreate, name]
	);

	return (
		<Modal open={open} title="New Command" onClose={onClose}>
			<div className="task-edit-form">
				{/* Name */}
				<div className="form-group">
					<label htmlFor="new-command-name">Name</label>
					<input
						id="new-command-name"
						type="text"
						value={name}
						onChange={(e) => setName(e.target.value)}
						onKeyDown={handleKeyDown}
						placeholder="my-command"
						autoFocus
					/>
					<span className="form-hint">The command will be invoked as /{name || 'name'}</span>
				</div>

				{/* Description */}
				<div className="form-group">
					<label htmlFor="new-command-description">Description</label>
					<input
						id="new-command-description"
						type="text"
						value={description}
						onChange={(e) => setDescription(e.target.value)}
						placeholder="Optional description"
					/>
				</div>

				{/* Scope */}
				<div className="form-group">
					<label htmlFor="new-command-scope">Scope</label>
					<select
						id="new-command-scope"
						value={scope}
						onChange={(e) => setScope(Number(e.target.value) as SettingsScope)}
					>
						{SCOPE_OPTIONS.map((opt) => (
							<option key={opt.value} value={opt.value}>
								{opt.label}
							</option>
						))}
					</select>
					<span className="form-hint">
						Global commands are available across all projects
					</span>
				</div>

				{/* Actions */}
				<div className="form-actions">
					<Button type="button" variant="secondary" onClick={onClose}>
						Cancel
					</Button>
					<Button
						type="button"
						variant="primary"
						onClick={handleCreate}
						disabled={!name.trim()}
						loading={saving}
					>
						Create
					</Button>
				</div>
			</div>
		</Modal>
	);
}
