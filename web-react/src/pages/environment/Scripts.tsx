import { useState, useEffect, useCallback } from 'react';
import {
	listScripts,
	getScript,
	createScript,
	updateScript,
	deleteScript,
	discoverScripts,
	type ProjectScript,
} from '@/lib/api';
import './Scripts.css';

/**
 * Scripts page (/environment/scripts)
 *
 * Manages orc project scripts (.orc/scripts/)
 */
export function Scripts() {
	const [scripts, setScripts] = useState<ProjectScript[]>([]);
	const [selectedScript, setSelectedScript] = useState<ProjectScript | null>(null);
	const [isCreating, setIsCreating] = useState(false);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [discovering, setDiscovering] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [success, setSuccess] = useState<string | null>(null);

	// Form state
	const [formName, setFormName] = useState('');
	const [formDescription, setFormDescription] = useState('');
	const [formPath, setFormPath] = useState('');
	const [formCommand, setFormCommand] = useState('');
	const [formArgs, setFormArgs] = useState('');

	const loadScripts = useCallback(async () => {
		try {
			const data = await listScripts();
			setScripts(data);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load scripts');
		}
	}, []);

	useEffect(() => {
		setLoading(true);
		setError(null);
		loadScripts().finally(() => setLoading(false));
	}, [loadScripts]);

	const selectScript = async (script: ProjectScript) => {
		setError(null);
		setSuccess(null);
		setIsCreating(false);

		try {
			const fullScript = await getScript(script.name);
			setSelectedScript(fullScript);
			setFormName(fullScript.name);
			setFormDescription(fullScript.description || '');
			setFormPath(fullScript.path || '');
			setFormCommand(fullScript.command || '');
			setFormArgs(fullScript.args?.join(' ') || '');
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load script');
		}
	};

	const startCreate = () => {
		setError(null);
		setSuccess(null);
		setSelectedScript(null);
		setIsCreating(true);

		setFormName('');
		setFormDescription('');
		setFormPath('');
		setFormCommand('');
		setFormArgs('');
	};

	const handleDiscover = async () => {
		setDiscovering(true);
		setError(null);
		setSuccess(null);

		try {
			const discovered = await discoverScripts();
			await loadScripts();
			setSuccess(`Discovered ${discovered.length} scripts`);
			globalThis.setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to discover scripts');
		} finally {
			setDiscovering(false);
		}
	};

	const handleSave = async () => {
		if (!formName.trim()) {
			setError('Name is required');
			return;
		}

		if (!formPath.trim() && !formCommand.trim()) {
			setError('Either path or command is required');
			return;
		}

		setSaving(true);
		setError(null);
		setSuccess(null);

		const args = formArgs
			.split(/\s+/)
			.filter((a) => a.trim());

		const script: ProjectScript = {
			name: formName.trim(),
			description: formDescription.trim() || undefined,
			path: formPath.trim() || undefined,
			command: formCommand.trim() || undefined,
			args: args.length > 0 ? args : undefined,
		};

		try {
			if (isCreating) {
				await createScript(script);
				setSuccess('Script created');
			} else if (selectedScript) {
				await updateScript(selectedScript.name, script);
				setSuccess('Script updated');
			}

			await loadScripts();
			setIsCreating(false);

			// Reload
			const updated = await getScript(formName.trim());
			setSelectedScript(updated);

			globalThis.setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to save script');
		} finally {
			setSaving(false);
		}
	};

	const handleDelete = async () => {
		if (!selectedScript) return;

		if (!confirm(`Delete script "${selectedScript.name}"?`)) return;

		setSaving(true);
		setError(null);
		setSuccess(null);

		try {
			await deleteScript(selectedScript.name);
			await loadScripts();
			setSelectedScript(null);
			setIsCreating(false);
			setSuccess('Script deleted');
			globalThis.setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to delete script');
		} finally {
			setSaving(false);
		}
	};

	return (
		<div className="scripts-page">
			<header className="scripts-header">
				<div className="header-content">
					<div>
						<h1>Project Scripts</h1>
						<p className="subtitle">Configure scripts for task execution</p>
					</div>
					<div className="header-actions">
						<button
							className="btn btn-secondary"
							onClick={handleDiscover}
							disabled={discovering}
						>
							{discovering ? 'Discovering...' : 'Discover Scripts'}
						</button>
						<button className="btn btn-primary" onClick={startCreate}>
							New Script
						</button>
					</div>
				</div>
			</header>

			{error && <div className="alert alert-error">{error}</div>}
			{success && <div className="alert alert-success">{success}</div>}

			{loading ? (
				<div className="loading-state">Loading scripts...</div>
			) : (
				<div className="scripts-layout">
					{/* Script List */}
					<aside className="script-list">
						<h2>Scripts</h2>
						{scripts.length === 0 ? (
							<p className="empty-message">
								No scripts configured. Click "Discover Scripts" to find executables.
							</p>
						) : (
							<ul>
								{scripts.map((script) => (
									<li key={script.name}>
										<button
											className={`script-item ${selectedScript?.name === script.name ? 'selected' : ''}`}
											onClick={() => selectScript(script)}
										>
											<span className="script-name">{script.name}</span>
											{script.description && (
												<span className="script-desc">{script.description}</span>
											)}
										</button>
									</li>
								))}
							</ul>
						)}
					</aside>

					{/* Editor Panel */}
					<div className="editor-panel">
						{selectedScript || isCreating ? (
							<>
								<div className="editor-header">
									<h2>{isCreating ? 'New Script' : selectedScript?.name}</h2>
									{selectedScript && !isCreating && (
										<button
											className="btn btn-danger"
											onClick={handleDelete}
											disabled={saving}
										>
											Delete
										</button>
									)}
								</div>

								<form
									className="script-form"
									onSubmit={(e) => {
										e.preventDefault();
										handleSave();
									}}
								>
									<div className="form-group">
										<label htmlFor="name">Name</label>
										<input
											id="name"
											type="text"
											value={formName}
											onChange={(e) => setFormName(e.target.value)}
											placeholder="build"
											disabled={!isCreating}
										/>
									</div>

									<div className="form-group">
										<label htmlFor="description">Description (optional)</label>
										<input
											id="description"
											type="text"
											value={formDescription}
											onChange={(e) => setFormDescription(e.target.value)}
											placeholder="Build the project"
										/>
									</div>

									<div className="form-group">
										<label htmlFor="path">Path (file or directory)</label>
										<input
											id="path"
											type="text"
											value={formPath}
											onChange={(e) => setFormPath(e.target.value)}
											placeholder="./scripts/build.sh"
										/>
									</div>

									<div className="form-group">
										<label htmlFor="command">Command (alternative to path)</label>
										<input
											id="command"
											type="text"
											value={formCommand}
											onChange={(e) => setFormCommand(e.target.value)}
											placeholder="npm run build"
										/>
									</div>

									<div className="form-group">
										<label htmlFor="args">Arguments (optional)</label>
										<input
											id="args"
											type="text"
											value={formArgs}
											onChange={(e) => setFormArgs(e.target.value)}
											placeholder="--production"
										/>
										<span className="form-hint">Space-separated arguments</span>
									</div>

									<div className="form-actions">
										<button type="submit" className="btn btn-primary" disabled={saving}>
											{saving ? 'Saving...' : isCreating ? 'Create' : 'Update'}
										</button>
									</div>
								</form>
							</>
						) : (
							<div className="no-selection">
								<p>Select a script from the list or create a new one.</p>
							</div>
						)}
					</div>
				</div>
			)}
		</div>
	);
}
