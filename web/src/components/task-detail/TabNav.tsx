import { Icon, type IconName } from '@/components/ui/Icon';
import './TabNav.css';

export type TabId = 'timeline' | 'changes' | 'transcript' | 'test-results' | 'attachments' | 'comments';

interface TabConfig {
	id: TabId;
	label: string;
	icon: IconName;
}

const TABS: TabConfig[] = [
	{ id: 'timeline', label: 'Timeline', icon: 'clock' },
	{ id: 'changes', label: 'Changes', icon: 'branch' },
	{ id: 'transcript', label: 'Transcript', icon: 'file-text' },
	{ id: 'test-results', label: 'Test Results', icon: 'check-circle' },
	{ id: 'attachments', label: 'Attachments', icon: 'folder' },
	{ id: 'comments', label: 'Comments', icon: 'message-circle' },
];

interface TabNavProps {
	activeTab: TabId;
	onTabChange: (tabId: TabId) => void;
}

export function TabNav({ activeTab, onTabChange }: TabNavProps) {
	return (
		<nav className="tab-nav" role="tablist" aria-label="Task detail tabs">
			{TABS.map((tab) => (
				<button
					key={tab.id}
					role="tab"
					aria-selected={activeTab === tab.id}
					aria-controls={`panel-${tab.id}`}
					className={`tab-btn ${activeTab === tab.id ? 'active' : ''}`}
					onClick={() => onTabChange(tab.id)}
				>
					<Icon name={tab.icon} size={16} />
					<span>{tab.label}</span>
				</button>
			))}
		</nav>
	);
}
