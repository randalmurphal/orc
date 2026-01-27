/**
 * Preferences page (/preferences)
 *
 * UI preferences for theme, sidebar, board view, and date formats.
 * All preferences persist in localStorage and apply immediately.
 */

import { usePreferencesStore, useDateFormat } from '@/stores';
import { Button, Icon } from '@/components/ui';
import { formatDate } from '@/lib/formatDate';
import { useDocumentTitle } from '@/hooks';
import type { DateFormat } from '@/stores';
import './Preferences.css';

export function Preferences() {
	useDocumentTitle('Preferences');
	const theme = usePreferencesStore((s) => s.theme);
	const sidebarDefault = usePreferencesStore((s) => s.sidebarDefault);
	const boardViewMode = usePreferencesStore((s) => s.boardViewMode);
	const dateFormat = useDateFormat();
	const setTheme = usePreferencesStore((s) => s.setTheme);
	const setSidebarDefault = usePreferencesStore((s) => s.setSidebarDefault);
	const setBoardViewMode = usePreferencesStore((s) => s.setBoardViewMode);
	const setDateFormat = usePreferencesStore((s) => s.setDateFormat);
	const resetToDefaults = usePreferencesStore((s) => s.resetToDefaults);

	// Example dates for preview
	const exampleDate = new Date();
	exampleDate.setHours(exampleDate.getHours() - 3);
	const exampleOldDate = new Date();
	exampleOldDate.setDate(exampleOldDate.getDate() - 5);

	return (
		<div className="page preferences-page">
			<h2>Preferences</h2>
			<p className="page-description">
				Customize your orc experience. Changes take effect immediately.
			</p>

			{/* Appearance Section */}
			<section className="preferences-section" aria-labelledby="appearance-heading">
				<header className="preferences-section-header">
					<Icon name="palette" size={20} />
					<h3 id="appearance-heading">Appearance</h3>
				</header>
				<div className="preferences-section-content">
					<div className="preference-row">
						<div className="preference-label">
							<span>Theme</span>
							<span>Choose between dark and light color schemes</span>
						</div>
						<div className="preference-control">
							<div className="toggle-group" role="group" aria-label="Theme selection">
								<Button
									variant={theme === 'dark' ? 'primary' : 'secondary'}
									size="sm"
									onClick={() => setTheme('dark')}
									leftIcon={<Icon name="moon" size={14} />}
									aria-pressed={theme === 'dark'}
								>
									Dark
								</Button>
								<Button
									variant={theme === 'light' ? 'primary' : 'secondary'}
									size="sm"
									onClick={() => setTheme('light')}
									leftIcon={<Icon name="sun" size={14} />}
									aria-pressed={theme === 'light'}
								>
									Light
								</Button>
							</div>
						</div>
					</div>
				</div>
			</section>

			{/* Layout Section */}
			<section className="preferences-section" aria-labelledby="layout-heading">
				<header className="preferences-section-header">
					<Icon name="layout" size={20} />
					<h3 id="layout-heading">Layout</h3>
				</header>
				<div className="preferences-section-content">
					<div className="preference-row">
						<div className="preference-label">
							<span>Default Sidebar State</span>
							<span>Choose whether the sidebar starts expanded or collapsed</span>
						</div>
						<div className="preference-control">
							<div className="toggle-group" role="group" aria-label="Sidebar default state">
								<Button
									variant={sidebarDefault === 'expanded' ? 'primary' : 'secondary'}
									size="sm"
									onClick={() => setSidebarDefault('expanded')}
									leftIcon={<Icon name="panel-left-open" size={14} />}
									aria-pressed={sidebarDefault === 'expanded'}
								>
									Expanded
								</Button>
								<Button
									variant={sidebarDefault === 'collapsed' ? 'primary' : 'secondary'}
									size="sm"
									onClick={() => setSidebarDefault('collapsed')}
									leftIcon={<Icon name="panel-left-close" size={14} />}
									aria-pressed={sidebarDefault === 'collapsed'}
								>
									Collapsed
								</Button>
							</div>
						</div>
					</div>

					<div className="preference-row">
						<div className="preference-label">
							<span>Default Board View</span>
							<span>Choose the default view mode for the task board</span>
						</div>
						<div className="preference-control">
							<div className="toggle-group" role="group" aria-label="Board view mode">
								<Button
									variant={boardViewMode === 'flat' ? 'primary' : 'secondary'}
									size="sm"
									onClick={() => setBoardViewMode('flat')}
									leftIcon={<Icon name="board" size={14} />}
									aria-pressed={boardViewMode === 'flat'}
								>
									Flat
								</Button>
								<Button
									variant={boardViewMode === 'swimlane' ? 'primary' : 'secondary'}
									size="sm"
									onClick={() => setBoardViewMode('swimlane')}
									leftIcon={<Icon name="layers" size={14} />}
									aria-pressed={boardViewMode === 'swimlane'}
								>
									Swimlane
								</Button>
							</div>
						</div>
					</div>
				</div>
			</section>

			{/* Date & Time Section */}
			<section className="preferences-section" aria-labelledby="datetime-heading">
				<header className="preferences-section-header">
					<Icon name="clock" size={20} />
					<h3 id="datetime-heading">Date & Time</h3>
				</header>
				<div className="preferences-section-content">
					<div className="preference-row">
						<div className="preference-label">
							<span>Date Format</span>
							<span>Choose how dates and times are displayed</span>
						</div>
						<div className="preference-control">
							<select
								className="preference-select"
								value={dateFormat}
								onChange={(e) => setDateFormat(e.target.value as DateFormat)}
								aria-label="Date format"
							>
								<option value="relative">Relative (3h ago)</option>
								<option value="absolute">Absolute (Jan 16, 3:45 PM)</option>
								<option value="absolute24">Absolute 24h (Jan 16, 15:45)</option>
							</select>
						</div>
					</div>

					{/* Date format preview */}
					<div className="preferences-preview">
						<h4>Preview</h4>
						<div className="preview-content">
							<div className="preview-item">
								<span>3 hours ago:</span>
								<span>{formatDate(exampleDate, dateFormat)}</span>
							</div>
							<div className="preview-item">
								<span>5 days ago:</span>
								<span>{formatDate(exampleOldDate, dateFormat)}</span>
							</div>
						</div>
					</div>
				</div>
			</section>

			{/* Actions */}
			<div className="preferences-actions">
				<Button
					variant="secondary"
					onClick={resetToDefaults}
					leftIcon={<Icon name="rotate-ccw" size={14} />}
				>
					Reset to Defaults
				</Button>
			</div>
		</div>
	);
}
