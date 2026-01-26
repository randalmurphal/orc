/**
 * Prompts page (/environment/prompts)
 * Displays and edits phase prompts for orc execution
 */

import { useState, useEffect, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Textarea } from '@/components/ui/Textarea';
import { Icon } from '@/components/ui/Icon';
import { Modal } from '@/components/overlays/Modal';
import type { IconName } from '@/components/ui/Icon';
import { toast } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import { configClient } from '@/lib/client';
import {
	type PromptTemplate,
	ListPromptsRequestSchema,
	GetPromptRequestSchema,
	GetDefaultPromptRequestSchema,
	UpdatePromptRequestSchema,
	DeletePromptRequestSchema,
} from '@/gen/orc/v1/config_pb';
import './environment.css';

// Phase icons and descriptions
const PHASE_INFO: Record<string, { icon: IconName; description: string }> = {
	research: { icon: 'search', description: 'Research phase for understanding requirements' },
	spec: { icon: 'file-text', description: 'Specification writing phase' },
	implement: { icon: 'code', description: 'Implementation phase for writing code' },
	test: { icon: 'check', description: 'Testing phase for verification' },
	docs: { icon: 'book', description: 'Documentation writing phase' },
	validate: { icon: 'target', description: 'Validation phase for quality checks' },
	finalize: { icon: 'check-circle', description: 'Finalize phase for branch sync and merge' },
};

// Extract template variables from content (e.g., {{TASK_ID}}, {{SPEC_CONTENT}})
function extractVariables(content: string): string[] {
	const matches = content.match(/\{\{([A-Z_]+)\}\}/g);
	if (!matches) return [];
	// Remove duplicates and extract just the variable name
	return [...new Set(matches.map((m) => m.slice(2, -2)))];
}

// Derive source from isCustom flag
function getSource(isCustom: boolean): string {
	return isCustom ? 'project' : 'embedded';
}

export function Prompts() {
	useDocumentTitle('Prompts');
	const [prompts, setPrompts] = useState<PromptTemplate[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Editor modal state
	const [editingPhase, setEditingPhase] = useState<string | null>(null);
	const [editorContent, setEditorContent] = useState('');
	const [defaultContent, setDefaultContent] = useState('');
	const [editorLoading, setEditorLoading] = useState(false);
	const [saving, setSaving] = useState(false);

	// Preview modal state
	const [previewingPhase, setPreviewingPhase] = useState<string | null>(null);
	const [previewContent, setPreviewContent] = useState<PromptTemplate | null>(null);
	const [previewLoading, setPreviewLoading] = useState(false);

	const loadPrompts = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const response = await configClient.listPrompts(create(ListPromptsRequestSchema, {}));
			setPrompts(response.prompts);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load prompts');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadPrompts();
	}, [loadPrompts]);

	const handlePreview = async (phase: string) => {
		setPreviewingPhase(phase);
		setPreviewLoading(true);
		try {
			const response = await configClient.getPrompt(
				create(GetPromptRequestSchema, { phase })
			);
			setPreviewContent(response.prompt ?? null);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to load prompt');
			setPreviewingPhase(null);
		} finally {
			setPreviewLoading(false);
		}
	};

	const handleEdit = async (phase: string) => {
		setEditingPhase(phase);
		setEditorLoading(true);
		try {
			// Load both current and default content
			const [currentResponse, defaultResponse] = await Promise.all([
				configClient.getPrompt(create(GetPromptRequestSchema, { phase })),
				configClient.getDefaultPrompt(create(GetDefaultPromptRequestSchema, { phase })).catch(() => null),
			]);
			const currentContent = currentResponse.prompt?.content ?? '';
			setEditorContent(currentContent);
			setDefaultContent(defaultResponse?.prompt?.content ?? currentContent);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to load prompt');
			setEditingPhase(null);
		} finally {
			setEditorLoading(false);
		}
	};

	const handleSave = async () => {
		if (!editingPhase) return;
		try {
			setSaving(true);
			await configClient.updatePrompt(
				create(UpdatePromptRequestSchema, { phase: editingPhase, content: editorContent })
			);
			toast.success(`${editingPhase} prompt saved`);
			setEditingPhase(null);
			await loadPrompts();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save prompt');
		} finally {
			setSaving(false);
		}
	};

	const handleDelete = async (phase: string) => {
		if (!confirm(`Delete override for ${phase} prompt? This will restore the default.`)) {
			return;
		}
		try {
			await configClient.deletePrompt(create(DeletePromptRequestSchema, { phase }));
			toast.success(`${phase} prompt override deleted`);
			await loadPrompts();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete prompt');
		}
	};

	const handleResetToDefault = () => {
		setEditorContent(defaultContent);
	};

	if (loading) {
		return (
			<div className="page environment-prompts-page">
				<div className="env-loading">Loading prompts...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="page environment-prompts-page">
				<div className="env-error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadPrompts}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	const phaseInfo = editingPhase ? PHASE_INFO[editingPhase] : null;

	return (
		<div className="page environment-prompts-page">
			<div className="env-page-header">
				<h3>Phase Prompts</h3>
				<p className="env-page-description">
					Customize the prompts used during task execution phases. Override defaults to tailor
					behavior for your project.
				</p>
			</div>

			<div className="prompts-list">
				{prompts.map((prompt) => {
					const info = PHASE_INFO[prompt.phase] || {
						icon: 'file',
						description: 'Custom phase',
					};
					const variables = extractVariables(prompt.content);
					const source = getSource(prompt.isCustom);
					return (
						<div key={prompt.phase} className="prompt-item">
							<div className="prompt-item-header">
								<div className="prompt-item-title">
									<Icon name={info.icon} size={18} />
									<span className="prompt-phase-name">{prompt.phase}</span>
									{prompt.isCustom && (
										<span className="prompt-badge override">Override</span>
									)}
									<span className={`prompt-badge source-${source}`}>
										{source}
									</span>
								</div>
								<div className="prompt-item-actions">
									<Button
										variant="ghost"
										size="sm"
										onClick={() => handlePreview(prompt.phase)}
									>
										<Icon name="eye" size={14} />
										Preview
									</Button>
									<Button
										variant="ghost"
										size="sm"
										onClick={() => handleEdit(prompt.phase)}
									>
										<Icon name="edit" size={14} />
										Edit
									</Button>
									{prompt.isCustom && (
										<Button
											variant="ghost"
											size="sm"
											onClick={() => handleDelete(prompt.phase)}
										>
											<Icon name="trash" size={14} />
										</Button>
									)}
								</div>
							</div>
							<p className="prompt-item-description">{info.description}</p>
							{variables.length > 0 && (
								<div className="prompt-variables">
									<span className="prompt-variables-label">Variables:</span>
									{variables.map((v) => (
										<code key={v} className="prompt-variable">
											{`{{${v}}}`}
										</code>
									))}
								</div>
							)}
						</div>
					);
				})}
			</div>

			{/* Preview Modal */}
			<Modal
				open={previewingPhase !== null}
				onClose={() => setPreviewingPhase(null)}
				title={`${previewingPhase} Prompt`}
				size="lg"
			>
				{previewLoading ? (
					<div className="env-loading">Loading prompt...</div>
				) : previewContent ? (
					<div className="prompt-preview">
						<div className="prompt-preview-meta">
							<span className={`prompt-badge source-${getSource(previewContent.isCustom)}`}>
								{getSource(previewContent.isCustom)}
							</span>
							{extractVariables(previewContent.content).length > 0 && (
								<div className="prompt-variables">
									<span className="prompt-variables-label">Variables:</span>
									{extractVariables(previewContent.content).map((v) => (
										<code key={v} className="prompt-variable">
											{`{{${v}}}`}
										</code>
									))}
								</div>
							)}
						</div>
						<pre className="prompt-preview-content">{previewContent.content}</pre>
					</div>
				) : null}
			</Modal>

			{/* Editor Modal */}
			<Modal
				open={editingPhase !== null}
				onClose={() => setEditingPhase(null)}
				title={
					<div className="prompt-editor-title">
						{phaseInfo && <Icon name={phaseInfo.icon} size={20} />}
						<span>Edit {editingPhase} Prompt</span>
					</div>
				}
				size="lg"
			>
				{editorLoading ? (
					<div className="env-loading">Loading prompt...</div>
				) : (
					<div className="prompt-editor">
						<p className="prompt-editor-hint">
							Use template variables like <code>{`{{TASK_ID}}`}</code>,{' '}
							<code>{`{{TASK_TITLE}}`}</code>, <code>{`{{SPEC_CONTENT}}`}</code> to
							inject task context.
						</p>
						<Textarea
							value={editorContent}
							onChange={(e) => setEditorContent(e.target.value)}
							rows={20}
							className="prompt-editor-textarea"
							placeholder="Enter prompt template..."
						/>
						<div className="prompt-editor-actions">
							<Button variant="ghost" onClick={handleResetToDefault}>
								Reset to Default
							</Button>
							<div className="prompt-editor-actions-right">
								<Button variant="secondary" onClick={() => setEditingPhase(null)}>
									Cancel
								</Button>
								<Button variant="primary" onClick={handleSave} loading={saving}>
									Save Override
								</Button>
							</div>
						</div>
					</div>
				)}
			</Modal>
		</div>
	);
}
