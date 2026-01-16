import * as Tabs from '@radix-ui/react-tabs';
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
	children: (tabId: TabId) => React.ReactNode;
}

/**
 * Tab navigation using Radix Tabs for accessible tab behavior.
 *
 * Uses render prop pattern for tab content:
 * ```tsx
 * <TabNav activeTab={activeTab} onTabChange={setTab}>
 *   {(tabId) => {
 *     switch (tabId) {
 *       case 'timeline': return <TimelineTab />;
 *       // ...
 *     }
 *   }}
 * </TabNav>
 * ```
 */
export function TabNav({ activeTab, onTabChange, children }: TabNavProps) {
	return (
		<Tabs.Root
			value={activeTab}
			onValueChange={(value) => onTabChange(value as TabId)}
			className="tab-nav-root"
		>
			<Tabs.List className="tab-nav" aria-label="Task details tabs">
				{TABS.map((tab) => (
					<Tabs.Trigger
						key={tab.id}
						value={tab.id}
						className="tab-btn"
					>
						<Icon name={tab.icon} size={16} />
						<span>{tab.label}</span>
					</Tabs.Trigger>
				))}
			</Tabs.List>

			<Tabs.Content
				value={activeTab}
				className="tab-content"
			>
				{children(activeTab)}
			</Tabs.Content>
		</Tabs.Root>
	);
}
