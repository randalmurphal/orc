import { useState, useEffect, useCallback } from 'react';
import {
	getConfig,
	updateConfig,
	type Config as ConfigType,
	type ConfigUpdateRequest,
} from '@/lib/api';
import './Config.css';

/**
 * Config page (/environment/config)
 *
 * Manages orc configuration (.orc/config.yaml)
 * - Automation profiles
 * - Timeouts
 * - Git settings
 * - Worktree settings
 * - Completion settings
 */
export function Config() {
	const [config, setConfig] = useState<ConfigType | null>(null);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [success, setSuccess] = useState<string | null>(null);

	// Form state
	const [profile, setProfile] = useState('auto');
	const [gatesDefault, setGatesDefault] = useState('default');
	const [retryEnabled, setRetryEnabled] = useState(true);
	const [retryMax, setRetryMax] = useState(3);
	const [model, setModel] = useState('');
	const [maxIterations, setMaxIterations] = useState(10);
	const [timeout, setTimeout] = useState('');
	const [branchPrefix, setBranchPrefix] = useState('orc/');
	const [commitPrefix, setCommitPrefix] = useState('[orc]');
	const [worktreeEnabled, setWorktreeEnabled] = useState(true);
	const [worktreeDir, setWorktreeDir] = useState('.orc/worktrees');
	const [cleanupOnComplete, setCleanupOnComplete] = useState(true);
	const [cleanupOnFail, setCleanupOnFail] = useState(false);
	const [completionAction, setCompletionAction] = useState('pr');
	const [targetBranch, setTargetBranch] = useState('main');
	const [deleteBranch, setDeleteBranch] = useState(true);
	const [phaseMax, setPhaseMax] = useState('');
	const [turnMax, setTurnMax] = useState('');
	const [idleWarning, setIdleWarning] = useState('');
	const [heartbeatInterval, setHeartbeatInterval] = useState('');
	const [idleTimeout, setIdleTimeout] = useState('');

	const loadConfig = useCallback(async () => {
		setLoading(true);
		setError(null);

		try {
			const configData = await getConfig();
			setConfig(configData);

			// Populate form
			setProfile(configData.profile || 'auto');
			setGatesDefault(configData.automation?.gates_default || 'default');
			setRetryEnabled(configData.automation?.retry_enabled ?? true);
			setRetryMax(configData.automation?.retry_max ?? 3);
			setModel(configData.execution?.model || '');
			setMaxIterations(configData.execution?.max_iterations ?? 10);
			setTimeout(configData.execution?.timeout || '');
			setBranchPrefix(configData.git?.branch_prefix || 'orc/');
			setCommitPrefix(configData.git?.commit_prefix || '[orc]');
			setWorktreeEnabled(configData.worktree?.enabled ?? true);
			setWorktreeDir(configData.worktree?.dir || '.orc/worktrees');
			setCleanupOnComplete(configData.worktree?.cleanup_on_complete ?? true);
			setCleanupOnFail(configData.worktree?.cleanup_on_fail ?? false);
			setCompletionAction(configData.completion?.action || 'pr');
			setTargetBranch(configData.completion?.target_branch || 'main');
			setDeleteBranch(configData.completion?.delete_branch ?? true);
			setPhaseMax(configData.timeouts?.phase_max || '');
			setTurnMax(configData.timeouts?.turn_max || '');
			setIdleWarning(configData.timeouts?.idle_warning || '');
			setHeartbeatInterval(configData.timeouts?.heartbeat_interval || '');
			setIdleTimeout(configData.timeouts?.idle_timeout || '');
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load config');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadConfig();
	}, [loadConfig]);

	const handleSave = async () => {
		setSaving(true);
		setError(null);
		setSuccess(null);

		const update: ConfigUpdateRequest = {
			profile,
			automation: {
				gates_default: gatesDefault,
				retry_enabled: retryEnabled,
				retry_max: retryMax,
			},
			execution: {
				model: model || undefined,
				max_iterations: maxIterations,
				timeout: timeout || undefined,
			},
			git: {
				branch_prefix: branchPrefix,
				commit_prefix: commitPrefix,
			},
			worktree: {
				enabled: worktreeEnabled,
				dir: worktreeDir,
				cleanup_on_complete: cleanupOnComplete,
				cleanup_on_fail: cleanupOnFail,
			},
			completion: {
				action: completionAction,
				target_branch: targetBranch,
				delete_branch: deleteBranch,
			},
			timeouts: {
				phase_max: phaseMax || undefined,
				turn_max: turnMax || undefined,
				idle_warning: idleWarning || undefined,
				heartbeat_interval: heartbeatInterval || undefined,
				idle_timeout: idleTimeout || undefined,
			},
		};

		try {
			await updateConfig(update);
			await loadConfig();
			setSuccess('Configuration saved successfully');
			globalThis.setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to save config');
		} finally {
			setSaving(false);
		}
	};

	const profiles = [
		{ value: 'auto', label: 'Auto', desc: 'Fully automated' },
		{ value: 'fast', label: 'Fast', desc: 'Minimal gates, speed over safety' },
		{ value: 'safe', label: 'Safe', desc: 'AI reviews, human for merge' },
		{ value: 'strict', label: 'Strict', desc: 'Human gates on spec/review/merge' },
	];

	return (
		<div className="config-page">
			<header className="config-header">
				<div className="header-content">
					<div>
						<h1>Orc Configuration</h1>
						<p className="subtitle">
							Manage orchestrator settings in .orc/config.yaml
						</p>
					</div>
					<button className="btn btn-primary" onClick={handleSave} disabled={saving}>
						{saving ? 'Saving...' : 'Save'}
					</button>
				</div>
			</header>

			{error && <div className="alert alert-error">{error}</div>}
			{success && <div className="alert alert-success">{success}</div>}

			{loading ? (
				<div className="loading-state">Loading configuration...</div>
			) : (
				<div className="config-sections">
					{/* Automation Profile */}
					<section className="config-section">
						<h2>Automation Profile</h2>
						<div className="profile-grid">
							{profiles.map((p) => (
								<label
									key={p.value}
									className={`profile-card ${profile === p.value ? 'selected' : ''}`}
								>
									<input
										type="radio"
										name="profile"
										value={p.value}
										checked={profile === p.value}
										onChange={(e) => setProfile(e.target.value)}
									/>
									<div className="profile-info">
										<span className="profile-name">{p.label}</span>
										<span className="profile-desc">{p.desc}</span>
									</div>
								</label>
							))}
						</div>
					</section>

					{/* Gates & Retry */}
					<section className="config-section">
						<h2>Gates & Retry</h2>
						<div className="form-grid">
							<div className="form-group">
								<label htmlFor="gates-default">Default Gates</label>
								<select
									id="gates-default"
									value={gatesDefault}
									onChange={(e) => setGatesDefault(e.target.value)}
								>
									<option value="default">Default</option>
									<option value="none">None</option>
									<option value="spec">Spec only</option>
									<option value="review">Review only</option>
									<option value="all">All</option>
								</select>
							</div>

							<div className="form-group">
								<label htmlFor="retry-max">Max Retries</label>
								<input
									id="retry-max"
									type="number"
									min={0}
									max={10}
									value={retryMax}
									onChange={(e) => setRetryMax(parseInt(e.target.value) || 0)}
								/>
							</div>

							<div className="form-group form-group-checkbox">
								<label>
									<input
										type="checkbox"
										checked={retryEnabled}
										onChange={(e) => setRetryEnabled(e.target.checked)}
									/>
									Enable automatic retry on failure
								</label>
							</div>
						</div>
					</section>

					{/* Execution */}
					<section className="config-section">
						<h2>Execution</h2>
						<div className="form-grid">
							<div className="form-group">
								<label htmlFor="model">Model</label>
								<input
									id="model"
									type="text"
									value={model}
									onChange={(e) => setModel(e.target.value)}
									placeholder="claude-sonnet-4-20250514"
								/>
							</div>

							<div className="form-group">
								<label htmlFor="max-iterations">Max Iterations</label>
								<input
									id="max-iterations"
									type="number"
									min={1}
									max={100}
									value={maxIterations}
									onChange={(e) => setMaxIterations(parseInt(e.target.value) || 1)}
								/>
							</div>

							<div className="form-group">
								<label htmlFor="timeout">Timeout</label>
								<input
									id="timeout"
									type="text"
									value={timeout}
									onChange={(e) => setTimeout(e.target.value)}
									placeholder="30m"
								/>
								<span className="form-hint">Go duration format (e.g., 30m, 1h)</span>
							</div>
						</div>
					</section>

					{/* Git */}
					<section className="config-section">
						<h2>Git Settings</h2>
						<div className="form-grid">
							<div className="form-group">
								<label htmlFor="branch-prefix">Branch Prefix</label>
								<input
									id="branch-prefix"
									type="text"
									value={branchPrefix}
									onChange={(e) => setBranchPrefix(e.target.value)}
									placeholder="orc/"
								/>
							</div>

							<div className="form-group">
								<label htmlFor="commit-prefix">Commit Prefix</label>
								<input
									id="commit-prefix"
									type="text"
									value={commitPrefix}
									onChange={(e) => setCommitPrefix(e.target.value)}
									placeholder="[orc]"
								/>
							</div>
						</div>
					</section>

					{/* Worktree */}
					<section className="config-section">
						<h2>Worktree Settings</h2>
						<div className="form-grid">
							<div className="form-group form-group-checkbox">
								<label>
									<input
										type="checkbox"
										checked={worktreeEnabled}
										onChange={(e) => setWorktreeEnabled(e.target.checked)}
									/>
									Enable git worktrees for task isolation
								</label>
							</div>

							<div className="form-group">
								<label htmlFor="worktree-dir">Worktree Directory</label>
								<input
									id="worktree-dir"
									type="text"
									value={worktreeDir}
									onChange={(e) => setWorktreeDir(e.target.value)}
									disabled={!worktreeEnabled}
								/>
							</div>

							<div className="form-group form-group-checkbox">
								<label>
									<input
										type="checkbox"
										checked={cleanupOnComplete}
										onChange={(e) => setCleanupOnComplete(e.target.checked)}
										disabled={!worktreeEnabled}
									/>
									Cleanup worktree on completion
								</label>
							</div>

							<div className="form-group form-group-checkbox">
								<label>
									<input
										type="checkbox"
										checked={cleanupOnFail}
										onChange={(e) => setCleanupOnFail(e.target.checked)}
										disabled={!worktreeEnabled}
									/>
									Cleanup worktree on failure
								</label>
							</div>
						</div>
					</section>

					{/* Completion */}
					<section className="config-section">
						<h2>Completion Settings</h2>
						<div className="form-grid">
							<div className="form-group">
								<label htmlFor="completion-action">Completion Action</label>
								<select
									id="completion-action"
									value={completionAction}
									onChange={(e) => setCompletionAction(e.target.value)}
								>
									<option value="pr">Create PR</option>
									<option value="merge">Merge directly</option>
									<option value="none">None</option>
								</select>
							</div>

							<div className="form-group">
								<label htmlFor="target-branch">Target Branch</label>
								<input
									id="target-branch"
									type="text"
									value={targetBranch}
									onChange={(e) => setTargetBranch(e.target.value)}
									placeholder="main"
								/>
							</div>

							<div className="form-group form-group-checkbox">
								<label>
									<input
										type="checkbox"
										checked={deleteBranch}
										onChange={(e) => setDeleteBranch(e.target.checked)}
									/>
									Delete branch after merge
								</label>
							</div>
						</div>
					</section>

					{/* Timeouts */}
					<section className="config-section">
						<h2>Timeouts</h2>
						<div className="form-grid">
							<div className="form-group">
								<label htmlFor="phase-max">Phase Max</label>
								<input
									id="phase-max"
									type="text"
									value={phaseMax}
									onChange={(e) => setPhaseMax(e.target.value)}
									placeholder="30m"
								/>
							</div>

							<div className="form-group">
								<label htmlFor="turn-max">Turn Max</label>
								<input
									id="turn-max"
									type="text"
									value={turnMax}
									onChange={(e) => setTurnMax(e.target.value)}
									placeholder="5m"
								/>
							</div>

							<div className="form-group">
								<label htmlFor="idle-warning">Idle Warning</label>
								<input
									id="idle-warning"
									type="text"
									value={idleWarning}
									onChange={(e) => setIdleWarning(e.target.value)}
									placeholder="2m"
								/>
							</div>

							<div className="form-group">
								<label htmlFor="heartbeat-interval">Heartbeat Interval</label>
								<input
									id="heartbeat-interval"
									type="text"
									value={heartbeatInterval}
									onChange={(e) => setHeartbeatInterval(e.target.value)}
									placeholder="10s"
								/>
							</div>

							<div className="form-group">
								<label htmlFor="idle-timeout">Idle Timeout</label>
								<input
									id="idle-timeout"
									type="text"
									value={idleTimeout}
									onChange={(e) => setIdleTimeout(e.target.value)}
									placeholder="5m"
								/>
							</div>
						</div>
					</section>
				</div>
			)}
		</div>
	);
}
