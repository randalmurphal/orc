/**
 * Skills page (/environment/skills)
 * Full CRUD for skills stored in GlobalDB skills table.
 */

import { useState, useEffect, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Icon } from '@/components/ui/Icon';
import { Modal } from '@/components/overlays/Modal';
import { toast } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import { configClient } from '@/lib/client';
import {
	type Skill,
	ListSkillsRequestSchema,
	CreateSkillRequestSchema,
	UpdateSkillRequestSchema,
	DeleteSkillRequestSchema,
} from '@/gen/orc/v1/config_pb';
import './environment.css';

// Editor form state for creating/editing a skill
interface SkillFormState {
	name: string;
	description: string;
	content: string;
}

const defaultFormState: SkillFormState = {
	name: '',
	description: '',
	content: '',
};

export function Skills() {
	useDocumentTitle('Skills');
	const [skills, setSkills] = useState<Skill[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Editor modal state
	const [editingSkill, setEditingSkill] = useState<Skill | null>(null);
	const [isCreating, setIsCreating] = useState(false);
	const [formState, setFormState] = useState<SkillFormState>(defaultFormState);
	const [saving, setSaving] = useState(false);

	// Preview modal state
	const [previewingSkill, setPreviewingSkill] = useState<Skill | null>(null);

	const loadSkills = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const response = await configClient.listSkills(
				create(ListSkillsRequestSchema, {})
			);
			setSkills(response.skills);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load skills');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadSkills();
	}, [loadSkills]);

	const handleCreate = () => {
		setFormState(defaultFormState);
		setIsCreating(true);
		setEditingSkill(null);
	};

	const handleEdit = (skill: Skill) => {
		setFormState({
			name: skill.name,
			description: skill.description,
			content: skill.content,
		});
		setEditingSkill(skill);
		setIsCreating(false);
	};

	const handleClone = (skill: Skill) => {
		setFormState({
			name: skill.name + '-copy',
			description: skill.description,
			content: skill.content,
		});
		setEditingSkill(null);
		setIsCreating(true);
	};

	const handleClose = () => {
		setEditingSkill(null);
		setIsCreating(false);
		setFormState(defaultFormState);
	};

	const handleFormChange = (field: keyof SkillFormState, value: string) => {
		setFormState((prev) => ({ ...prev, [field]: value }));
	};

	const handleSave = async () => {
		if (!formState.name.trim() || !formState.content.trim()) {
			toast.error('Name and content are required');
			return;
		}

		try {
			setSaving(true);

			if (isCreating) {
				await configClient.createSkill(
					create(CreateSkillRequestSchema, {
						name: formState.name.trim(),
						description: formState.description.trim(),
						content: formState.content.trim(),
					})
				);
				toast.success('Skill created');
			} else if (editingSkill) {
				await configClient.updateSkill(
					create(UpdateSkillRequestSchema, {
						id: editingSkill.id,
						name: formState.name.trim(),
						description: formState.description.trim(),
						content: formState.content.trim(),
					})
				);
				toast.success('Skill updated');
			}

			handleClose();
			await loadSkills();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save skill');
		} finally {
			setSaving(false);
		}
	};

	const handleDelete = async (skill: Skill) => {
		if (!confirm(`Delete skill "${skill.name}"?`)) {
			return;
		}
		try {
			await configClient.deleteSkill(
				create(DeleteSkillRequestSchema, {
					id: skill.id,
				})
			);
			toast.success('Skill deleted');
			await loadSkills();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete skill');
		}
	};

	const handlePreview = (skill: Skill) => {
		setPreviewingSkill(skill);
	};

	if (loading) {
		return (
			<div className="page environment-skills-page">
				<div className="env-loading">Loading skills...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="page environment-skills-page">
				<div className="env-error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadSkills}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	const isEditorOpen = isCreating || editingSkill !== null;

	return (
		<div className="page environment-skills-page">
			<div className="env-page-header">
				<div>
					<h3>Skills</h3>
					<p className="env-page-description">
						Manage skills that provide specialized capabilities to Claude Code.
					</p>
				</div>
				<Button variant="primary" size="sm" onClick={handleCreate}>
					<Icon name="plus" size={14} />
					Add Skill
				</Button>
			</div>

			{skills.length === 0 ? (
				<div className="env-empty">
					<Icon name="book" size={48} />
					<p>No skills found</p>
					<p className="env-empty-hint">
						Click "Add Skill" to create your first skill.
					</p>
				</div>
			) : (
				<div className="env-card-grid">
					{skills.map((skill) => (
						<div
							key={skill.id}
							className="env-card skill-card"
							onClick={() => handlePreview(skill)}
						>
							<div className="env-card-header">
								<h4 className="env-card-title">
									<Icon name="book" size={16} />
									{skill.name}
								</h4>
								<div className="hook-item-actions" onClick={(e) => e.stopPropagation()}>
									{!skill.isBuiltin && (
										<Button
											variant="ghost"
											size="sm"
											iconOnly
											onClick={() => handleEdit(skill)}
											aria-label="Edit"
										>
											<Icon name="edit" size={14} />
										</Button>
									)}
									<Button
										variant="ghost"
										size="sm"
										iconOnly
										onClick={() => handleClone(skill)}
										aria-label="Clone"
									>
										<Icon name="copy" size={14} />
									</Button>
									{!skill.isBuiltin && (
										<Button
											variant="ghost"
											size="sm"
											iconOnly
											onClick={() => handleDelete(skill)}
											aria-label="Delete"
										>
											<Icon name="trash" size={14} />
										</Button>
									)}
								</div>
							</div>
							<p className="env-card-description">{skill.description}</p>
							<div className="skill-card-meta">
								{skill.isBuiltin && (
									<span className="skill-card-badge">Built-in</span>
								)}
							</div>
						</div>
					))}
				</div>
			)}

			{/* Preview Modal */}
			<Modal
				open={previewingSkill !== null}
				onClose={() => setPreviewingSkill(null)}
				title={
					<div className="skill-preview-title">
						<Icon name="book" size={20} />
						<span>{previewingSkill?.name}</span>
					</div>
				}
				size="lg"
			>
				{previewingSkill ? (
					<div className="skill-preview">
						<div className="skill-preview-meta">
							<div className="skill-preview-description">
								{previewingSkill.description}
							</div>
						</div>

						<div className="skill-preview-flags">
							{previewingSkill.isBuiltin && (
								<span className="skill-preview-flag">
									<Icon name="shield" size={12} />
									Built-in
								</span>
							)}
						</div>

						<div className="skill-preview-section">
							<h5>Content</h5>
							<pre className="skill-preview-content">{previewingSkill.content}</pre>
						</div>
					</div>
				) : null}
			</Modal>

			{/* Editor Modal */}
			<Modal
				open={isEditorOpen}
				onClose={handleClose}
				title={isCreating ? 'Create Skill' : `Edit Skill: ${editingSkill?.name}`}
				size="lg"
			>
				<div className="hooks-editor">
					<div className="hooks-editor-field">
						<label htmlFor="skill-name">Name</label>
						<Input
							id="skill-name"
							value={formState.name}
							onChange={(e) => handleFormChange('name', e.target.value)}
							placeholder="my-skill"
							size="sm"
						/>
					</div>

					<div className="hooks-editor-field">
						<label htmlFor="skill-description">Description</label>
						<Input
							id="skill-description"
							value={formState.description}
							onChange={(e) => handleFormChange('description', e.target.value)}
							placeholder="What this skill does"
							size="sm"
						/>
					</div>

					<div className="hooks-editor-field">
						<label htmlFor="skill-content">Content (markdown)</label>
						<textarea
							id="skill-content"
							className="input-field"
							value={formState.content}
							onChange={(e) => handleFormChange('content', e.target.value)}
							placeholder="# Skill content&#10;&#10;Describe the skill behavior..."
							rows={12}
							style={{ fontFamily: 'var(--font-mono)', fontSize: '0.85rem', padding: 'var(--space-2)', resize: 'vertical' }}
						/>
					</div>

					<div className="hooks-editor-actions">
						<Button variant="secondary" onClick={handleClose}>
							Cancel
						</Button>
						<Button variant="primary" onClick={handleSave} loading={saving}>
							{isCreating ? 'Create Skill' : 'Save Changes'}
						</Button>
					</div>
				</div>
			</Modal>
		</div>
	);
}
