/**
 * SettingsPlaceholder component - placeholder for unimplemented settings sections.
 */

import { Icon, type IconName } from '../ui/Icon';
import './SettingsPlaceholder.css';

interface SettingsPlaceholderProps {
	title: string;
	description: string;
	icon: IconName;
}

export function SettingsPlaceholder({ title, description, icon }: SettingsPlaceholderProps) {
	return (
		<div className="settings-placeholder">
			<div className="settings-placeholder__icon">
				<Icon name={icon} size={48} />
			</div>
			<h2 className="settings-placeholder__title">{title}</h2>
			<p className="settings-placeholder__description">{description}</p>
			<p className="settings-placeholder__status">Coming soon</p>
		</div>
	);
}
