import { useState, useEffect, useCallback, useMemo, type KeyboardEvent } from 'react';
import { configClient } from '@/lib/client';
import type { Agent } from '@/gen/orc/v1/config_pb';
import './AgentsPalette.css';

interface AgentsPaletteProps {
	onAgentClick: (agent: Agent) => void;
	onAgentAssign: (agent: Agent) => void;
	selectedNodeId?: string | null;
	readOnly?: boolean;
	defaultCollapsed?: boolean;
}

export function AgentsPalette({
	onAgentClick,
	onAgentAssign,
	selectedNodeId,
	readOnly = false,
	defaultCollapsed = false,
}: AgentsPaletteProps) {
	const [agents, setAgents] = useState<Agent[]>([]);
	const [isLoading, setIsLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [isExpanded, setIsExpanded] = useState(!defaultCollapsed);

	const fetchAgents = useCallback(async () => {
		setIsLoading(true);
		setError(null);
		try {
			const response = await configClient.listAgents({});
			setAgents(response.agents || []);
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Unknown error';
			setError(`Failed to load agents: ${message}`);
		} finally {
			setIsLoading(false);
		}
	}, []);

	useEffect(() => {
		fetchAgents();
	}, [fetchAgents]);

	const { builtinAgents, customAgents } = useMemo(() => {
		const builtin: Agent[] = [];
		const custom: Agent[] = [];
		for (const agent of agents) {
			if (agent.isBuiltin) {
				builtin.push(agent);
			} else {
				custom.push(agent);
			}
		}
		return { builtinAgents: builtin, customAgents: custom };
	}, [agents]);

	const hasPhaseSelected = Boolean(selectedNodeId);

	const handleAgentClick = useCallback(
		(agent: Agent) => {
			if (readOnly) return;
			if (hasPhaseSelected) {
				onAgentAssign(agent);
			} else {
				onAgentClick(agent);
			}
		},
		[hasPhaseSelected, onAgentClick, onAgentAssign, readOnly]
	);

	const handleKeyDown = useCallback(
		(e: KeyboardEvent<HTMLDivElement>, agent: Agent) => {
			if (readOnly) return;
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				handleAgentClick(agent);
			}
		},
		[handleAgentClick, readOnly]
	);

	const toggleExpanded = () => {
		setIsExpanded((prev) => !prev);
	};

	const getAgentAriaLabel = (agent: Agent) => {
		if (readOnly) {
			return `${agent.name} (read-only)`;
		}
		if (hasPhaseSelected) {
			return `Assign ${agent.name} to selected phase`;
		}
		return `View ${agent.name} details`;
	};

	return (
		<div
			className="agents-palette"
			data-testid="agents-palette"
			data-readonly={String(readOnly)}
			aria-busy={isLoading}
		>
			{/* Collapsible Header */}
			<button
				type="button"
				className="agents-palette-header"
				onClick={toggleExpanded}
				aria-expanded={isExpanded}
				aria-controls="agents-palette-content"
			>
				<span className="agents-palette-chevron" data-testid="agents-chevron">
					{isExpanded ? '▾' : '▸'}
				</span>
				<span className="agents-palette-title">Agents</span>
				{!isLoading && !error && (
					<span className="agents-palette-count">{agents.length}</span>
				)}
			</button>

			{/* Content (collapsed/expanded) */}
			<div
				id="agents-palette-content"
				className="agents-palette-content"
				hidden={!isExpanded}
			>
					{/* Loading State */}
					{isLoading && (
						<div className="agents-palette-loading">
							Loading agents...
						</div>
					)}

					{/* Error State */}
					{error && (
						<div className="agents-palette-error">
							<p>{error}</p>
							<button
								type="button"
								className="agents-palette-retry"
								onClick={fetchAgents}
							>
								Retry
							</button>
						</div>
					)}

					{/* Empty State */}
					{!isLoading && !error && agents.length === 0 && (
						<div className="agents-palette-empty">
							No agents available
						</div>
					)}

					{/* Agent Groups */}
					{!isLoading && !error && agents.length > 0 && (
						<div className="agents-palette-list">
							{/* Built-in Agents */}
							{builtinAgents.length > 0 && (
								<div
									className="agents-group"
									data-testid="agents-group-builtin"
								>
									<h4 className="agents-group-header">Built-in</h4>
									{builtinAgents.map((agent) => (
										<AgentCard
											key={agent.id}
											agent={agent}
											onClick={() => handleAgentClick(agent)}
											onKeyDown={(e) => handleKeyDown(e, agent)}
											ariaLabel={getAgentAriaLabel(agent)}
											hasPhaseSelected={hasPhaseSelected}
											readOnly={readOnly}
										/>
									))}
								</div>
							)}

							{/* Custom Agents */}
							{customAgents.length > 0 && (
								<div
									className="agents-group"
									data-testid="agents-group-custom"
								>
									<h4 className="agents-group-header">Custom</h4>
									{customAgents.map((agent) => (
										<AgentCard
											key={agent.id}
											agent={agent}
											onClick={() => handleAgentClick(agent)}
											onKeyDown={(e) => handleKeyDown(e, agent)}
											ariaLabel={getAgentAriaLabel(agent)}
											hasPhaseSelected={hasPhaseSelected}
											readOnly={readOnly}
										/>
									))}
								</div>
							)}
						</div>
					)}
			</div>
		</div>
	);
}

interface AgentCardProps {
	agent: Agent;
	onClick: () => void;
	onKeyDown: (e: KeyboardEvent<HTMLDivElement>) => void;
	ariaLabel: string;
	hasPhaseSelected: boolean;
	readOnly: boolean;
}

function AgentCard({
	agent,
	onClick,
	onKeyDown,
	ariaLabel,
	hasPhaseSelected,
	readOnly,
}: AgentCardProps) {
	const cardClasses = [
		'agent-card',
		readOnly ? 'readonly' : '',
		hasPhaseSelected && !readOnly ? 'assignable' : '',
	]
		.filter(Boolean)
		.join(' ');

	return (
		<div
			className={cardClasses}
			data-testid={`agent-card-${agent.id}`}
			onClick={onClick}
			onKeyDown={onKeyDown}
			tabIndex={0}
			role="button"
			aria-label={ariaLabel}
		>
			{/* Agent Icon */}
			<div className="agent-icon" data-testid="agent-icon">
				{agent.isBuiltin ? '⚙' : '🤖'}
			</div>

			{/* Agent Info */}
			<div className="agent-info">
				<div className="agent-name">{agent.name}</div>
				{agent.description && (
					<div
						className="agent-description truncated"
						data-testid={`agent-description-${agent.id}`}
					>
						{agent.description}
					</div>
				)}
			</div>
		</div>
	);
}
