/**
 * ToolPermissions component for controlling agent tool access.
 * Displays a grid of permission toggles for various agent capabilities.
 */

import { useCallback, useState } from 'react';
import {
	FileText,
	FileEdit,
	Terminal,
	Search,
	GitBranch,
	Monitor,
	type LucideIcon,
} from 'lucide-react';
import { Toggle } from '../core/Toggle';
import './ToolPermissions.css';

/** Tool identifiers */
export type ToolId =
	| 'file_read'
	| 'file_write'
	| 'bash_commands'
	| 'web_search'
	| 'git_operations'
	| 'mcp_servers';

/** Permission configuration for a single tool */
interface ToolConfig {
	id: ToolId;
	label: string;
	icon: LucideIcon;
	/** Whether disabling this permission should show a warning */
	critical?: boolean;
}

/** Tool configurations with icons and labels */
const TOOL_CONFIGS: ToolConfig[] = [
	{ id: 'file_read', label: 'File Read', icon: FileText },
	{ id: 'file_write', label: 'File Write', icon: FileEdit, critical: true },
	{ id: 'bash_commands', label: 'Bash Commands', icon: Terminal, critical: true },
	{ id: 'web_search', label: 'Web Search', icon: Search },
	{ id: 'git_operations', label: 'Git Operations', icon: GitBranch },
	{ id: 'mcp_servers', label: 'MCP Servers', icon: Monitor },
];

export interface ToolPermissionsProps {
	/** Current permission states */
	permissions: Record<string, boolean>;
	/** Callback when a permission is toggled */
	onChange: (tool: ToolId, enabled: boolean) => void;
	/** Whether the component is in a loading state */
	loading?: boolean;
	/** Whether to show warnings for critical permissions */
	showWarnings?: boolean;
}

/**
 * ToolPermissions component displays a 3-column grid of permission toggles.
 *
 * @example
 * const [permissions, setPermissions] = useState({
 *   file_read: true,
 *   file_write: true,
 *   bash_commands: true,
 *   web_search: true,
 *   git_operations: true,
 *   mcp_servers: true,
 * });
 *
 * <ToolPermissions
 *   permissions={permissions}
 *   onChange={(tool, enabled) => {
 *     setPermissions(prev => ({ ...prev, [tool]: enabled }));
 *   }}
 * />
 */
export function ToolPermissions({
	permissions,
	onChange,
	loading = false,
	showWarnings = true,
}: ToolPermissionsProps) {
	const [pendingWarning, setPendingWarning] = useState<ToolId | null>(null);

	const handleToggle = useCallback(
		(tool: ToolConfig, enabled: boolean) => {
			// Show warning when disabling critical permissions
			if (!enabled && tool.critical && showWarnings) {
				setPendingWarning(tool.id);
				return;
			}
			onChange(tool.id, enabled);
		},
		[onChange, showWarnings]
	);

	const confirmDisable = useCallback(() => {
		if (pendingWarning) {
			onChange(pendingWarning, false);
			setPendingWarning(null);
		}
	}, [pendingWarning, onChange]);

	const cancelDisable = useCallback(() => {
		setPendingWarning(null);
	}, []);

	return (
		<div className="tool-permissions">
			<div className="tool-permissions__grid">
				{TOOL_CONFIGS.map((tool) => {
					const isEnabled = permissions[tool.id] ?? true;
					const isWarningTarget = pendingWarning === tool.id;

					return (
						<div
							key={tool.id}
							className={`tool-permissions__item ${isWarningTarget ? 'tool-permissions__item--warning' : ''}`}
						>
							<span className="tool-permissions__label">
								<tool.icon size={14} className="tool-permissions__icon" aria-hidden="true" />
								{tool.label}
							</span>
							<Toggle
								checked={isEnabled}
								onChange={(checked) => handleToggle(tool, checked)}
								disabled={loading}
								size="md"
								aria-label={`${tool.label} permission`}
							/>
						</div>
					);
				})}
			</div>

			{pendingWarning && (
				<div className="tool-permissions__warning">
					<div className="tool-permissions__warning-content">
						<span className="tool-permissions__warning-text">
							Disabling{' '}
							<strong>
								{TOOL_CONFIGS.find((t) => t.id === pendingWarning)?.label}
							</strong>{' '}
							may limit agent capabilities. Continue?
						</span>
						<div className="tool-permissions__warning-actions">
							<button
								type="button"
								className="tool-permissions__warning-btn tool-permissions__warning-btn--cancel"
								onClick={cancelDisable}
							>
								Cancel
							</button>
							<button
								type="button"
								className="tool-permissions__warning-btn tool-permissions__warning-btn--confirm"
								onClick={confirmDisable}
							>
								Disable
							</button>
						</div>
					</div>
				</div>
			)}
		</div>
	);
}
