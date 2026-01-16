/**
 * Config page (/environment/config)
 * Displays and edits orc configuration from .orc/config.yaml
 */

import { useState, useEffect, useCallback } from 'react';
import * as Accordion from '@radix-ui/react-accordion';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Icon } from '@/components/ui/Icon';
import { toast } from '@/stores';
import {
	getConfig,
	updateConfig,
	type Config as ConfigType,
	type ConfigUpdateRequest,
} from '@/lib/api';
import './environment.css';

export function Config() {
	const [config, setConfig] = useState<ConfigType | null>(null);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [hasChanges, setHasChanges] = useState(false);

	// Form state mirrors config structure
	const [formData, setFormData] = useState<ConfigUpdateRequest>({});

	const loadConfig = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const data = await getConfig();
			setConfig(data);
			// Initialize form with current values
			setFormData({
				profile: data.profile,
				automation: { ...data.automation },
				execution: { ...data.execution },
				git: { ...data.git },
				worktree: { ...data.worktree },
				completion: { ...data.completion },
				timeouts: { ...data.timeouts },
			});
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load configuration');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadConfig();
	}, [loadConfig]);

	const handleChange = <K extends keyof ConfigUpdateRequest>(
		section: K,
		field: keyof NonNullable<ConfigUpdateRequest[K]>,
		value: unknown
	) => {
		setFormData((prev) => ({
			...prev,
			[section]: {
				...(prev[section] as object),
				[field]: value,
			},
		}));
		setHasChanges(true);
	};

	const handleProfileChange = (value: string) => {
		setFormData((prev) => ({ ...prev, profile: value }));
		setHasChanges(true);
	};

	const handleSave = async () => {
		try {
			setSaving(true);
			await updateConfig(formData);
			toast.success('Configuration saved');
			setHasChanges(false);
			await loadConfig();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save configuration');
		} finally {
			setSaving(false);
		}
	};

	const handleReset = () => {
		if (!config) return;
		setFormData({
			profile: config.profile,
			automation: { ...config.automation },
			execution: { ...config.execution },
			git: { ...config.git },
			worktree: { ...config.worktree },
			completion: { ...config.completion },
			timeouts: { ...config.timeouts },
		});
		setHasChanges(false);
	};

	if (loading) {
		return (
			<div className="page environment-config-page">
				<div className="env-loading">Loading configuration...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="page environment-config-page">
				<div className="env-error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadConfig}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	return (
		<div className="page environment-config-page">
			<div className="env-page-header">
				<h3>Orc Configuration</h3>
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

			<Accordion.Root type="multiple" defaultValue={['automation', 'execution']} className="config-accordion">
				{/* Profile */}
				<Accordion.Item value="profile" className="config-section">
					<Accordion.Header>
						<Accordion.Trigger className="config-section-header">
							<h4 className="config-section-title">
								<Icon name="zap" size={18} />
								Profile
							</h4>
							<Icon name="chevron-down" size={16} />
						</Accordion.Trigger>
					</Accordion.Header>
					<Accordion.Content className="config-section-content">
						<div className="config-field">
							<label htmlFor="profile">Automation Profile</label>
							<select
								id="profile"
								value={formData.profile || ''}
								onChange={(e) => handleProfileChange(e.target.value)}
								className="input-field"
								style={{ padding: 'var(--space-2)' }}
							>
								<option value="auto">Auto (fully automated)</option>
								<option value="fast">Fast (speed over safety)</option>
								<option value="safe">Safe (AI reviews, human merge)</option>
								<option value="strict">Strict (human gates)</option>
							</select>
						</div>
					</Accordion.Content>
				</Accordion.Item>

				{/* Automation */}
				<Accordion.Item value="automation" className="config-section">
					<Accordion.Header>
						<Accordion.Trigger className="config-section-header">
							<h4 className="config-section-title">
								<Icon name="target" size={18} />
								Automation
							</h4>
							<Icon name="chevron-down" size={16} />
						</Accordion.Trigger>
					</Accordion.Header>
					<Accordion.Content className="config-section-content">
						<div className="config-field-group">
							<div className="config-field">
								<label htmlFor="gates_default">Gates Default</label>
								<select
									id="gates_default"
									value={formData.automation?.gates_default || ''}
									onChange={(e) => handleChange('automation', 'gates_default', e.target.value)}
									className="input-field"
									style={{ padding: 'var(--space-2)' }}
								>
									<option value="ai">AI</option>
									<option value="human">Human</option>
									<option value="auto">Auto</option>
								</select>
							</div>
							<div className="config-field config-checkbox-field">
								<input
									type="checkbox"
									id="retry_enabled"
									checked={formData.automation?.retry_enabled ?? false}
									onChange={(e) => handleChange('automation', 'retry_enabled', e.target.checked)}
								/>
								<label htmlFor="retry_enabled">Retry Enabled</label>
							</div>
							<div className="config-field">
								<label htmlFor="retry_max">Max Retries</label>
								<Input
									id="retry_max"
									type="number"
									min="0"
									max="10"
									value={formData.automation?.retry_max ?? ''}
									onChange={(e) => handleChange('automation', 'retry_max', parseInt(e.target.value) || 0)}
									size="sm"
								/>
							</div>
						</div>
					</Accordion.Content>
				</Accordion.Item>

				{/* Execution */}
				<Accordion.Item value="execution" className="config-section">
					<Accordion.Header>
						<Accordion.Trigger className="config-section-header">
							<h4 className="config-section-title">
								<Icon name="cpu" size={18} />
								Execution
							</h4>
							<Icon name="chevron-down" size={16} />
						</Accordion.Trigger>
					</Accordion.Header>
					<Accordion.Content className="config-section-content">
						<div className="config-field-group">
							<div className="config-field">
								<label htmlFor="model">Model</label>
								<Input
									id="model"
									value={formData.execution?.model ?? ''}
									onChange={(e) => handleChange('execution', 'model', e.target.value)}
									size="sm"
									placeholder="claude-sonnet-4-20250514"
								/>
							</div>
							<div className="config-field">
								<label htmlFor="max_iterations">Max Iterations</label>
								<Input
									id="max_iterations"
									type="number"
									min="1"
									max="100"
									value={formData.execution?.max_iterations ?? ''}
									onChange={(e) => handleChange('execution', 'max_iterations', parseInt(e.target.value) || 10)}
									size="sm"
								/>
							</div>
							<div className="config-field">
								<label htmlFor="timeout">Timeout</label>
								<Input
									id="timeout"
									value={formData.execution?.timeout ?? ''}
									onChange={(e) => handleChange('execution', 'timeout', e.target.value)}
									size="sm"
									placeholder="30m"
								/>
							</div>
						</div>
					</Accordion.Content>
				</Accordion.Item>

				{/* Git */}
				<Accordion.Item value="git" className="config-section">
					<Accordion.Header>
						<Accordion.Trigger className="config-section-header">
							<h4 className="config-section-title">
								<Icon name="git-branch" size={18} />
								Git
							</h4>
							<Icon name="chevron-down" size={16} />
						</Accordion.Trigger>
					</Accordion.Header>
					<Accordion.Content className="config-section-content">
						<div className="config-field-group">
							<div className="config-field">
								<label htmlFor="branch_prefix">Branch Prefix</label>
								<Input
									id="branch_prefix"
									value={formData.git?.branch_prefix ?? ''}
									onChange={(e) => handleChange('git', 'branch_prefix', e.target.value)}
									size="sm"
									placeholder="orc/"
								/>
							</div>
							<div className="config-field">
								<label htmlFor="commit_prefix">Commit Prefix</label>
								<Input
									id="commit_prefix"
									value={formData.git?.commit_prefix ?? ''}
									onChange={(e) => handleChange('git', 'commit_prefix', e.target.value)}
									size="sm"
									placeholder="[orc]"
								/>
							</div>
						</div>
					</Accordion.Content>
				</Accordion.Item>

				{/* Worktree */}
				<Accordion.Item value="worktree" className="config-section">
					<Accordion.Header>
						<Accordion.Trigger className="config-section-header">
							<h4 className="config-section-title">
								<Icon name="folder" size={18} />
								Worktree
							</h4>
							<Icon name="chevron-down" size={16} />
						</Accordion.Trigger>
					</Accordion.Header>
					<Accordion.Content className="config-section-content">
						<div className="config-field-group">
							<div className="config-field config-checkbox-field">
								<input
									type="checkbox"
									id="worktree_enabled"
									checked={formData.worktree?.enabled ?? false}
									onChange={(e) => handleChange('worktree', 'enabled', e.target.checked)}
								/>
								<label htmlFor="worktree_enabled">Enabled</label>
							</div>
							<div className="config-field">
								<label htmlFor="worktree_dir">Directory</label>
								<Input
									id="worktree_dir"
									value={formData.worktree?.dir ?? ''}
									onChange={(e) => handleChange('worktree', 'dir', e.target.value)}
									size="sm"
									placeholder=".orc/worktrees"
								/>
							</div>
							<div className="config-field config-checkbox-field">
								<input
									type="checkbox"
									id="cleanup_on_complete"
									checked={formData.worktree?.cleanup_on_complete ?? false}
									onChange={(e) => handleChange('worktree', 'cleanup_on_complete', e.target.checked)}
								/>
								<label htmlFor="cleanup_on_complete">Cleanup on Complete</label>
							</div>
							<div className="config-field config-checkbox-field">
								<input
									type="checkbox"
									id="cleanup_on_fail"
									checked={formData.worktree?.cleanup_on_fail ?? false}
									onChange={(e) => handleChange('worktree', 'cleanup_on_fail', e.target.checked)}
								/>
								<label htmlFor="cleanup_on_fail">Cleanup on Fail</label>
							</div>
						</div>
					</Accordion.Content>
				</Accordion.Item>

				{/* Completion */}
				<Accordion.Item value="completion" className="config-section">
					<Accordion.Header>
						<Accordion.Trigger className="config-section-header">
							<h4 className="config-section-title">
								<Icon name="check-circle" size={18} />
								Completion
							</h4>
							<Icon name="chevron-down" size={16} />
						</Accordion.Trigger>
					</Accordion.Header>
					<Accordion.Content className="config-section-content">
						<div className="config-field-group">
							<div className="config-field">
								<label htmlFor="completion_action">Action</label>
								<select
									id="completion_action"
									value={formData.completion?.action || ''}
									onChange={(e) => handleChange('completion', 'action', e.target.value)}
									className="input-field"
									style={{ padding: 'var(--space-2)' }}
								>
									<option value="pr">Create PR</option>
									<option value="merge">Merge</option>
									<option value="none">None</option>
								</select>
							</div>
							<div className="config-field">
								<label htmlFor="target_branch">Target Branch</label>
								<Input
									id="target_branch"
									value={formData.completion?.target_branch ?? ''}
									onChange={(e) => handleChange('completion', 'target_branch', e.target.value)}
									size="sm"
									placeholder="main"
								/>
							</div>
							<div className="config-field config-checkbox-field">
								<input
									type="checkbox"
									id="delete_branch"
									checked={formData.completion?.delete_branch ?? false}
									onChange={(e) => handleChange('completion', 'delete_branch', e.target.checked)}
								/>
								<label htmlFor="delete_branch">Delete Branch After Merge</label>
							</div>
						</div>
					</Accordion.Content>
				</Accordion.Item>

				{/* Timeouts */}
				<Accordion.Item value="timeouts" className="config-section">
					<Accordion.Header>
						<Accordion.Trigger className="config-section-header">
							<h4 className="config-section-title">
								<Icon name="clock" size={18} />
								Timeouts
							</h4>
							<Icon name="chevron-down" size={16} />
						</Accordion.Trigger>
					</Accordion.Header>
					<Accordion.Content className="config-section-content">
						<div className="config-field-group">
							<div className="config-field">
								<label htmlFor="phase_max">Phase Max</label>
								<Input
									id="phase_max"
									value={formData.timeouts?.phase_max ?? ''}
									onChange={(e) => handleChange('timeouts', 'phase_max', e.target.value)}
									size="sm"
									placeholder="1h"
								/>
							</div>
							<div className="config-field">
								<label htmlFor="turn_max">Turn Max</label>
								<Input
									id="turn_max"
									value={formData.timeouts?.turn_max ?? ''}
									onChange={(e) => handleChange('timeouts', 'turn_max', e.target.value)}
									size="sm"
									placeholder="5m"
								/>
							</div>
							<div className="config-field">
								<label htmlFor="idle_warning">Idle Warning</label>
								<Input
									id="idle_warning"
									value={formData.timeouts?.idle_warning ?? ''}
									onChange={(e) => handleChange('timeouts', 'idle_warning', e.target.value)}
									size="sm"
									placeholder="2m"
								/>
							</div>
							<div className="config-field">
								<label htmlFor="heartbeat_interval">Heartbeat Interval</label>
								<Input
									id="heartbeat_interval"
									value={formData.timeouts?.heartbeat_interval ?? ''}
									onChange={(e) => handleChange('timeouts', 'heartbeat_interval', e.target.value)}
									size="sm"
									placeholder="10s"
								/>
							</div>
							<div className="config-field">
								<label htmlFor="idle_timeout">Idle Timeout</label>
								<Input
									id="idle_timeout"
									value={formData.timeouts?.idle_timeout ?? ''}
									onChange={(e) => handleChange('timeouts', 'idle_timeout', e.target.value)}
									size="sm"
									placeholder="10m"
								/>
							</div>
						</div>
					</Accordion.Content>
				</Accordion.Item>
			</Accordion.Root>
		</div>
	);
}
