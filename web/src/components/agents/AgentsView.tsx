/**
 * AgentsView container component - assembles the complete agents
 * configuration page with agent cards grid, execution settings,
 * and tool permissions sections.
 */

import { useState, useEffect, useCallback } from 'react';
import { configClient } from '@/lib/client';
import type { Agent as ProtoAgent, Config } from '@/gen/orc/v1/config_pb';
import { AgentCard, type Agent, type AgentStatus, type IconColor } from './AgentCard';
import { ExecutionSettings, type ExecutionSettingsData } from './ExecutionSettings';
import { ToolPermissions, type ToolId } from './ToolPermissions';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import './AgentsView.css';

// =============================================================================
// Types
// =============================================================================

export interface AgentsViewProps {
	className?: string;
}

// =============================================================================
// Skeleton Components
// =============================================================================

function AgentCardSkeleton() {
	return (
		<article className="agents-view-card-skeleton" aria-hidden="true">
			<div className="agents-view-card-skeleton-header">
				<div className="agents-view-card-skeleton-icon" />
				<div className="agents-view-card-skeleton-info">
					<div className="agents-view-card-skeleton-title" />
					<div className="agents-view-card-skeleton-model" />
				</div>
				<div className="agents-view-card-skeleton-badge" />
			</div>
			<div className="agents-view-card-skeleton-stats">
				<div className="agents-view-card-skeleton-stat" />
				<div className="agents-view-card-skeleton-stat" />
				<div className="agents-view-card-skeleton-stat" />
			</div>
			<div className="agents-view-card-skeleton-tools">
				<div className="agents-view-card-skeleton-tool" />
				<div className="agents-view-card-skeleton-tool" />
				<div className="agents-view-card-skeleton-tool" />
			</div>
		</article>
	);
}

function AgentsViewSkeleton() {
	return (
		<div className="agents-view-grid" aria-busy="true" aria-label="Loading agents">
			<AgentCardSkeleton />
			<AgentCardSkeleton />
			<AgentCardSkeleton />
		</div>
	);
}

// =============================================================================
// Empty State
// =============================================================================

function AgentsViewEmpty() {
	return (
		<div className="agents-view-empty" role="status">
			<div className="agents-view-empty-icon">
				<Icon name="agents" size={32} />
			</div>
			<h2 className="agents-view-empty-title">Create your first agent</h2>
			<p className="agents-view-empty-desc">
				Agents are configured Claude instances with specific models, token limits, and tool
				permissions for different tasks.
			</p>
		</div>
	);
}

// =============================================================================
// Error State
// =============================================================================

interface AgentsViewErrorProps {
	error: string;
	onRetry: () => void;
}

function AgentsViewError({ error, onRetry }: AgentsViewErrorProps) {
	return (
		<div className="agents-view-error" role="alert">
			<div className="agents-view-error-icon">
				<Icon name="alert-circle" size={24} />
			</div>
			<h2 className="agents-view-error-title">Failed to load agents</h2>
			<p className="agents-view-error-desc">{error}</p>
			<Button variant="secondary" onClick={onRetry}>
				Retry
			</Button>
		</div>
	);
}

// =============================================================================
// Helper Functions
// =============================================================================

const ICON_COLORS: IconColor[] = ['purple', 'blue', 'green', 'amber'];
const AGENT_EMOJIS = ['🧠', '🔧', '📝', '🔍', '🚀', '💡'];

function protoAgentToAgent(protoAgent: ProtoAgent, index: number): Agent {
	// Derive icon color based on index
	const iconColor = ICON_COLORS[index % ICON_COLORS.length];
	const emoji = AGENT_EMOJIS[index % AGENT_EMOJIS.length];

	// Extract tools from protoAgent
	const tools = protoAgent.tools?.allow ?? [];

	return {
		id: protoAgent.name,
		name: protoAgent.name,
		model: protoAgent.model || 'default',
		status: 'idle' as AgentStatus,
		emoji,
		iconColor,
		stats: {
			tokensToday: 0,
			tasksDone: 0,
			successRate: 100,
		},
		tools,
	};
}

function configToExecutionSettings(config: Config): ExecutionSettingsData {
	return {
		parallelTasks: 2, // Default, not in current config
		autoApprove: config.automation?.profile === 'auto',
		defaultModel: config.claude?.model || 'claude-sonnet-4-20250514',
		costLimit: 25, // Default, not in current config
	};
}

// =============================================================================
// AgentsView Component
// =============================================================================

/**
 * AgentsView displays all agent configurations with execution settings.
 *
 * @example
 * <AgentsView />
 */
export function AgentsView({ className = '' }: AgentsViewProps) {
	const [agents, setAgents] = useState<Agent[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [executionSettings, setExecutionSettings] = useState<ExecutionSettingsData>({
		parallelTasks: 2,
		autoApprove: false,
		defaultModel: 'claude-sonnet-4-20250514',
		costLimit: 25,
	});
	const [isSaving, setIsSaving] = useState(false);
	const [toolPermissions, setToolPermissions] = useState<Record<string, boolean>>({
		file_read: true,
		file_write: true,
		bash_commands: true,
		web_search: true,
		git_operations: true,
		mcp_servers: true,
	});

	// Load agents and config from API
	const loadData = useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			const [agentsResponse, configResponse] = await Promise.all([
				configClient.listAgents({}),
				configClient.getConfig({}),
			]);
			// Transform ProtoAgent[] to Agent[]
			setAgents(agentsResponse.agents.map((a, i) => protoAgentToAgent(a, i)));
			if (configResponse.config) {
				setExecutionSettings(configToExecutionSettings(configResponse.config));
			}
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load agents');
		} finally {
			setLoading(false);
		}
	}, []);

	// Initial load
	useEffect(() => {
		loadData();
	}, [loadData]);

	// Handle new agent button click
	const handleAddAgent = useCallback(() => {
		window.dispatchEvent(new CustomEvent('orc:add-agent'));
	}, []);

	// Handle agent card selection
	const handleSelectAgent = useCallback((agent: Agent) => {
		window.dispatchEvent(new CustomEvent('orc:select-agent', { detail: { agent } }));
	}, []);

	// Handle execution settings change
	const handleSettingsChange = useCallback(
		async (update: Partial<ExecutionSettingsData>) => {
			setExecutionSettings((prev) => ({ ...prev, ...update }));

			// Persist to API
			setIsSaving(true);
			try {
				if (update.defaultModel) {
					await configClient.updateConfig({
						claude: { model: update.defaultModel },
					});
				}
				// Note: autoApprove would need profile update via automation.profile
			} catch {
				// Silently fail - settings will revert on reload
			} finally {
				setIsSaving(false);
			}
		},
		[]
	);

	// Handle tool permission change
	const handlePermissionChange = useCallback((tool: ToolId, enabled: boolean) => {
		setToolPermissions((prev) => ({ ...prev, [tool]: enabled }));
	}, []);

	const classes = ['agents-view', className].filter(Boolean).join(' ');

	return (
		<div className={classes}>
			<header className="agents-view-header">
				<div className="agents-view-header-text">
					<h1 className="agents-view-title">Agents</h1>
					<p className="agents-view-subtitle">
						Configure Claude models and execution settings
					</p>
				</div>
				<Button
					variant="primary"
					leftIcon={<Icon name="plus" size={12} />}
					onClick={handleAddAgent}
					disabled
					title="Coming soon"
				>
					Add Agent
				</Button>
			</header>

			<div className="agents-view-content">
				{/* Active Agents Section */}
				<section className="agents-view-section">
					<div className="agents-view-section-header">
						<h2 className="section-title">Active Agents</h2>
						<p className="section-subtitle">Currently configured Claude instances</p>
					</div>

					{loading && <AgentsViewSkeleton />}

					{!loading && error && <AgentsViewError error={error} onRetry={loadData} />}

					{!loading && !error && agents.length === 0 && <AgentsViewEmpty />}

					{!loading && !error && agents.length > 0 && (
						<div className="agents-view-grid">
							{agents.map((agent) => (
								<AgentCard
									key={agent.id}
									agent={agent}
									onSelect={handleSelectAgent}
								/>
							))}
						</div>
					)}
				</section>

				{/* Execution Settings Section */}
				<section className="agents-view-section">
					<div className="agents-view-section-header">
						<h2 className="section-title">Execution Settings</h2>
						<p className="section-subtitle">Global configuration for all agents</p>
					</div>
					<ExecutionSettings
						settings={executionSettings}
						onChange={handleSettingsChange}
						isSaving={isSaving}
					/>
				</section>

				{/* Tool Permissions Section */}
				<section className="agents-view-section">
					<div className="agents-view-section-header">
						<h2 className="section-title">Tool Permissions</h2>
						<p className="section-subtitle">Control what actions agents can perform</p>
					</div>
					<ToolPermissions
						permissions={toolPermissions}
						onChange={handlePermissionChange}
						loading={loading}
					/>
				</section>
			</div>
		</div>
	);
}
