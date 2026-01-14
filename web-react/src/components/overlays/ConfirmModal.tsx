/**
 * ConfirmModal - Generic confirmation dialog
 *
 * Features:
 * - Customizable title, message, and buttons
 * - Action-specific icons and colors
 * - Keyboard: Enter to confirm, Escape to cancel
 * - Loading state during async operations
 */

import { useCallback, useEffect, useRef } from 'react';
import { createPortal } from 'react-dom';
import { Icon, type IconName } from '@/components/ui/Icon';
import './ConfirmModal.css';

export type ConfirmVariant = 'primary' | 'warning' | 'danger';
export type ConfirmAction = 'run' | 'pause' | 'resume' | 'delete' | 'cancel';

interface ConfirmModalProps {
	open: boolean;
	title: string;
	message: string;
	confirmLabel?: string;
	cancelLabel?: string;
	confirmVariant?: ConfirmVariant;
	action?: ConfirmAction;
	loading?: boolean;
	onConfirm: () => void;
	onCancel: () => void;
}

const ACTION_ICONS: Record<ConfirmAction, IconName> = {
	run: 'play',
	pause: 'pause',
	resume: 'play',
	delete: 'trash',
	cancel: 'close',
};

const VARIANT_ICONS: Record<ConfirmVariant, IconName> = {
	primary: 'check',
	warning: 'clock',
	danger: 'close',
};

export function ConfirmModal({
	open,
	title,
	message,
	confirmLabel = 'Confirm',
	cancelLabel = 'Cancel',
	confirmVariant = 'primary',
	action,
	loading = false,
	onConfirm,
	onCancel,
}: ConfirmModalProps) {
	const confirmButtonRef = useRef<HTMLButtonElement>(null);

	// Get icon based on action or variant
	const iconName = action ? ACTION_ICONS[action] : VARIANT_ICONS[confirmVariant];

	// Handle keyboard navigation
	const handleKeyDown = useCallback(
		(e: KeyboardEvent) => {
			if (!open) return;

			if (e.key === 'Escape') {
				e.preventDefault();
				onCancel();
			} else if (e.key === 'Enter' && !loading) {
				e.preventDefault();
				onConfirm();
			}
		},
		[open, loading, onConfirm, onCancel]
	);

	// Focus confirm button when modal opens
	useEffect(() => {
		if (open) {
			// Small delay to ensure modal is rendered
			setTimeout(() => confirmButtonRef.current?.focus(), 50);
		}
	}, [open]);

	// Add keyboard listener
	useEffect(() => {
		if (open) {
			window.addEventListener('keydown', handleKeyDown);
			// Prevent body scroll
			document.body.style.overflow = 'hidden';
		}

		return () => {
			window.removeEventListener('keydown', handleKeyDown);
			document.body.style.overflow = '';
		};
	}, [open, handleKeyDown]);

	// Handle backdrop click
	const handleBackdropClick = (e: React.MouseEvent) => {
		if (e.target === e.currentTarget) {
			onCancel();
		}
	};

	if (!open) return null;

	const modalContent = (
		<div
			className="confirm-backdrop"
			role="dialog"
			aria-modal="true"
			aria-labelledby="confirm-title"
			onClick={handleBackdropClick}
		>
			<div className={`confirm-modal variant-${confirmVariant}`}>
				{/* Icon */}
				<div className={`confirm-icon variant-${confirmVariant}`}>
					<Icon name={iconName} size={24} />
				</div>

				{/* Content */}
				<div className="confirm-content">
					<h2 id="confirm-title" className="confirm-title">
						{title}
					</h2>
					<p className="confirm-message">{message}</p>
				</div>

				{/* Actions */}
				<div className="confirm-actions">
					<button
						type="button"
						className="btn-cancel"
						onClick={onCancel}
						disabled={loading}
					>
						{cancelLabel}
					</button>
					<button
						ref={confirmButtonRef}
						type="button"
						className={`btn-confirm variant-${confirmVariant}`}
						onClick={onConfirm}
						disabled={loading}
					>
						{loading ? (
							<>
								<span className="spinner" />
								Processing...
							</>
						) : (
							confirmLabel
						)}
					</button>
				</div>

				{/* Keyboard hints */}
				<div className="confirm-hints">
					<span>
						<kbd>Enter</kbd> to confirm
					</span>
					<span>
						<kbd>Esc</kbd> to cancel
					</span>
				</div>
			</div>
		</div>
	);

	return createPortal(modalContent, document.body);
}
