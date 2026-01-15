import { useLocation } from 'react-router-dom';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui';
import { useCurrentProject } from '@/stores';
import { getModifierKey, formatShortcut } from '@/lib/platform';
import './Header.css';

interface HeaderProps {
	onProjectClick?: () => void;
	onNewTask?: () => void;
	onCommandPalette?: () => void;
}

/**
 * Application header with project selector, page title, and action buttons.
 *
 * Features:
 * - Project switcher dropdown button
 * - Page title / breadcrumb
 * - Commands button with keyboard shortcut hint
 * - New Task button
 */
export function Header({ onProjectClick, onNewTask, onCommandPalette }: HeaderProps) {
	const location = useLocation();
	const currentProject = useCurrentProject();
	const modKey = getModifierKey();

	// Derive page title from route
	const pageTitle = getPageTitle(location.pathname);

	return (
		<header className="header">
			<div className="header-left">
				{/* Project Switcher Button */}
				<button
					className="project-btn"
					onClick={onProjectClick}
					title={`Switch project (${modKey}+P)`}
				>
					<span className="project-icon">
						<Icon name="folder" size={16} />
					</span>
					<span className="project-name">{currentProject?.name || 'Select project'}</span>
					<span className="project-chevron">
						<Icon name="chevron-down" size={12} />
					</span>
				</button>

				{/* Page Title / Breadcrumb */}
				<div className="page-info">
					<span className="separator">/</span>
					<h1 className="page-title">{pageTitle}</h1>
				</div>
			</div>

			<div className="header-right">
				{/* Command Palette Hint */}
				<button
					className="cmd-hint"
					onClick={onCommandPalette}
					title={`Command palette (${modKey}+K)`}
				>
					<span className="cmd-hint-label">Commands</span>
					<kbd>{formatShortcut('K')}</kbd>
				</button>

				{/* New Task Button */}
				{onNewTask && (
					<Button
						variant="primary"
						size="sm"
						leftIcon={<Icon name="plus" size={16} />}
						onClick={onNewTask}
					>
						New Task
					</Button>
				)}
			</div>
		</header>
	);
}

function getPageTitle(pathname: string): string {
	if (pathname === '/' || pathname.startsWith('/tasks')) {
		if (pathname.startsWith('/tasks/')) {
			return 'Task Details';
		}
		return 'Tasks';
	}
	const segment = pathname.split('/')[1];
	const titles: Record<string, string> = {
		board: 'Board',
		dashboard: 'Dashboard',
		prompts: 'Prompts',
		claudemd: 'CLAUDE.md',
		skills: 'Skills',
		hooks: 'Hooks',
		mcp: 'MCP Servers',
		tools: 'Tools',
		agents: 'Agents',
		scripts: 'Scripts',
		settings: 'Settings',
		config: 'Configuration',
		preferences: 'Preferences',
		environment: 'Environment',
		initiatives: 'Initiative',
	};
	return titles[segment] || segment;
}
