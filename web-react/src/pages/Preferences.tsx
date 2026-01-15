import { useState, useEffect, useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
	getSettings,
	getGlobalSettings,
	getProjectSettings,
	updateSettings,
	updateGlobalSettings,
	type Settings,
} from '@/lib/api';
import './Preferences.css';

type Tab = 'global' | 'project' | 'env';

/**
 * Preferences page (/preferences)
 *
 * Settings for Claude Code with tabs:
 * - Global settings (~/.claude/settings.json)
 * - Project settings (.claude/settings.json)
 * - Environment variables
 */
export function Preferences() {
	const [searchParams, setSearchParams] = useSearchParams();
	const [globalSettings, setGlobalSettings] = useState<Settings | null>(null);
	const [projectSettings, setProjectSettings] = useState<Settings | null>(null);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [success, setSuccess] = useState<string | null>(null);

	// Form state for environment variables
	const [envVars, setEnvVars] = useState<[string, string][]>([]);
	const [newEnvKey, setNewEnvKey] = useState('');
	const [newEnvValue, setNewEnvValue] = useState('');

	const activeTab = (searchParams.get('tab') as Tab) || 'global';

	const setActiveTab = (tab: Tab) => {
		if (tab === 'global') {
			setSearchParams({});
		} else {
			setSearchParams({ tab });
		}
	};

	const loadSettings = useCallback(async () => {
		setLoading(true);
		setError(null);

		try {
			const [global, project] = await Promise.all([
				getGlobalSettings(),
				getProjectSettings(),
			]);
			setGlobalSettings(global);
			setProjectSettings(project);

			// Initialize env vars from project settings
			const env = project?.env || global?.env || {};
			setEnvVars(Object.entries(env));
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load settings');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadSettings();
	}, [loadSettings]);

	const handleSaveEnvVars = async () => {
		setSaving(true);
		setError(null);
		setSuccess(null);

		try {
			const env: Record<string, string> = {};
			for (const [key, value] of envVars) {
				if (key.trim()) {
					env[key.trim()] = value;
				}
			}

			await updateSettings({ ...projectSettings, env }, undefined);
			await loadSettings();
			setSuccess('Environment variables saved');
			globalThis.setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to save settings');
		} finally {
			setSaving(false);
		}
	};

	const addEnvVar = () => {
		if (newEnvKey.trim()) {
			setEnvVars([...envVars, [newEnvKey.trim(), newEnvValue]]);
			setNewEnvKey('');
			setNewEnvValue('');
		}
	};

	const updateEnvVar = (index: number, key: string, value: string) => {
		const updated = [...envVars];
		updated[index] = [key, value];
		setEnvVars(updated);
	};

	const removeEnvVar = (index: number) => {
		setEnvVars(envVars.filter((_, i) => i !== index));
	};

	const renderGlobalSettings = () => (
		<div className="settings-content">
			<h2>Global Settings</h2>
			<p className="settings-desc">
				Settings stored in ~/.claude/settings.json. These apply to all projects.
			</p>

			{globalSettings ? (
				<div className="settings-preview">
					<pre>{JSON.stringify(globalSettings, null, 2)}</pre>
				</div>
			) : (
				<p className="no-settings">No global settings configured</p>
			)}

			<div className="settings-hint">
				<p>
					To edit global settings, use the{' '}
					<a href="/environment/hooks?scope=global">Hooks</a>,{' '}
					<a href="/environment/skills?scope=global">Skills</a>, or{' '}
					<a href="/environment/claudemd?scope=global">CLAUDE.md</a> pages.
				</p>
			</div>
		</div>
	);

	const renderProjectSettings = () => (
		<div className="settings-content">
			<h2>Project Settings</h2>
			<p className="settings-desc">
				Settings stored in .claude/settings.json. These apply only to this project.
			</p>

			{projectSettings ? (
				<div className="settings-preview">
					<pre>{JSON.stringify(projectSettings, null, 2)}</pre>
				</div>
			) : (
				<p className="no-settings">No project settings configured</p>
			)}

			<div className="settings-hint">
				<p>
					To edit project settings, use the{' '}
					<a href="/environment/hooks">Hooks</a>,{' '}
					<a href="/environment/skills">Skills</a>, or{' '}
					<a href="/environment/claudemd">CLAUDE.md</a> pages.
				</p>
			</div>
		</div>
	);

	const renderEnvVars = () => (
		<div className="settings-content">
			<div className="env-header">
				<div>
					<h2>Environment Variables</h2>
					<p className="settings-desc">
						Environment variables available to Claude Code and hooks.
					</p>
				</div>
				<button
					className="btn btn-primary"
					onClick={handleSaveEnvVars}
					disabled={saving}
				>
					{saving ? 'Saving...' : 'Save'}
				</button>
			</div>

			<div className="env-vars-list">
				{envVars.length === 0 ? (
					<p className="no-settings">No environment variables configured</p>
				) : (
					envVars.map(([key, value], index) => (
						<div className="env-var-row" key={index}>
							<input
								type="text"
								value={key}
								onChange={(e) => updateEnvVar(index, e.target.value, value)}
								placeholder="KEY"
								className="env-key"
							/>
							<span className="env-equals">=</span>
							<input
								type="text"
								value={value}
								onChange={(e) => updateEnvVar(index, key, e.target.value)}
								placeholder="value"
								className="env-value"
							/>
							<button
								className="btn-icon btn-danger"
								onClick={() => removeEnvVar(index)}
								title="Remove"
							>
								&times;
							</button>
						</div>
					))
				)}

				<div className="env-var-row env-var-new">
					<input
						type="text"
						value={newEnvKey}
						onChange={(e) => setNewEnvKey(e.target.value)}
						placeholder="NEW_KEY"
						className="env-key"
					/>
					<span className="env-equals">=</span>
					<input
						type="text"
						value={newEnvValue}
						onChange={(e) => setNewEnvValue(e.target.value)}
						placeholder="value"
						className="env-value"
						onKeyDown={(e) => e.key === 'Enter' && addEnvVar()}
					/>
					<button
						className="btn btn-secondary btn-sm"
						onClick={addEnvVar}
						disabled={!newEnvKey.trim()}
					>
						Add
					</button>
				</div>
			</div>
		</div>
	);

	return (
		<div className="preferences-page">
			<header className="preferences-header">
				<h1>Preferences</h1>
				<p className="subtitle">Claude Code settings and environment</p>
			</header>

			{error && <div className="alert alert-error">{error}</div>}
			{success && <div className="alert alert-success">{success}</div>}

			{/* Tabs */}
			<div className="tabs">
				<button
					className={`tab ${activeTab === 'global' ? 'active' : ''}`}
					onClick={() => setActiveTab('global')}
				>
					Global Settings
				</button>
				<button
					className={`tab ${activeTab === 'project' ? 'active' : ''}`}
					onClick={() => setActiveTab('project')}
				>
					Project Settings
				</button>
				<button
					className={`tab ${activeTab === 'env' ? 'active' : ''}`}
					onClick={() => setActiveTab('env')}
				>
					Environment Variables
				</button>
			</div>

			{/* Tab Content */}
			<div className="tab-content">
				{loading ? (
					<div className="loading-state">Loading settings...</div>
				) : (
					<>
						{activeTab === 'global' && renderGlobalSettings()}
						{activeTab === 'project' && renderProjectSettings()}
						{activeTab === 'env' && renderEnvVars()}
					</>
				)}
			</div>
		</div>
	);
}
