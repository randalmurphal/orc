import { useState, useEffect, type ReactElement } from 'react';
import { useNavigate } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { Button, Icon } from '@/components/ui';
import type { IconName } from '@/components/ui';
import { notificationClient } from '@/lib/client';
import { timestampToDate } from '@/lib/time';
import {
	ListNotificationsRequestSchema,
	DismissNotificationRequestSchema,
	DismissAllNotificationsRequestSchema,
	type Notification,
} from '@/gen/orc/v1/notification_pb';
import { useCurrentProjectId } from '@/stores';
import './NotificationBar.css';

// Local type for working with notifications (convert proto timestamps to Date)
interface NotificationData {
	id: string;
	type: string;
	title: string;
	message?: string;
	sourceType?: string;
	sourceId?: string;
	createdAt: Date | null;
	expiresAt?: Date | null;
}

function notificationToData(n: Notification): NotificationData {
	return {
		id: n.id,
		type: n.type,
		title: n.title,
		message: n.message,
		sourceType: n.sourceType,
		sourceId: n.sourceId,
		createdAt: timestampToDate(n.createdAt),
		expiresAt: n.expiresAt ? timestampToDate(n.expiresAt) : undefined,
	};
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
	const projectId = useCurrentProjectId();
	const [notifications, setNotifications] = useState<NotificationData[]>([]);
	const [dismissing, setDismissing] = useState<string | null>(null);

	// Fetch notifications on mount and periodically
	useEffect(() => {
		let isMounted = true;

		const fetchNotifications = async () => {
			try {
				const response = await notificationClient.listNotifications(
					create(ListNotificationsRequestSchema, { projectId: projectId ?? '' })
				);
				if (isMounted) {
					setNotifications(response.notifications.map(notificationToData));
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
	}, [projectId]);

	const handleDismissNotification = async (id: string) => {
		setDismissing(id);
		try {
			await notificationClient.dismissNotification(
				create(DismissNotificationRequestSchema, { id })
			);
			setNotifications((prev) => prev.filter((n) => n.id !== id));
		} catch (error) {
			console.error('Failed to dismiss notification:', error);
		} finally {
			setDismissing(null);
		}
	};

	const dismissAll = async () => {
		try {
			await notificationClient.dismissAllNotifications(
				create(DismissAllNotificationsRequestSchema, {})
			);
			setNotifications([]);
		} catch (error) {
			console.error('Failed to dismiss all notifications:', error);
		}
	};

	if (notifications.length === 0) {
		return null;
	}

	// Group notifications by type
	const pending = notifications.filter((n) => n.type === 'automation_pending');
	const failed = notifications.filter((n) => n.type === 'automation_failed');
	const blocked = notifications.filter((n) => n.type === 'automation_blocked');

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
								onClick={() => handleDismissNotification(pending[0].id)}
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
								onClick={() => handleDismissNotification(failed[0].id)}
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
								onClick={() => handleDismissNotification(blocked[0].id)}
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
