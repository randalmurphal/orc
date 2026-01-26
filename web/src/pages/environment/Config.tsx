/**
 * Config page (/environment/config)
 * Displays and edits orc configuration from .orc/config.yaml
 */

import { useState, useEffect, useCallback } from 'react';
import * as Accordion from '@radix-ui/react-accordion';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Icon } from '@/components/ui/Icon';
import { toast } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import { configClient } from '@/lib/client';
import {
	type Config as ConfigType,
	GetConfigRequestSchema,
	UpdateConfigRequestSchema,
	AutomationConfigSchema,
	CompletionConfigSchema,
	ExportConfigSchema,
	ClaudeConfigSchema,
} from '@/gen/orc/v1/config_pb';
import './environment.css';

// Form state mirrors protobuf Config structure
interface FormData {
	automation: {
		profile: string;
		autoApprove: boolean;
		autoSkip: boolean;
	};
	completion: {
		action: string;
		autoMerge: boolean;
		targetBranch: string;
	};
	export: {
		includeTranscripts: boolean;
		includeAttachments: boolean;
		format: string;
	};
	claude: {
		model: string;
		thinking: boolean;
		maxTurns: number;
		temperature: number;
	};
}

const defaultFormData: FormData = {
	automation: {
		profile: 'auto',
		autoApprove: false,
		autoSkip: false,
	},
	completion: {
		action: 'pr',
		autoMerge: false,
		targetBranch: 'main',
	},
	export: {
		includeTranscripts: true,
		includeAttachments: false,
		format: 'tar.gz',
	},
	claude: {
		model: 'claude-sonnet-4-20250514',
		thinking: false,
		maxTurns: 10,
		temperature: 0.7,
	},
};

export function Config() {
	useDocumentTitle('Configuration');
	const [config, setConfig] = useState<ConfigType | null>(null);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [hasChanges, setHasChanges] = useState(false);

	// Form state mirrors config structure
	const [formData, setFormData] = useState<FormData>(defaultFormData);

	const loadConfig = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const response = await configClient.getConfig(create(GetConfigRequestSchema, {}));
			const cfg = response.config;
			setConfig(cfg ?? null);
			// Initialize form with current values
			setFormData({
				automation: {
					profile: cfg?.automation?.profile || 'auto',
					autoApprove: cfg?.automation?.autoApprove || false,
					autoSkip: cfg?.automation?.autoSkip || false,
				},
				completion: {
					action: cfg?.completion?.action || 'pr',
					autoMerge: cfg?.completion?.autoMerge || false,
					targetBranch: cfg?.completion?.targetBranch || 'main',
				},
				export: {
					includeTranscripts: cfg?.export?.includeTranscripts ?? true,
					includeAttachments: cfg?.export?.includeAttachments || false,
					format: cfg?.export?.format || 'tar.gz',
				},
				claude: {
					model: cfg?.claude?.model || 'claude-sonnet-4-20250514',
					thinking: cfg?.claude?.thinking || false,
					maxTurns: cfg?.claude?.maxTurns || 10,
					temperature: cfg?.claude?.temperature || 0.7,
				},
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

	const handleChange = <K extends keyof FormData>(
		section: K,
		field: keyof FormData[K],
		value: unknown
	) => {
		setFormData((prev) => ({
			...prev,
			[section]: {
				...prev[section],
				[field]: value,
			},
		}));
		setHasChanges(true);
	};

	const handleSave = async () => {
		try {
			setSaving(true);

			const automation = create(AutomationConfigSchema, {
				profile: formData.automation.profile,
				autoApprove: formData.automation.autoApprove,
				autoSkip: formData.automation.autoSkip,
			});

			const completion = create(CompletionConfigSchema, {
				action: formData.completion.action,
				autoMerge: formData.completion.autoMerge,
				targetBranch: formData.completion.targetBranch || undefined,
			});

			const exportConfig = create(ExportConfigSchema, {
				includeTranscripts: formData.export.includeTranscripts,
				includeAttachments: formData.export.includeAttachments,
				format: formData.export.format,
			});

			const claude = create(ClaudeConfigSchema, {
				model: formData.claude.model,
				thinking: formData.claude.thinking,
				maxTurns: formData.claude.maxTurns,
				temperature: formData.claude.temperature,
			});

			await configClient.updateConfig(
				create(UpdateConfigRequestSchema, {
					automation,
					completion,
					export: exportConfig,
					claude,
				})
			);

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
			automation: {
				profile: config.automation?.profile || 'auto',
				autoApprove: config.automation?.autoApprove || false,
				autoSkip: config.automation?.autoSkip || false,
			},
			completion: {
				action: config.completion?.action || 'pr',
				autoMerge: config.completion?.autoMerge || false,
				targetBranch: config.completion?.targetBranch || 'main',
			},
			export: {
				includeTranscripts: config.export?.includeTranscripts ?? true,
				includeAttachments: config.export?.includeAttachments || false,
				format: config.export?.format || 'tar.gz',
			},
			claude: {
				model: config.claude?.model || 'claude-sonnet-4-20250514',
				thinking: config.claude?.thinking || false,
				maxTurns: config.claude?.maxTurns || 10,
				temperature: config.claude?.temperature || 0.7,
			},
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

			<Accordion.Root type="multiple" defaultValue={['automation', 'completion']} className="config-accordion">
				{/* Automation */}
				<Accordion.Item value="automation" className="config-section">
					<Accordion.Header>
						<Accordion.Trigger className="config-section-header">
							<h4 className="config-section-title">
								<Icon name="zap" size={18} />
								Automation
							</h4>
							<Icon name="chevron-down" size={16} />
						</Accordion.Trigger>
					</Accordion.Header>
					<Accordion.Content className="config-section-content">
						<div className="config-field-group">
							<div className="config-field">
								<label htmlFor="profile">Profile</label>
								<select
									id="profile"
									value={formData.automation.profile}
									onChange={(e) => handleChange('automation', 'profile', e.target.value)}
									className="input-field"
									style={{ padding: 'var(--space-2)' }}
								>
									<option value="auto">Auto (fully automated)</option>
									<option value="fast">Fast (speed over safety)</option>
									<option value="safe">Safe (AI reviews, human merge)</option>
									<option value="strict">Strict (human gates)</option>
								</select>
							</div>
							<div className="config-field config-checkbox-field">
								<input
									type="checkbox"
									id="auto_approve"
									checked={formData.automation.autoApprove}
									onChange={(e) => handleChange('automation', 'autoApprove', e.target.checked)}
								/>
								<label htmlFor="auto_approve">Auto-Approve Decisions</label>
							</div>
							<div className="config-field config-checkbox-field">
								<input
									type="checkbox"
									id="auto_skip"
									checked={formData.automation.autoSkip}
									onChange={(e) => handleChange('automation', 'autoSkip', e.target.checked)}
								/>
								<label htmlFor="auto_skip">Auto-Skip Blocked Phases</label>
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
									value={formData.completion.action}
									onChange={(e) => handleChange('completion', 'action', e.target.value)}
									className="input-field"
									style={{ padding: 'var(--space-2)' }}
								>
									<option value="pr">Create PR</option>
									<option value="merge">Merge</option>
									<option value="none">None</option>
								</select>
							</div>
							<div className="config-field config-checkbox-field">
								<input
									type="checkbox"
									id="auto_merge"
									checked={formData.completion.autoMerge}
									onChange={(e) => handleChange('completion', 'autoMerge', e.target.checked)}
								/>
								<label htmlFor="auto_merge">Auto-Merge After Finalize</label>
							</div>
							<div className="config-field">
								<label htmlFor="target_branch">Target Branch</label>
								<Input
									id="target_branch"
									value={formData.completion.targetBranch}
									onChange={(e) => handleChange('completion', 'targetBranch', e.target.value)}
									size="sm"
									placeholder="main"
								/>
							</div>
						</div>
					</Accordion.Content>
				</Accordion.Item>

				{/* Export */}
				<Accordion.Item value="export" className="config-section">
					<Accordion.Header>
						<Accordion.Trigger className="config-section-header">
							<h4 className="config-section-title">
								<Icon name="download" size={18} />
								Export
							</h4>
							<Icon name="chevron-down" size={16} />
						</Accordion.Trigger>
					</Accordion.Header>
					<Accordion.Content className="config-section-content">
						<div className="config-field-group">
							<div className="config-field config-checkbox-field">
								<input
									type="checkbox"
									id="include_transcripts"
									checked={formData.export.includeTranscripts}
									onChange={(e) => handleChange('export', 'includeTranscripts', e.target.checked)}
								/>
								<label htmlFor="include_transcripts">Include Transcripts</label>
							</div>
							<div className="config-field config-checkbox-field">
								<input
									type="checkbox"
									id="include_attachments"
									checked={formData.export.includeAttachments}
									onChange={(e) => handleChange('export', 'includeAttachments', e.target.checked)}
								/>
								<label htmlFor="include_attachments">Include Attachments</label>
							</div>
							<div className="config-field">
								<label htmlFor="export_format">Format</label>
								<select
									id="export_format"
									value={formData.export.format}
									onChange={(e) => handleChange('export', 'format', e.target.value)}
									className="input-field"
									style={{ padding: 'var(--space-2)' }}
								>
									<option value="tar.gz">tar.gz (compressed)</option>
									<option value="json">JSON</option>
								</select>
							</div>
						</div>
					</Accordion.Content>
				</Accordion.Item>

				{/* Claude */}
				<Accordion.Item value="claude" className="config-section">
					<Accordion.Header>
						<Accordion.Trigger className="config-section-header">
							<h4 className="config-section-title">
								<Icon name="cpu" size={18} />
								Claude
							</h4>
							<Icon name="chevron-down" size={16} />
						</Accordion.Trigger>
					</Accordion.Header>
					<Accordion.Content className="config-section-content">
						<div className="config-field-group">
							<div className="config-field">
								<label htmlFor="claude_model">Model</label>
								<Input
									id="claude_model"
									value={formData.claude.model}
									onChange={(e) => handleChange('claude', 'model', e.target.value)}
									size="sm"
									placeholder="claude-sonnet-4-20250514"
								/>
							</div>
							<div className="config-field config-checkbox-field">
								<input
									type="checkbox"
									id="claude_thinking"
									checked={formData.claude.thinking}
									onChange={(e) => handleChange('claude', 'thinking', e.target.checked)}
								/>
								<label htmlFor="claude_thinking">Enable Thinking Mode</label>
							</div>
							<div className="config-field">
								<label htmlFor="claude_max_turns">Max Turns</label>
								<Input
									id="claude_max_turns"
									type="number"
									min="1"
									max="100"
									value={formData.claude.maxTurns}
									onChange={(e) => handleChange('claude', 'maxTurns', parseInt(e.target.value) || 10)}
									size="sm"
								/>
							</div>
							<div className="config-field">
								<label htmlFor="claude_temperature">Temperature</label>
								<Input
									id="claude_temperature"
									type="number"
									min="0"
									max="1"
									step="0.1"
									value={formData.claude.temperature}
									onChange={(e) => handleChange('claude', 'temperature', parseFloat(e.target.value) || 0.7)}
									size="sm"
								/>
							</div>
						</div>
					</Accordion.Content>
				</Accordion.Item>
			</Accordion.Root>
		</div>
	);
}
