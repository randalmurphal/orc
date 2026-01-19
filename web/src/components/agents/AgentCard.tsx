/**
 * AgentCard component - displays individual AI agent configurations.
 * Shows agent identity (emoji, name, model), status, usage statistics,
 * and enabled tools in a visually distinct format that supports active/selected states.
 */

import {
	forwardRef,
	type HTMLAttributes,
	type KeyboardEvent,
	type MouseEvent,
} from 'react';
import { Badge } from '../core/Badge';
import { formatLargeNumber } from '../core/Stat';
import './AgentCard.css';

// =============================================================================
// Types
// =============================================================================

export type AgentStatus = 'active' | 'idle';
export type IconColor = 'purple' | 'blue' | 'green' | 'amber';

export interface AgentStats {
	/** Token usage for today */
	tokensToday: number;
	/** Number of tasks completed */
	tasksDone: number;
	/** Success rate as percentage (0-100) */
	successRate: number;
	/** Optional custom label for middle stat (e.g., "Reviews" instead of "Tasks Done") */
	tasksDoneLabel?: string;
}

export interface Agent {
	/** Unique identifier */
	id: string;
	/** Display name */
	name: string;
	/** Model identifier (e.g., "claude-sonnet-4-20250514") */
	model: string;
	/** Current status */
	status: AgentStatus;
	/** Emoji icon for the agent */
	emoji: string;
	/** Icon background color */
	iconColor: IconColor;
	/** Agent statistics */
	stats: AgentStats;
	/** List of enabled tools */
	tools: string[];
	/** Whether the agent is disabled */
	disabled?: boolean;
}

export interface AgentCardProps extends Omit<HTMLAttributes<HTMLDivElement>, 'onClick' | 'onSelect'> {
	/** Agent data to display */
	agent: Agent;
	/** Whether this card is currently selected */
	isActive?: boolean;
	/** Called when the card is clicked or activated via keyboard */
	onSelect?: (agent: Agent) => void;
	/** Maximum number of tools to show before truncating */
	maxToolsDisplayed?: number;
}

// =============================================================================
// Constants
// =============================================================================

const DEFAULT_MAX_TOOLS = 4;

// =============================================================================
// Component
// =============================================================================

/**
 * AgentCard component for displaying agent configurations.
 *
 * @example
 * // Basic usage
 * <AgentCard
 *   agent={{
 *     id: 'primary',
 *     name: 'Primary Coder',
 *     model: 'claude-sonnet-4-20250514',
 *     status: 'active',
 *     emoji: '🧠',
 *     iconColor: 'purple',
 *     stats: { tokensToday: 847000, tasksDone: 34, successRate: 94 },
 *     tools: ['File Read/Write', 'Bash', 'Web Search', 'MCP'],
 *   }}
 *   onSelect={(agent) => console.log('Selected:', agent.name)}
 * />
 *
 * @example
 * // Active/selected card
 * <AgentCard agent={agent} isActive onSelect={handleSelect} />
 *
 * @example
 * // Disabled agent
 * <AgentCard agent={{ ...agent, disabled: true }} />
 */
export const AgentCard = forwardRef<HTMLDivElement, AgentCardProps>(
	(
		{
			agent,
			isActive = false,
			onSelect,
			maxToolsDisplayed = DEFAULT_MAX_TOOLS,
			className = '',
			onKeyDown,
			...props
		},
		ref
	) => {
		const { name, model, status, emoji, iconColor, stats, tools, disabled } = agent;
		const isInteractive = Boolean(onSelect) && !disabled;

		// Handle click
		const handleClick = (_event: MouseEvent<HTMLDivElement>) => {
			if (isInteractive) {
				onSelect?.(agent);
			}
		};

		// Handle keyboard activation
		const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
			onKeyDown?.(event);

			if (isInteractive && (event.key === 'Enter' || event.key === ' ')) {
				event.preventDefault();
				onSelect?.(agent);
			}
		};

		// Build class list
		const classes = [
			'agent-card',
			isActive && 'agent-card-active',
			disabled && 'agent-card-disabled',
			isInteractive && 'agent-card-interactive',
			className,
		]
			.filter(Boolean)
			.join(' ');

		// Format stats
		const formattedTokens = formatLargeNumber(stats.tokensToday);
		const formattedTasksDone = String(stats.tasksDone);
		const formattedSuccessRate = `${stats.successRate}%`;
		const tasksDoneLabel = stats.tasksDoneLabel ?? 'Tasks Done';

		// Tool truncation
		const visibleTools = tools.slice(0, maxToolsDisplayed);
		const hiddenToolCount = tools.length - maxToolsDisplayed;

		return (
			<div
				ref={ref}
				className={classes}
				onClick={handleClick}
				onKeyDown={handleKeyDown}
				role={isInteractive ? 'button' : undefined}
				tabIndex={isInteractive ? 0 : undefined}
				aria-pressed={isInteractive ? isActive : undefined}
				aria-label={`${name} agent, ${status}, ${formattedTokens} tokens today, ${stats.tasksDone} tasks done, ${stats.successRate}% success rate`}
				aria-disabled={disabled}
				{...props}
			>
				{/* Header */}
				<div className="agent-card-header">
					<div className={`agent-card-icon agent-card-icon-${iconColor}`}>
						<span role="img" aria-hidden="true">
							{emoji}
						</span>
					</div>
					<div className="agent-card-info">
						<div className="agent-card-name">{name}</div>
						<div className="agent-card-model">{model}</div>
					</div>
					<Badge variant="status" status={status}>
						{status}
					</Badge>
				</div>

				{/* Stats */}
				<div className="agent-card-stats">
					<div className="agent-card-stat">
						<div className="agent-card-stat-value">{formattedTokens}</div>
						<div className="agent-card-stat-label">Tokens Today</div>
					</div>
					<div className="agent-card-stat">
						<div className="agent-card-stat-value">{formattedTasksDone}</div>
						<div className="agent-card-stat-label">{tasksDoneLabel}</div>
					</div>
					<div className="agent-card-stat">
						<div className="agent-card-stat-value">{formattedSuccessRate}</div>
						<div className="agent-card-stat-label">Success</div>
					</div>
				</div>

				{/* Tools */}
				{tools.length > 0 && (
					<div className="agent-card-tools">
						{visibleTools.map((tool) => (
							<Badge key={tool} variant="tool">
								{tool}
							</Badge>
						))}
						{hiddenToolCount > 0 && (
							<Badge variant="tool" className="agent-card-tools-more">
								+{hiddenToolCount} more
							</Badge>
						)}
					</div>
				)}
			</div>
		);
	}
);

AgentCard.displayName = 'AgentCard';
