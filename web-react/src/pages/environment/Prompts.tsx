import { useState, useEffect, useCallback } from 'react';
import {
	listPrompts,
	getPrompt,
	updatePrompt,
	resetPrompt,
	type PromptInfo,
	type Prompt,
} from '@/lib/api';
import './Prompts.css';

/**
 * Prompts page (/environment/prompts)
 *
 * Manages orc phase prompt overrides (.orc/prompts/)
 */
export function Prompts() {
	const [prompts, setPrompts] = useState<PromptInfo[]>([]);
	const [selectedPrompt, setSelectedPrompt] = useState<Prompt | null>(null);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [success, setSuccess] = useState<string | null>(null);

	// Form state
	const [formContent, setFormContent] = useState('');
	const [hasChanges, setHasChanges] = useState(false);

	const loadPrompts = useCallback(async () => {
		try {
			const data = await listPrompts();
			setPrompts(data);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load prompts');
		}
	}, []);

	useEffect(() => {
		setLoading(true);
		setError(null);
		loadPrompts().finally(() => setLoading(false));
	}, [loadPrompts]);

	const selectPrompt = async (info: PromptInfo) => {
		setError(null);
		setSuccess(null);
		setHasChanges(false);

		try {
			const prompt = await getPrompt(info.phase);
			setSelectedPrompt(prompt);
			setFormContent(prompt.content);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load prompt');
		}
	};

	const handleContentChange = (value: string) => {
		setFormContent(value);
		setHasChanges(value !== selectedPrompt?.content);
	};

	const handleSave = async () => {
		if (!selectedPrompt) return;

		setSaving(true);
		setError(null);
		setSuccess(null);

		try {
			await updatePrompt(selectedPrompt.phase, formContent);
			await loadPrompts();

			// Reload the prompt
			const updated = await getPrompt(selectedPrompt.phase);
			setSelectedPrompt(updated);
			setFormContent(updated.content);
			setHasChanges(false);

			setSuccess('Prompt saved successfully');
			globalThis.setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to save prompt');
		} finally {
			setSaving(false);
		}
	};

	const handleReset = async () => {
		if (!selectedPrompt) return;

		if (!confirm(`Reset "${selectedPrompt.phase}" prompt to default?`)) return;

		setSaving(true);
		setError(null);
		setSuccess(null);

		try {
			await resetPrompt(selectedPrompt.phase);
			await loadPrompts();

			// Reload the prompt
			const updated = await getPrompt(selectedPrompt.phase);
			setSelectedPrompt(updated);
			setFormContent(updated.content);
			setHasChanges(false);

			setSuccess('Prompt reset to default');
			globalThis.setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to reset prompt');
		} finally {
			setSaving(false);
		}
	};

	const getPhaseDescription = (phase: string): string => {
		switch (phase) {
			case 'spec':
				return 'Define requirements and approach';
			case 'research':
				return 'Investigate codebase and dependencies';
			case 'implement':
				return 'Write the implementation code';
			case 'test':
				return 'Write and run tests';
			case 'docs':
				return 'Update documentation';
			case 'validate':
				return 'Verify implementation is complete';
			case 'finalize':
				return 'Sync branch and create PR';
			default:
				return '';
		}
	};

	return (
		<div className="prompts-page">
			<header className="prompts-header">
				<div className="header-content">
					<div>
						<h1>Phase Prompts</h1>
						<p className="subtitle">Customize prompts for each execution phase</p>
					</div>
				</div>
			</header>

			{error && <div className="alert alert-error">{error}</div>}
			{success && <div className="alert alert-success">{success}</div>}

			{loading ? (
				<div className="loading-state">Loading prompts...</div>
			) : (
				<div className="prompts-layout">
					{/* Phase List */}
					<aside className="phase-list">
						<h2>Phases</h2>
						<ul>
							{prompts.map((prompt) => (
								<li key={prompt.phase}>
									<button
										className={`phase-item ${selectedPrompt?.phase === prompt.phase ? 'selected' : ''}`}
										onClick={() => selectPrompt(prompt)}
									>
										<span className="phase-name">{prompt.phase}</span>
										<span className="phase-desc">{getPhaseDescription(prompt.phase)}</span>
										{prompt.is_custom && <span className="badge badge-custom">Custom</span>}
									</button>
								</li>
							))}
						</ul>
					</aside>

					{/* Editor Panel */}
					<div className="editor-panel">
						{selectedPrompt ? (
							<>
								<div className="editor-header">
									<div>
										<h2>{selectedPrompt.phase}</h2>
										<p className="phase-desc">{getPhaseDescription(selectedPrompt.phase)}</p>
									</div>
									<div className="header-buttons">
										{selectedPrompt.is_custom && (
											<button
												className="btn btn-secondary"
												onClick={handleReset}
												disabled={saving}
											>
												Reset to Default
											</button>
										)}
										<button
											className="btn btn-primary"
											onClick={handleSave}
											disabled={saving || !hasChanges}
										>
											{saving ? 'Saving...' : 'Save'}
										</button>
									</div>
								</div>

								<div className="editor-content">
									<textarea
										value={formContent}
										onChange={(e) => handleContentChange(e.target.value)}
										placeholder="Enter phase prompt..."
										rows={25}
									/>
								</div>

								<div className="editor-footer">
									<span className="hint">
										Use <code>{'{{TASK_TITLE}}'}</code>, <code>{'{{TASK_DESCRIPTION}}'}</code>, <code>{'{{SPEC_CONTENT}}'}</code> for
										template variables
									</span>
								</div>
							</>
						) : (
							<div className="no-selection">
								<p>Select a phase to view or edit its prompt</p>
							</div>
						)}
					</div>
				</div>
			)}
		</div>
	);
}
