import { useState, useEffect, type ReactElement } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Icon } from '@/components/ui';
import type { IconName } from '@/components/ui';
import './NotificationBar.css';

interface Notification {
	id: string;
	type: string;
	title: string;
	message?: string;
	source_type?: string;
	source_id?: string;
	created_at: string;
	expires_at?: string;
}

interface NotificationResponse {
	notifications: Notification[];
}

/**
 * NotificationBar displays active notifications at the top of all pages.
 * Currently supports automation notifications:
 * - automation_pending: Tasks pending approval
 * - automation_failed: Failed automation tasks
 * - automation_blocked: Triggers blocked by cooldown
 */
export function NotificationBar(): ReactElement | null {
	const navigate = useNavigate();
	const [notifications, setNotifications] = useState<Notification[]>([]);
	const [dismissing, setDismissing] = useState<string | null>(null);

	// Fetch notifications on mount and periodically
	useEffect(() => {
		let isMounted = true;

		const fetchNotifications = async () => {
			try {
				const res = await fetch('/api/notifications');
				if (res.ok && isMounted) {
					const data: NotificationResponse = await res.json();
					setNotifications(data.notifications || []);
				}
			} catch (error) {
				if (isMounted) {
					console.error('Failed to fetch notifications:', error);
				}
			}
		};

		fetchNotifications();
		const interval = setInterval(fetchNotifications, 30000); // Refresh every 30s

		return () => {
			isMounted = false;
			clearInterval(interval);
		};
	}, []);

	const dismissNotification = async (id: string) => {
		setDismissing(id);
		try {
			const res = await fetch(`/api/notifications/${id}/dismiss`, {
				method: 'PUT',
			});
			if (res.ok) {
				setNotifications((prev: Notification[]) => prev.filter((n: Notification) => n.id !== id));
			}
		} catch (error) {
			console.error('Failed to dismiss notification:', error);
		} finally {
			setDismissing(null);
		}
	};

	const dismissAll = async () => {
		try {
			const res = await fetch('/api/notifications/dismiss-all', {
				method: 'PUT',
			});
			if (res.ok) {
				setNotifications([]);
			}
		} catch (error) {
			console.error('Failed to dismiss all notifications:', error);
		}
	};

	if (notifications.length === 0) {
		return null;
	}

	// Group notifications by type
	const pending = notifications.filter((n: Notification) => n.type === 'automation_pending');
	const failed = notifications.filter((n: Notification) => n.type === 'automation_failed');
	const blocked = notifications.filter((n: Notification) => n.type === 'automation_blocked');

	const getNotificationIcon = (type: string): IconName => {
		switch (type) {
			case 'automation_pending':
				return 'clock';
			case 'automation_failed':
				return 'error';
			case 'automation_blocked':
				return 'pause';
			default:
				return 'info';
		}
	};

	return (
		<div className="notification-bar">
			{pending.length > 0 && (
				<div className={`notification-item notification-warning`}>
					<Icon name={getNotificationIcon('automation_pending')} size={16} />
					<span className="notification-text">
						{pending.length === 1
							? pending[0].title
							: `${pending.length} automation tasks pending approval`}
					</span>
					<div className="notification-actions">
						<Button
							variant="ghost"
							size="sm"
							onClick={() => navigate('/automation')}
						>
							Review
						</Button>
						{pending.length === 1 ? (
							<Button
								variant="ghost"
								size="sm"
								onClick={() => dismissNotification(pending[0].id)}
								loading={dismissing === pending[0].id}
							>
								Dismiss
							</Button>
						) : (
							<Button variant="ghost" size="sm" onClick={dismissAll}>
								Dismiss All
							</Button>
						)}
					</div>
				</div>
			)}

			{failed.length > 0 && (
				<div className={`notification-item notification-error`}>
					<Icon name={getNotificationIcon('automation_failed')} size={16} />
					<span className="notification-text">
						{failed.length === 1
							? failed[0].title
							: `${failed.length} automation tasks failed`}
					</span>
					<div className="notification-actions">
						<Button
							variant="ghost"
							size="sm"
							onClick={() => navigate('/automation')}
						>
							View Details
						</Button>
						{failed.length === 1 ? (
							<Button
								variant="ghost"
								size="sm"
								onClick={() => dismissNotification(failed[0].id)}
								loading={dismissing === failed[0].id}
							>
								Dismiss
							</Button>
						) : (
							<Button variant="ghost" size="sm" onClick={dismissAll}>
								Dismiss All
							</Button>
						)}
					</div>
				</div>
			)}

			{blocked.length > 0 && (
				<div className={`notification-item notification-info`}>
					<Icon name={getNotificationIcon('automation_blocked')} size={16} />
					<span className="notification-text">
						{blocked.length === 1
							? blocked[0].title
							: `${blocked.length} triggers blocked by cooldown`}
					</span>
					<div className="notification-actions">
						{blocked.length === 1 ? (
							<Button
								variant="ghost"
								size="sm"
								onClick={() => dismissNotification(blocked[0].id)}
								loading={dismissing === blocked[0].id}
							>
								Dismiss
							</Button>
						) : (
							<Button variant="ghost" size="sm" onClick={dismissAll}>
								Dismiss All
							</Button>
						)}
					</div>
				</div>
			)}
		</div>
	);
}
