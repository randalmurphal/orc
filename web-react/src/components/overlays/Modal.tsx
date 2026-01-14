/**
 * Modal component for overlays
 * Features:
 * - Portal rendering to document.body
 * - Focus trap (Tab/Shift+Tab cycles within modal)
 * - Escape to close
 * - Click outside to close
 */

import { useEffect, useCallback, useRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { Icon } from '@/components/ui/Icon';
import './Modal.css';

export type ModalSize = 'sm' | 'md' | 'lg' | 'xl';

interface ModalProps {
	open: boolean;
	onClose: () => void;
	size?: ModalSize;
	title?: string;
	showClose?: boolean;
	children: ReactNode;
}

const sizeClasses: Record<ModalSize, string> = {
	sm: 'max-width-sm',
	md: 'max-width-md',
	lg: 'max-width-lg',
	xl: 'max-width-xl',
};

// Selector for all focusable elements
const FOCUSABLE_SELECTOR =
	'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

export function Modal({
	open,
	onClose,
	size = 'md',
	title,
	showClose = true,
	children,
}: ModalProps) {
	const modalRef = useRef<HTMLDivElement>(null);
	const previousActiveElement = useRef<HTMLElement | null>(null);

	// Handle escape key
	const handleKeydown = useCallback(
		(e: KeyboardEvent) => {
			if (e.key === 'Escape') {
				e.preventDefault();
				onClose();
			}
		},
		[onClose]
	);

	// Focus trap: keep focus within modal
	const handleFocusTrap = useCallback((e: KeyboardEvent) => {
		if (e.key !== 'Tab' || !modalRef.current) return;

		const focusableElements = modalRef.current.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR);
		const firstElement = focusableElements[0];
		const lastElement = focusableElements[focusableElements.length - 1];

		if (!firstElement) return;

		if (e.shiftKey) {
			// Shift+Tab: if at first element, wrap to last
			if (document.activeElement === firstElement) {
				e.preventDefault();
				lastElement?.focus();
			}
		} else {
			// Tab: if at last element, wrap to first
			if (document.activeElement === lastElement) {
				e.preventDefault();
				firstElement?.focus();
			}
		}
	}, []);

	// Setup event listeners and focus management
	useEffect(() => {
		if (!open) return;

		// Store previously focused element to restore later
		previousActiveElement.current = document.activeElement as HTMLElement;

		// Focus the modal or first focusable element
		const focusFirstElement = () => {
			if (!modalRef.current) return;
			const focusableElements = modalRef.current.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR);
			if (focusableElements.length > 0) {
				focusableElements[0].focus();
			} else {
				// If no focusable elements, focus the modal itself
				modalRef.current.focus();
			}
		};

		// Small delay to ensure modal is rendered
		requestAnimationFrame(focusFirstElement);

		// Add event listeners
		window.addEventListener('keydown', handleKeydown);
		window.addEventListener('keydown', handleFocusTrap);

		// Prevent body scroll when modal is open
		const originalOverflow = document.body.style.overflow;
		document.body.style.overflow = 'hidden';

		return () => {
			window.removeEventListener('keydown', handleKeydown);
			window.removeEventListener('keydown', handleFocusTrap);
			document.body.style.overflow = originalOverflow;

			// Restore focus to previously focused element
			if (previousActiveElement.current && previousActiveElement.current.focus) {
				previousActiveElement.current.focus();
			}
		};
	}, [open, handleKeydown, handleFocusTrap]);

	// Handle backdrop click
	const handleBackdropClick = (e: React.MouseEvent<HTMLDivElement>) => {
		if (e.target === e.currentTarget) {
			onClose();
		}
	};

	if (!open) return null;

	const modalContent = (
		<div
			ref={modalRef}
			className="modal-backdrop"
			role="dialog"
			aria-modal="true"
			aria-labelledby={title ? 'modal-title' : undefined}
			tabIndex={-1}
			onClick={handleBackdropClick}
		>
			<div className={`modal-content ${sizeClasses[size]}`}>
				{(title || showClose) && (
					<div className="modal-header">
						{title && (
							<h2 id="modal-title" className="modal-title">
								{title}
							</h2>
						)}
						{showClose && (
							<button
								className="modal-close"
								onClick={onClose}
								aria-label="Close modal"
								title="Close (Esc)"
							>
								<Icon name="close" size={18} />
							</button>
						)}
					</div>
				)}
				<div className="modal-body">{children}</div>
			</div>
		</div>
	);

	// Render via portal to document.body
	return createPortal(modalContent, document.body);
}
