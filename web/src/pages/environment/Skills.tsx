/**
 * Skills page (/environment/skills)
 * Full CRUD for skills stored in GlobalDB skills table.
 */

import { useState, useEffect, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import * as Tabs from '@radix-ui/react-tabs';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Icon } from '@/components/ui/Icon';
import { Modal } from '@/components/overlays/Modal';
import { toast } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import { configClient } from '@/lib/client';
import {
	type Skill,
	type DiscoveredItem,
	ListSkillsRequestSchema,
	CreateSkillRequestSchema,
	UpdateSkillRequestSchema,
	DeleteSkillRequestSchema,
	ExportSkillsRequestSchema,
	ImportSkillsRequestSchema,
	ScanClaudeDirRequestSchema,
	SettingsScope,
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

	// Export/Import state
	const [exportDest, setExportDest] = useState<SettingsScope>(SettingsScope.PROJECT);
	const [selectedExportIds, setSelectedExportIds] = useState<Set<string>>(new Set());
	const [exporting, setExporting] = useState(false);
	const [scanSource, setScanSource] = useState<SettingsScope>(SettingsScope.PROJECT);
	const [scanning, setScanning] = useState(false);
	const [discoveredItems, setDiscoveredItems] = useState<DiscoveredItem[]>([]);
	const [selectedImportNames, setSelectedImportNames] = useState<Set<string>>(new Set());
	const [importing, setImporting] = useState(false);

	const handleExportSkills = async () => {
		if (selectedExportIds.size === 0) {
			toast.error('Select at least one skill to export');
			return;
		}
		try {
			setExporting(true);
			const resp = await configClient.exportSkills(
				create(ExportSkillsRequestSchema, {
					skillIds: Array.from(selectedExportIds),
					destination: exportDest,
				})
			);
			toast.success(`Exported ${resp.writtenPaths.length} file(s)`);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to export skills');
		} finally {
			setExporting(false);
		}
	};

	const handleScanSkills = async () => {
		try {
			setScanning(true);
			const resp = await configClient.scanClaudeDir(
				create(ScanClaudeDirRequestSchema, {
					source: scanSource,
				})
			);
			const skillItems = resp.items.filter((i) => i.itemType === 'skill');
			setDiscoveredItems(skillItems);
			setSelectedImportNames(new Set(skillItems.map((i) => i.name)));
			if (skillItems.length === 0) {
				toast.info('No new or modified skills found');
			}
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to scan directory');
		} finally {
			setScanning(false);
		}
	};

	const handleImportSkills = async () => {
		const itemsToImport = discoveredItems.filter((i) => selectedImportNames.has(i.name));
		if (itemsToImport.length === 0) {
			toast.error('Select at least one skill to import');
			return;
		}
		try {
			setImporting(true);
			const resp = await configClient.importSkills(
				create(ImportSkillsRequestSchema, {
					items: itemsToImport,
				})
			);
			toast.success(`Imported ${resp.imported.length} skill(s)`);
			setDiscoveredItems([]);
			setSelectedImportNames(new Set());
			await loadSkills();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to import skills');
		} finally {
			setImporting(false);
		}
	};

	const toggleExportId = (id: string) => {
		setSelectedExportIds((prev) => {
			const next = new Set(prev);
			if (next.has(id)) next.delete(id);
			else next.add(id);
			return next;
		});
	};

	const toggleImportName = (name: string) => {
		setSelectedImportNames((prev) => {
			const next = new Set(prev);
			if (next.has(name)) next.delete(name);
			else next.add(name);
			return next;
		});
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

			<Tabs.Root defaultValue="library">
				<Tabs.List className="env-scope-tabs" aria-label="Skills view">
					<Tabs.Trigger value="library">Library</Tabs.Trigger>
					<Tabs.Trigger value="export-import">Export / Import</Tabs.Trigger>
				</Tabs.List>

				<Tabs.Content value="library">
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
				</Tabs.Content>

				<Tabs.Content value="export-import">
					<div className="export-import-section">
						<div className="export-import-panel">
							<h4>Export Skills</h4>
							<p className="env-page-description">Export skills from the library to .claude/skills/ directory.</p>
							<div className="export-import-controls">
								<div className="export-dest-selector">
									<Button
										variant={exportDest === SettingsScope.PROJECT ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => setExportDest(SettingsScope.PROJECT)}
									>
										Project .claude/
									</Button>
									<Button
										variant={exportDest === SettingsScope.GLOBAL ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => setExportDest(SettingsScope.GLOBAL)}
									>
										User ~/.claude/
									</Button>
								</div>
							</div>
							{skills.length === 0 ? (
								<div className="hooks-empty">No skills in library to export</div>
							) : (
								<div className="export-import-list">
									{skills.map((skill) => (
										<label key={skill.id} className="export-import-item">
											<input
												type="checkbox"
												checked={selectedExportIds.has(skill.id)}
												onChange={() => toggleExportId(skill.id)}
											/>
											<span className="export-import-item-name">{skill.name}</span>
											{skill.isBuiltin && <span className="skill-card-badge">Built-in</span>}
										</label>
									))}
								</div>
							)}
							<Button variant="primary" size="sm" onClick={handleExportSkills} loading={exporting} disabled={selectedExportIds.size === 0}>
								Export Selected ({selectedExportIds.size})
							</Button>
						</div>

						<div className="export-import-panel">
							<h4>Import Skills</h4>
							<p className="env-page-description">Scan .claude/skills/ directory and import discovered skills.</p>
							<div className="export-import-controls">
								<div className="export-dest-selector">
									<Button
										variant={scanSource === SettingsScope.PROJECT ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => setScanSource(SettingsScope.PROJECT)}
									>
										Project .claude/
									</Button>
									<Button
										variant={scanSource === SettingsScope.GLOBAL ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => setScanSource(SettingsScope.GLOBAL)}
									>
										User ~/.claude/
									</Button>
								</div>
								<Button variant="secondary" size="sm" onClick={handleScanSkills} loading={scanning}>
									<Icon name="search" size={14} />
									Scan
								</Button>
							</div>
							{discoveredItems.length === 0 ? (
								<div className="hooks-empty">No items discovered. Click Scan to search.</div>
							) : (
								<div className="export-import-list">
									{discoveredItems.map((item) => (
										<label key={item.name} className="export-import-item">
											<input
												type="checkbox"
												checked={selectedImportNames.has(item.name)}
												onChange={() => toggleImportName(item.name)}
											/>
											<span className="export-import-item-name">{item.name}</span>
											<span className={`export-import-badge export-import-badge-${item.status}`}>
												{item.status}
											</span>
											{Object.keys(item.supportingFiles).length > 0 && (
												<span className="export-import-item-meta">
													+{Object.keys(item.supportingFiles).length} file(s)
												</span>
											)}
										</label>
									))}
								</div>
							)}
							{discoveredItems.length > 0 && (
								<Button variant="primary" size="sm" onClick={handleImportSkills} loading={importing} disabled={selectedImportNames.size === 0}>
									Import Selected ({selectedImportNames.size})
								</Button>
							)}
						</div>
					</div>
				</Tabs.Content>
			</Tabs.Root>

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
