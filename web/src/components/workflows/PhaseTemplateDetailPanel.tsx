/**
 * PhaseTemplateDetailPanel - Shows detailed information about a selected phase template.
 *
 * Features:
 * - Displays phase template name, description, and configuration
 * - Shows prompt source and configuration
 * - Shows input variables
 * - Clone/Edit/Delete actions for custom templates (built-ins are read-only)
 */

import { useState, useCallback } from 'react';
import { RightPanel } from '@/components/layout/RightPanel';
import { Button, Icon } from '@/components/ui';
import { workflowClient } from '@/lib/client';
import type { PhaseTemplate, DefinitionSource } from '@/gen/orc/v1/workflow_pb';
import { DefinitionSource as DS, GateType } from '@/gen/orc/v1/workflow_pb';
import './PhaseTemplateDetailPanel.css';

export interface PhaseTemplateDetailPanelProps {
	/** The phase template to display */
	template: PhaseTemplate | null;
	/** The definition source of the template */
	source?: DefinitionSource;
	/** Whether the panel is open */
	isOpen: boolean;
	/** Callback when panel should close */
	onClose: () => void;
	/** Callback when clone action is triggered */
	onClone: (template: PhaseTemplate) => void;
	/** Callback when edit action is triggered */
	onEdit: (template: PhaseTemplate) => void;
	/** Callback when template is deleted */
	onDeleted: (id: string) => void;
}

/** Get display name for a gate type */
function getGateTypeLabel(gateType: GateType): string {
	switch (gateType) {
		case GateType.AUTO:
			return 'Auto';
		case GateType.HUMAN:
			return 'Human';
		case GateType.SKIP:
			return 'Skip';
		default:
			return 'Unknown';
	}
}

/** Get display name for a definition source */
function getSourceLabel(source?: DefinitionSource): string {
	switch (source) {
		case DS.EMBEDDED:
			return 'Built-in';
		case DS.PROJECT:
			return 'Project';
		case DS.SHARED:
			return 'Shared';
		case DS.LOCAL:
			return 'Local';
		case DS.PERSONAL:
			return 'Personal';
		default:
			return 'Unknown';
	}
}

/** Check if source is file-based (editable) */
function isEditableSource(source?: DefinitionSource): boolean {
	return (
		source === DS.PROJECT ||
		source === DS.SHARED ||
		source === DS.LOCAL ||
		source === DS.PERSONAL
	);
}

/**
 * PhaseTemplateDetailPanel displays detailed information about a phase template.
 */
export function PhaseTemplateDetailPanel({
	template,
	source,
	isOpen,
	onClose,
	onClone,
	onEdit,
	onDeleted,
}: PhaseTemplateDetailPanelProps) {
	const [deleting, setDeleting] = useState(false);
	const [error, setError] = useState<string | null>(null);

	const isBuiltin = source === DS.EMBEDDED || template?.isBuiltin;
	const canEdit = isEditableSource(source);

	const handleDelete = useCallback(async () => {
		if (!template || isBuiltin) return;

		const confirmed = window.confirm(
			`Delete phase template "${template.name}"? This cannot be undone.`
		);
		if (!confirmed) return;

		setDeleting(true);
		setError(null);
		try {
			await workflowClient.deletePhaseTemplate({ id: template.id });
			onDeleted(template.id);
			onClose();
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to delete template');
		} finally {
			setDeleting(false);
		}
	}, [template, isBuiltin, onDeleted, onClose]);

	const handleClone = useCallback(() => {
		if (template) {
			onClone(template);
		}
	}, [template, onClone]);

	const handleEdit = useCallback(() => {
		if (template && canEdit) {
			onEdit(template);
		}
	}, [template, canEdit, onEdit]);

	if (!template) {
		return null;
	}

	return (
		<RightPanel isOpen={isOpen} onClose={onClose}>
			{/* Header Section */}
			<RightPanel.Section id="template-header">
				<div className="template-detail-header">
					<div className="template-detail-header-icon">
						<Icon name="file-text" size={20} />
					</div>
					<div className="template-detail-header-info">
						<h2 className="template-detail-title">{template.name}</h2>
						<code className="template-detail-id">{template.id}</code>
					</div>
					<span
						className={`template-detail-badge ${isBuiltin ? 'builtin' : 'custom'}`}
					>
						{getSourceLabel(source)}
					</span>
				</div>

				{template.description && (
					<p className="template-detail-description">{template.description}</p>
				)}

				{/* Configuration Info */}
				<div className="template-detail-meta">
					<span className="template-detail-meta-item">
						<Icon name="shield" size={12} />
						Gate: {getGateTypeLabel(template.gateType)}
					</span>
					<span className="template-detail-meta-item">
						<Icon name="refresh" size={12} />
						Max iterations: {template.maxIterations}
					</span>
					{template.agentId && (
						<span className="template-detail-meta-item">
							<Icon name="cpu" size={12} />
							Agent: {template.agentId}
						</span>
					)}
					{template.thinkingEnabled && (
						<span className="template-detail-meta-item">
							<Icon name="brain" size={12} />
							Thinking
						</span>
					)}
					{template.checkpoint && (
						<span className="template-detail-meta-item">
							<Icon name="check-circle" size={12} />
							Checkpoint
						</span>
					)}
				</div>

				{/* Actions */}
				<div className="template-detail-actions">
					{canEdit && (
						<Button
							variant="primary"
							size="sm"
							leftIcon={<Icon name="edit" size={12} />}
							onClick={handleEdit}
						>
							Edit
						</Button>
					)}
					<Button
						variant="secondary"
						size="sm"
						leftIcon={<Icon name="copy" size={12} />}
						onClick={handleClone}
					>
						Clone
					</Button>
					{canEdit && (
						<Button
							variant="danger"
							size="sm"
							leftIcon={<Icon name="trash" size={12} />}
							onClick={handleDelete}
							disabled={deleting}
						>
							{deleting ? 'Deleting...' : 'Delete'}
						</Button>
					)}
				</div>

				{/* Error State */}
				{error && (
					<div className="template-detail-error">
						<Icon name="alert-circle" size={14} />
						<span>{error}</span>
					</div>
				)}
			</RightPanel.Section>

			{/* Prompt Section */}
			<RightPanel.Section id="template-prompt" defaultCollapsed={false}>
				<RightPanel.Header
					title="Prompt"
					icon="file-text"
					iconColor="blue"
				/>
				<RightPanel.Body>
					<div className="template-detail-prompt">
						<div className="template-detail-prompt-source">
							<span className="template-detail-prompt-label">Source:</span>
							<code className="template-detail-prompt-value">
								{template.promptSource}
							</code>
						</div>
						{template.promptPath && (
							<div className="template-detail-prompt-path">
								<span className="template-detail-prompt-label">Path:</span>
								<code className="template-detail-prompt-value">
									{template.promptPath}
								</code>
							</div>
						)}
					</div>
				</RightPanel.Body>
			</RightPanel.Section>

			{/* Input Variables Section */}
			{template.inputVariables && template.inputVariables.length > 0 && (
				<RightPanel.Section id="template-variables" defaultCollapsed={true}>
					<RightPanel.Header
						title="Input Variables"
						icon="code"
						iconColor="purple"
						count={template.inputVariables.length}
						badgeColor="purple"
					/>
					<RightPanel.Body>
						<div className="template-detail-variables">
							{template.inputVariables.map((varName) => (
								<code key={varName} className="template-detail-variable">
									{varName}
								</code>
							))}
						</div>
					</RightPanel.Body>
				</RightPanel.Section>
			)}

			{/* Artifact Section */}
			{template.producesArtifact && (
				<RightPanel.Section id="template-artifact" defaultCollapsed={true}>
					<RightPanel.Header
						title="Artifact"
						icon="box"
						iconColor="green"
					/>
					<RightPanel.Body>
						<div className="template-detail-artifact">
							<span className="template-detail-artifact-label">Type:</span>
							<code className="template-detail-artifact-type">
								{template.artifactType || 'artifact'}
							</code>
						</div>
					</RightPanel.Body>
				</RightPanel.Section>
			)}
		</RightPanel>
	);
}
