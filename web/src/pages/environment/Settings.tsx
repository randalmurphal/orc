/**
 * Environment Settings page (/environment/settings)
 * Displays and edits Claude Code settings (global + project)
 */

import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Icon } from '@/components/ui/Icon';
import { toast } from '@/stores';
import {
	getSettingsHierarchy,
	updateSettings,
	updateGlobalSettings,
	type Settings as SettingsType,
	type SettingsHierarchy,
} from '@/lib/api';
import './environment.css';

type Scope = 'global' | 'project';

interface EnvVar {
	key: string;
	value: string;
}

export function Settings() {
	const [hierarchy, setHierarchy] = useState<SettingsHierarchy | null>(null);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [scope, setScope] = useState<Scope>('project');

	// Form state
	const [envVars, setEnvVars] = useState<EnvVar[]>([]);
	const [statusLineType, setStatusLineType] = useState<string>('');
	const [statusLineCommand, setStatusLineCommand] = useState<string>('');
	const [hasChanges, setHasChanges] = useState(false);

	const loadSettings = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const data = await getSettingsHierarchy();
			setHierarchy(data);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load settings');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadSettings();
	}, [loadSettings]);

	// Update form when scope changes
	useEffect(() => {
		if (!hierarchy) return;

		const settings =
			scope === 'global' ? hierarchy.global.settings : hierarchy.project.settings;

		// Convert env object to array
		const env = settings?.env || {};
		setEnvVars(Object.entries(env).map(([key, value]) => ({ key, value })));

		// Status line
		setStatusLineType(settings?.statusLine?.type || '');
		setStatusLineCommand(settings?.statusLine?.command || '');
		setHasChanges(false);
	}, [hierarchy, scope]);

	const handleEnvChange = (index: number, field: 'key' | 'value', value: string) => {
		const newVars = [...envVars];
		newVars[index] = { ...newVars[index], [field]: value };
		setEnvVars(newVars);
		setHasChanges(true);
	};

	const handleAddEnvVar = () => {
		setEnvVars([...envVars, { key: '', value: '' }]);
		setHasChanges(true);
	};

	const handleRemoveEnvVar = (index: number) => {
		setEnvVars(envVars.filter((_, i) => i !== index));
		setHasChanges(true);
	};

	const handleStatusLineChange = (field: 'type' | 'command', value: string) => {
		if (field === 'type') {
			setStatusLineType(value);
		} else {
			setStatusLineCommand(value);
		}
		setHasChanges(true);
	};

	const handleSave = async () => {
		try {
			setSaving(true);

			// Build settings object
			const settings: SettingsType = {};

			// Convert env vars array back to object (filtering empty keys)
			const env: Record<string, string> = {};
			for (const v of envVars) {
				if (v.key.trim()) {
					env[v.key.trim()] = v.value;
				}
			}
			if (Object.keys(env).length > 0) {
				settings.env = env;
			}

			// Status line
			if (statusLineType || statusLineCommand) {
				settings.statusLine = {};
				if (statusLineType) settings.statusLine.type = statusLineType;
				if (statusLineCommand) settings.statusLine.command = statusLineCommand;
			}

			// Save based on scope
			if (scope === 'global') {
				await updateGlobalSettings(settings);
			} else {
				await updateSettings(settings);
			}

			toast.success(`${scope === 'global' ? 'Global' : 'Project'} settings saved`);
			setHasChanges(false);

			// Reload to get updated state
			await loadSettings();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save settings');
		} finally {
			setSaving(false);
		}
	};

	const handleReset = () => {
		// Re-apply current settings from hierarchy
		if (!hierarchy) return;

		const settings =
			scope === 'global' ? hierarchy.global.settings : hierarchy.project.settings;

		const env = settings?.env || {};
		setEnvVars(Object.entries(env).map(([key, value]) => ({ key, value })));
		setStatusLineType(settings?.statusLine?.type || '');
		setStatusLineCommand(settings?.statusLine?.command || '');
		setHasChanges(false);
	};

	if (loading) {
		return (
			<div className="page environment-settings-page">
				<div className="env-loading">Loading settings...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="page environment-settings-page">
				<div className="env-error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadSettings}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	const currentPath = scope === 'global' ? hierarchy?.global.path : hierarchy?.project.path;

	return (
		<div className="page environment-settings-page">
			<div className="env-page-header">
				<h3>Claude Code Settings</h3>
				<div className="env-page-header-actions">
					{hasChanges && (
						<>
							<Button variant="ghost" onClick={handleReset}>
								Reset
							</Button>
							<Button variant="primary" onClick={handleSave} loading={saving}>
								Save Changes
							</Button>
						</>
					)}
				</div>
			</div>

			<Tabs.Root value={scope} onValueChange={(v) => setScope(v as Scope)}>
				<Tabs.List className="env-scope-tabs" aria-label="Settings scope">
					<Tabs.Trigger value="project" className="env-scope-tab">
						<Icon name="folder" size={16} />
						Project
					</Tabs.Trigger>
					<Tabs.Trigger value="global" className="env-scope-tab">
						<Icon name="user" size={16} />
						Global
					</Tabs.Trigger>
				</Tabs.List>

				<Tabs.Content value={scope} className="settings-content">
					{currentPath && (
						<p className="claudemd-path">
							<Icon name="file" size={14} /> {currentPath}
						</p>
					)}

					{/* Environment Variables */}
					<div className="settings-section">
						<h4 className="settings-section-title">
							<Icon name="terminal" size={18} />
							Environment Variables
						</h4>
						<div className="settings-env-vars">
							{envVars.length === 0 ? (
								<p className="env-empty" style={{ padding: 'var(--space-4)', textAlign: 'left' }}>
									No environment variables configured
								</p>
							) : (
								envVars.map((v, i) => (
									<div key={i} className="settings-env-row">
										<Input
											placeholder="KEY"
											value={v.key}
											onChange={(e) => handleEnvChange(i, 'key', e.target.value)}
											size="sm"
										/>
										<Input
											placeholder="value"
											value={v.value}
											onChange={(e) => handleEnvChange(i, 'value', e.target.value)}
											size="sm"
										/>
										<Button
											variant="ghost"
											size="sm"
											iconOnly
											aria-label="Remove variable"
											onClick={() => handleRemoveEnvVar(i)}
										>
											<Icon name="trash" size={16} />
										</Button>
									</div>
								))
							)}
							<Button
								variant="ghost"
								size="sm"
								className="settings-add-btn"
								leftIcon={<Icon name="plus" size={16} />}
								onClick={handleAddEnvVar}
							>
								Add Variable
							</Button>
						</div>
					</div>

					{/* Status Line */}
					<div className="settings-section">
						<h4 className="settings-section-title">
							<Icon name="statusline" size={18} />
							Status Line
						</h4>
						<div className="settings-statusline-type">
							<label>
								<input
									type="radio"
									name="statusLineType"
									value=""
									checked={!statusLineType}
									onChange={() => handleStatusLineChange('type', '')}
								/>
								None (default)
							</label>
							<label>
								<input
									type="radio"
									name="statusLineType"
									value="command"
									checked={statusLineType === 'command'}
									onChange={() => handleStatusLineChange('type', 'command')}
								/>
								Custom command
							</label>
						</div>
						{statusLineType === 'command' && (
							<Input
								placeholder="echo -n '[$USER:${HOSTNAME%%.*}]:${PWD##*/}'"
								value={statusLineCommand}
								onChange={(e) => handleStatusLineChange('command', e.target.value)}
								size="sm"
							/>
						)}
					</div>
				</Tabs.Content>
			</Tabs.Root>
		</div>
	);
}
