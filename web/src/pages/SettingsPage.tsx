/**
 * SettingsPage - Main wrapper component for the settings view.
 *
 * Renders SettingsTabs providing:
 * - Three top-level tabs: General, Agents, Environment
 * - URL-driven tab state
 * - Tab content via nested routes
 */

import { SettingsTabs } from '@/components/settings/SettingsTabs';
import { useDocumentTitle } from '@/hooks';

export function SettingsPage() {
	useDocumentTitle('Settings');
	return <SettingsTabs />;
}
