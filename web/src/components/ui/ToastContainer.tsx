/**
 * ToastContainer component - renders the toast notification queue.
 * Uses uiStore for state management.
 */

import { useCallback } from 'react';
import { createPortal } from 'react-dom';
import { useToasts, useUIStore, type ToastType } from '@/stores';
import { Icon } from './Icon';
import './ToastContainer.css';

// Map toast types to icon names
const toastIcons: Record<ToastType, 'success' | 'error' | 'warning' | 'info'> = {
	success: 'success',
	error: 'error',
	warning: 'warning',
	info: 'info',
};

export function ToastContainer() {
	const toasts = useToasts();
	const dismissToast = useUIStore((state) => state.dismissToast);

	const handleDismiss = useCallback(
		(id: string) => {
			dismissToast(id);
		},
		[dismissToast]
	);

	if (toasts.length === 0) return null;

	const container = (
		<div className="toast-container" role="region" aria-label="Notifications">
			{toasts.map((t) => (
				<div key={t.id} className={`toast toast-${t.type}`} role="alert">
					<div className="toast-icon">
						<Icon name={toastIcons[t.type]} size={18} />
					</div>
					<div className="toast-content">
						{t.title && <div className="toast-title">{t.title}</div>}
						<div className="toast-message">{t.message}</div>
					</div>
					{t.dismissible && (
						<button
							className="toast-dismiss"
							onClick={() => handleDismiss(t.id)}
							aria-label="Dismiss notification"
						>
							<Icon name="close" size={14} />
						</button>
					)}
				</div>
			))}
		</div>
	);

	// Render via portal to document.body
	return createPortal(container, document.body);
}
