/**
 * ExecutionSettings component for configuring global agent execution parameters.
 * Displays a 2-column grid of setting cards with appropriate controls.
 */

import { forwardRef, type HTMLAttributes } from 'react';
import { Toggle } from '../core/Toggle';
import { Slider } from '../core/Slider';
import { Select, type SelectOption } from '../core/Select';
import './ExecutionSettings.css';

export interface ExecutionSettingsData {
	/** Maximum parallel tasks (1-5) */
	parallelTasks: number;
	/** Auto-approve safe operations */
	autoApprove: boolean;
	/** Default model ID */
	defaultModel: string;
	/** Daily cost limit in dollars (0-100) */
	costLimit: number;
}

export interface ExecutionSettingsProps extends Omit<HTMLAttributes<HTMLDivElement>, 'onChange'> {
	/** Current settings values */
	settings: ExecutionSettingsData;
	/** Callback when a setting changes - receives partial update object */
	onChange: (update: Partial<ExecutionSettingsData>) => void;
	/** Whether settings are being saved */
	isSaving?: boolean;
}

const MODEL_OPTIONS: SelectOption[] = [
	{ value: 'claude-sonnet-4-20250514', label: 'Claude Sonnet 4' },
	{ value: 'claude-opus-4-20250514', label: 'Claude Opus 4' },
	{ value: 'claude-haiku-3-5-20241022', label: 'Claude Haiku 3.5' },
];

/**
 * ExecutionSettings component for global agent configuration.
 *
 * @example
 * <ExecutionSettings
 *   settings={{
 *     parallelTasks: 2,
 *     autoApprove: true,
 *     defaultModel: 'claude-sonnet-4-20250514',
 *     costLimit: 25,
 *   }}
 *   onChange={(update) => setSettings(prev => ({ ...prev, ...update }))}
 *   isSaving={isSaving}
 * />
 */
export const ExecutionSettings = forwardRef<HTMLDivElement, ExecutionSettingsProps>(
	({ settings, onChange, isSaving = false, className = '', ...props }, ref) => {
		const wrapperClasses = ['execution-settings', className].filter(Boolean).join(' ');

		return (
			<div ref={ref} className={wrapperClasses} {...props}>
				{isSaving && (
					<div className="execution-settings__saving" aria-live="polite">
						Saving...
					</div>
				)}

				<div className="settings-grid">
					{/* Parallel Tasks */}
					<div className="setting-card">
						<div className="setting-header">
							<span className="setting-label">Parallel Tasks</span>
						</div>
						<div className="setting-desc">
							Maximum number of tasks to run simultaneously
						</div>
						<Slider
							value={settings.parallelTasks}
							onChange={(value) => onChange({ parallelTasks: value })}
							min={1}
							max={5}
							step={1}
							showValue
							disabled={isSaving}
							aria-label="Parallel Tasks"
						/>
					</div>

					{/* Auto-Approve */}
					<div className="setting-card">
						<div className="setting-header">
							<span className="setting-label">Auto-Approve</span>
							<Toggle
								checked={settings.autoApprove}
								onChange={(checked) => onChange({ autoApprove: checked })}
								disabled={isSaving}
								aria-label="Auto-Approve"
							/>
						</div>
						<div className="setting-desc">
							Automatically approve safe operations without prompting
						</div>
					</div>

					{/* Default Model */}
					<div className="setting-card">
						<div className="setting-header">
							<span className="setting-label">Default Model</span>
						</div>
						<div className="setting-desc">Model to use for new tasks</div>
						<Select
							value={settings.defaultModel}
							onChange={(value) => onChange({ defaultModel: value })}
							options={MODEL_OPTIONS}
							disabled={isSaving}
							aria-label="Default Model"
						/>
					</div>

					{/* Cost Limit */}
					<div className="setting-card">
						<div className="setting-header">
							<span className="setting-label">Cost Limit</span>
						</div>
						<div className="setting-desc">Daily spending limit before pause</div>
						<Slider
							value={settings.costLimit}
							onChange={(value) => onChange({ costLimit: value })}
							min={0}
							max={100}
							step={1}
							showValue
							formatValue={(v) => `$${v}`}
							disabled={isSaving}
							aria-label="Cost Limit"
						/>
					</div>
				</div>
			</div>
		);
	}
);

ExecutionSettings.displayName = 'ExecutionSettings';
