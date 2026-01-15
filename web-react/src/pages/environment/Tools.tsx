import { useState, useEffect, useCallback } from 'react';
import {
	listToolsByCategory,
	getToolPermissions,
	updateToolPermissions,
	type ToolsByCategory,
	type ToolPermissions,
} from '@/lib/api';
import './Tools.css';

/**
 * Tools page (/environment/tools)
 *
 * Manages Claude Code tool permissions
 */
export function Tools() {
	const [tools, setTools] = useState<ToolsByCategory>({});
	const [permissions, setPermissions] = useState<ToolPermissions>({});
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [success, setSuccess] = useState<string | null>(null);
	const [hasChanges, setHasChanges] = useState(false);

	// Original permissions for comparison
	const [originalPermissions, setOriginalPermissions] = useState<ToolPermissions>({});

	const loadData = useCallback(async () => {
		try {
			const [toolsData, permsData] = await Promise.all([
				listToolsByCategory(),
				getToolPermissions(),
			]);
			setTools(toolsData);
			setPermissions(permsData);
			setOriginalPermissions(permsData);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load tools');
		}
	}, []);

	useEffect(() => {
		setLoading(true);
		setError(null);
		loadData().finally(() => setLoading(false));
	}, [loadData]);

	const togglePermission = (tool: string, permission: 'allow' | 'deny') => {
		setPermissions((prev) => {
			const current = prev[tool];
			if (current === permission) {
				// Toggle off - remove from permissions
				const updated = { ...prev };
				delete updated[tool];
				setHasChanges(JSON.stringify(updated) !== JSON.stringify(originalPermissions));
				return updated;
			} else {
				// Set permission
				const updated = { ...prev, [tool]: permission };
				setHasChanges(JSON.stringify(updated) !== JSON.stringify(originalPermissions));
				return updated;
			}
		});
	};

	const handleSave = async () => {
		setSaving(true);
		setError(null);
		setSuccess(null);

		try {
			await updateToolPermissions(permissions);
			setOriginalPermissions(permissions);
			setHasChanges(false);
			setSuccess('Permissions saved');
			globalThis.setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to save permissions');
		} finally {
			setSaving(false);
		}
	};

	const handleReset = () => {
		setPermissions(originalPermissions);
		setHasChanges(false);
	};

	const categories = Object.keys(tools).sort();

	return (
		<div className="tools-page">
			<header className="tools-header">
				<div className="header-content">
					<div>
						<h1>Tool Permissions</h1>
						<p className="subtitle">Configure which tools Claude Code can use</p>
					</div>
					<div className="header-actions">
						{hasChanges && (
							<button className="btn btn-secondary" onClick={handleReset}>
								Reset
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
			</header>

			{error && <div className="alert alert-error">{error}</div>}
			{success && <div className="alert alert-success">{success}</div>}

			{loading ? (
				<div className="loading-state">Loading tools...</div>
			) : (
				<div className="tools-content">
					{categories.length === 0 ? (
						<p className="empty-message">No tools available</p>
					) : (
						categories.map((category) => (
							<section key={category} className="tool-category">
								<h2>{category}</h2>
								<div className="tool-grid">
									{tools[category].map((tool) => (
										<div key={tool.name} className="tool-card">
											<div className="tool-info">
												<span className="tool-name">{tool.name}</span>
												{tool.description && (
													<span className="tool-desc">{tool.description}</span>
												)}
											</div>
											<div className="permission-toggle">
												<button
													className={`toggle-btn allow ${permissions[tool.name] === 'allow' ? 'active' : ''}`}
													onClick={() => togglePermission(tool.name, 'allow')}
													title="Allow"
												>
													Allow
												</button>
												<button
													className={`toggle-btn deny ${permissions[tool.name] === 'deny' ? 'active' : ''}`}
													onClick={() => togglePermission(tool.name, 'deny')}
													title="Deny"
												>
													Deny
												</button>
											</div>
										</div>
									))}
								</div>
							</section>
						))
					)}
				</div>
			)}
		</div>
	);
}
