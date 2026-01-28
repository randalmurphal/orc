/**
 * Constitution settings page (/settings/constitution)
 * Manages project-level principles and invariants for AI-assisted task execution.
 */

import { useState, useEffect, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { ConfigEditor } from '@/components/settings/ConfigEditor';
import { useDocumentTitle } from '@/hooks';
import { configClient } from '@/lib/client';
import {
	type Constitution,
	GetConstitutionRequestSchema,
	UpdateConstitutionRequestSchema,
	DeleteConstitutionRequestSchema,
} from '@/gen/orc/v1/config_pb';
import './Constitution.css';

const CONSTITUTION_TEMPLATE = `# Project Constitution

These rules guide all AI-assisted task execution. Invariants CANNOT be ignored or overridden.

## Priority Hierarchy
When rules conflict, higher priority wins:
1. Safety & correctness (invariants)
2. Security (invariants)
3. Existing patterns (defaults)
4. Performance (defaults)
5. Style (defaults)

## Invariants (MUST NOT violate)

**These are absolute rules. Violations block task completion. No exceptions.**

| ID | Rule | Verification | Why |
|----|------|--------------|-----|
| INV-1 | No silent error swallowing | Linting passes | Hides bugs |
| INV-2 | All public APIs have tests | Coverage check | Prevents regressions |
| INV-3 | Database is source of truth | No YAML task files | Consistency |

## Defaults (SHOULD follow)

**These are defaults. Can deviate with documented justification.**

| ID | Default | When to Deviate |
|----|---------|-----------------|
| DEF-1 | Functions < 50 lines | Complex state machines |
| DEF-2 | One file = one responsibility | Test helpers |

## Architectural Decisions

| Decision | Rationale | Pattern Location |
|----------|-----------|------------------|
| Repository pattern | Testability | \`internal/storage/\` |
`;

export function ConstitutionPage() {
	useDocumentTitle('Constitution');
	const [constitution, setConstitution] = useState<Constitution | null>(null);
	const [content, setContent] = useState('');
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);

	const loadConstitution = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const response = await configClient.getConstitution(
				create(GetConstitutionRequestSchema, {})
			);
			if (response.constitution) {
				setConstitution(response.constitution);
				setContent(response.constitution.content);
			} else {
				setConstitution(null);
				setContent('');
			}
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load constitution');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadConstitution();
	}, [loadConstitution]);

	const handleSave = useCallback(async () => {
		try {
			setSaving(true);
			setError(null);
			const response = await configClient.updateConstitution(
				create(UpdateConstitutionRequestSchema, { content })
			);
			if (response.constitution) {
				setConstitution(response.constitution);
			}
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to save constitution');
		} finally {
			setSaving(false);
		}
	}, [content]);

	const handleDelete = useCallback(async () => {
		if (!confirm('Are you sure you want to delete the constitution? This cannot be undone.')) {
			return;
		}
		try {
			setSaving(true);
			setError(null);
			await configClient.deleteConstitution(create(DeleteConstitutionRequestSchema, {}));
			setConstitution(null);
			setContent('');
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to delete constitution');
		} finally {
			setSaving(false);
		}
	}, []);

	const handleUseTemplate = useCallback(() => {
		setContent(CONSTITUTION_TEMPLATE);
	}, []);

	// Constitution exists if we have a constitution object with content
	const exists = constitution !== null && constitution.content.length > 0;

	if (loading) {
		return (
			<div className="page constitution-page">
				<div className="constitution-loading">Loading constitution...</div>
			</div>
		);
	}

	return (
		<div className="page constitution-page">
			<div className="constitution-header">
				<div>
					<h3>Constitution</h3>
					<p className="constitution-description">
						Project-level principles and invariants for AI-assisted task execution.
						These rules are injected into all phase prompts.
					</p>
				</div>
				<div className="constitution-actions">
					{!exists && (
						<Button
							variant="secondary"
							size="sm"
							onClick={handleUseTemplate}
							leftIcon={<Icon name="file-text" size={14} />}
						>
							Use Template
						</Button>
					)}
					{exists && (
						<Button
							variant="danger"
							size="sm"
							onClick={handleDelete}
							disabled={saving}
							leftIcon={<Icon name="trash" size={14} />}
						>
							Delete
						</Button>
					)}
				</div>
			</div>

			{error && (
				<div className="constitution-error">
					<Icon name="alert-circle" size={16} />
					{error}
				</div>
			)}

			{exists ? (
				<ConfigEditor
					filePath=".orc/CONSTITUTION.md"
					content={content}
					onChange={setContent}
					onSave={handleSave}
					language="markdown"
				/>
			) : content ? (
				<>
					<div className="constitution-preview-notice">
						<Icon name="info" size={16} />
						Preview mode - click Save to create the constitution
					</div>
					<ConfigEditor
						filePath=".orc/CONSTITUTION.md (new)"
						content={content}
						onChange={setContent}
						onSave={handleSave}
						language="markdown"
					/>
				</>
			) : (
				<div className="constitution-empty">
					<Icon name="shield" size={48} />
					<h4>No Constitution Set</h4>
					<p>
						A constitution defines invariants (rules that can never be broken) and
						defaults (guidelines that can be deviated from with justification).
					</p>
					<div className="constitution-empty-actions">
						<Button
							variant="primary"
							onClick={handleUseTemplate}
							leftIcon={<Icon name="file" size={14} />}
						>
							Start from Template
						</Button>
						<Button
							variant="secondary"
							onClick={() => setContent('# My Constitution\n\n')}
							leftIcon={<Icon name="edit" size={14} />}
						>
							Start from Scratch
						</Button>
					</div>
				</div>
			)}

			<div className="constitution-info">
				<h4>How it works</h4>
				<ul>
					<li>
						<strong>Invariants</strong> are absolute rules. Violations automatically block task completion.
					</li>
					<li>
						<strong>Defaults</strong> are guidelines. Can be deviated from with documented justification.
					</li>
					<li>
						The constitution is injected into all phase prompts via <code>{'{{CONSTITUTION_CONTENT}}'}</code>
					</li>
					<li>
						Review phases check for constitution violations and tag findings accordingly.
					</li>
				</ul>
			</div>
		</div>
	);
}
