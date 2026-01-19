/**
 * ConfigPanel component for right panel showing Claude Code configuration quick links.
 *
 * Displays a cyan-themed collapsible section with links to:
 * - Slash Commands (with count badge)
 * - CLAUDE.md (with file size badge)
 * - MCP Servers (with count badge)
 * - Permissions (with profile badge)
 *
 * Each link shows: icon, title, description, badge and navigates to /settings/[section]
 *
 * Reference: example_ui/board.html (.config-item class, lines 640-676)
 * Reference: example_ui/Screenshot_20260116_201804.png (right panel, cyan section)
 */

import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Icon } from '@/components/ui/Icon';
import './ConfigPanel.css';

/** Stats for Claude configuration items */
export interface ConfigStats {
	/** Number of slash commands configured */
	slashCommandsCount?: number;
	/** Size of CLAUDE.md file in bytes */
	claudeMdSize?: number;
	/** Number of MCP servers configured */
	mcpServersCount?: number;
	/** Current permissions profile (e.g., "Auto", "Manual", etc.) */
	permissionsProfile?: string;
	/** Whether stats are still loading */
	loading?: boolean;
}

export interface ConfigPanelProps {
	/** Config statistics for badge display */
	config?: ConfigStats;
}

/** Configuration link definition */
interface ConfigLink {
	id: string;
	icon: 'terminal' | 'file-text' | 'server' | 'shield';
	title: string;
	description: string;
	route: string;
	getBadge: (config?: ConfigStats) => string | undefined;
}

const CONFIG_LINKS: ConfigLink[] = [
	{
		id: 'slash-commands',
		icon: 'terminal',
		title: 'Slash Commands',
		description: '~/.claude/commands',
		route: '/settings/advanced/skills',
		getBadge: (config) =>
			config?.slashCommandsCount !== undefined
				? String(config.slashCommandsCount)
				: undefined,
	},
	{
		id: 'claude-md',
		icon: 'file-text',
		title: 'CLAUDE.md',
		description: 'Project context',
		route: '/settings/advanced/claudemd',
		getBadge: (config) => {
			if (config?.claudeMdSize === undefined) return undefined;
			if (config.claudeMdSize >= 1024) {
				return `${(config.claudeMdSize / 1024).toFixed(1)}K`;
			}
			return `${config.claudeMdSize}`;
		},
	},
	{
		id: 'mcp-servers',
		icon: 'server',
		title: 'MCP Servers',
		description: 'Integrations',
		route: '/settings/advanced/mcp',
		getBadge: (config) =>
			config?.mcpServersCount !== undefined
				? String(config.mcpServersCount)
				: undefined,
	},
	{
		id: 'permissions',
		icon: 'shield',
		title: 'Permissions',
		description: 'Tools & actions',
		route: '/settings/configuration/general',
		getBadge: (config) => config?.permissionsProfile,
	},
];

/**
 * ConfigPanel displays Claude Code configuration quick links in the right panel.
 */
export function ConfigPanel({ config }: ConfigPanelProps) {
	const navigate = useNavigate();
	const [collapsed, setCollapsed] = useState(false);

	const handleToggle = useCallback(() => {
		setCollapsed((prev) => !prev);
	}, []);

	const handleLinkClick = useCallback(
		(route: string) => {
			navigate(route);
		},
		[navigate]
	);

	const isLoading = config?.loading ?? false;

	return (
		<div className={`config-panel panel-section ${collapsed ? 'collapsed' : ''}`}>
			<button
				className="panel-header"
				onClick={handleToggle}
				aria-expanded={!collapsed}
				aria-controls="config-panel-body"
			>
				<div className="panel-title">
					<div className="panel-title-icon cyan">
						<Icon name="code" size={12} />
					</div>
					<span>Claude Config</span>
				</div>
				<Icon
					name={collapsed ? 'chevron-right' : 'chevron-down'}
					size={12}
					className="panel-chevron"
				/>
			</button>

			<div id="config-panel-body" className="panel-body" role="region">
				{CONFIG_LINKS.map((link) => {
					const badge = link.getBadge(config);

					return (
						<button
							key={link.id}
							className="config-item"
							onClick={() => handleLinkClick(link.route)}
							aria-label={`${link.title} - ${link.description}`}
						>
							<div className="config-icon">
								<Icon name={link.icon} size={14} />
							</div>
							<div className="config-content">
								<div className="config-title">{link.title}</div>
								<div className="config-desc">{link.description}</div>
							</div>
							{isLoading ? (
								<div className="config-badge config-badge-loading" aria-label="Loading">
									<span className="loading-skeleton" />
								</div>
							) : (
								badge && <div className="config-badge">{badge}</div>
							)}
							<div className="config-arrow">
								<Icon name="chevron-right" size={12} />
							</div>
						</button>
					);
				})}
			</div>
		</div>
	);
}
