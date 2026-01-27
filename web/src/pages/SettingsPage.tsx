/**
 * SettingsPage - Main wrapper component for the settings view.
 *
 * Renders SettingsLayout within AppShell, providing:
 * - 240px sidebar with grouped navigation
 * - Content area with nested route outlet
 */

import { SettingsLayout } from '@/components/settings/SettingsLayout';
import { useDocumentTitle } from '@/hooks';

export function SettingsPage() {
	useDocumentTitle('Settings');
	return <SettingsLayout />;
}
