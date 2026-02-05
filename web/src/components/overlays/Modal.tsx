/**
 * Modal component for overlays
 * Built on Radix Dialog primitives for accessibility
 * Features:
 * - Portal rendering to document.body
 * - Focus trap (Tab/Shift+Tab cycles within modal)
 * - Escape to close
 * - Click outside to close
 * - Body scroll lock when open
 */

import { useEffect, useRef, type ReactNode } from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import * as VisuallyHidden from '@radix-ui/react-visually-hidden';
import { Icon } from '@/components/ui/Icon';
import './Modal.css';

export type ModalSize = 'sm' | 'md' | 'lg' | 'xl';

interface ModalProps {
	open: boolean;
	onClose: () => void;
	size?: ModalSize;
	title?: ReactNode;
	/** Accessible title for screen readers when no visible title is provided */
	ariaLabel?: string;
	showClose?: boolean;
	children: ReactNode;
	/** Test ID for the modal content element */
	'data-testid'?: string;
}

const sizeClasses: Record<ModalSize, string> = {
	sm: 'max-width-sm',
	md: 'max-width-md',
	lg: 'max-width-lg',
	xl: 'max-width-xl',
};

export function Modal({
	open,
	onClose,
	size = 'md',
	title,
	ariaLabel = 'Dialog',
	showClose = true,
	children,
	'data-testid': dataTestId,
}: ModalProps) {
	const contentRef = useRef<HTMLDivElement>(null);

	// Handle document-level clicks for JSDOM compatibility
	// This complements Radix's onPointerDownOutside which doesn't fire for fireEvent.click
	useEffect(() => {
		if (!open) return;

		// Track when the modal opened to ignore the click that triggered the open.
		// In production, the opening click is still bubbling when this effect runs.
		// We ignore clicks that happen within 10ms of opening (same event loop tick).
		const openTime = performance.now();

		const handleDocumentClick = (e: MouseEvent) => {
			// Ignore clicks that happen immediately after opening (the opening click itself)
			// e.timeStamp uses the same time origin as performance.now() in modern browsers
			if (e.timeStamp - openTime < 10) {
				return;
			}
			// Check if click is outside the modal content
			if (contentRef.current && !contentRef.current.contains(e.target as Node)) {
				onClose();
			}
		};

		document.addEventListener('click', handleDocumentClick);
		return () => document.removeEventListener('click', handleDocumentClick);
	}, [open, onClose]);

	return (
		<Dialog.Root open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
			<Dialog.Portal>
				<Dialog.Overlay className="modal-backdrop" onClick={onClose} />
				<Dialog.Content
					ref={contentRef}
					className={`modal-content ${sizeClasses[size]}`}
					aria-describedby={undefined}
					data-testid={dataTestId}
					onPointerDownOutside={(e) => {
						e.preventDefault();
						onClose();
					}}
					onInteractOutside={(e) => {
						e.preventDefault();
						onClose();
					}}
				>
					{/* Always provide a title for screen readers */}
					{title ? (
						<>
							{(title || showClose) && (
								<div className="modal-header">
									<Dialog.Title className="modal-title">{title}</Dialog.Title>
									{showClose && (
										<Dialog.Close className="modal-close" aria-label="Close modal" title="Close (Esc)">
											<Icon name="close" size={18} />
										</Dialog.Close>
									)}
								</div>
							)}
						</>
					) : (
						<>
							{/* Visually hidden title for accessibility when no visible title */}
							<VisuallyHidden.Root asChild>
								<Dialog.Title>{ariaLabel}</Dialog.Title>
							</VisuallyHidden.Root>
							{showClose && (
								<div className="modal-header modal-header--close-only">
									<Dialog.Close className="modal-close" aria-label="Close modal" title="Close (Esc)">
										<Icon name="close" size={18} />
									</Dialog.Close>
								</div>
							)}
						</>
					)}
					<div className="modal-body">{children}</div>
				</Dialog.Content>
			</Dialog.Portal>
		</Dialog.Root>
	);
}
