/**
 * Modal component for overlays
 * Built on Radix Dialog primitives for accessibility
 * Features:
 * - Portal rendering to document.body
 * - Focus trap (Tab/Shift+Tab cycles within modal)
 * - Escape to close
 * - Click outside to close (via overlay click)
 * - Body scroll lock when open
 */

import { type ReactNode } from 'react';
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
	return (
		<Dialog.Root open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
			<Dialog.Portal>
				{/* Overlay handles click-outside-to-close explicitly */}
				<Dialog.Overlay className="modal-backdrop" onClick={onClose} />
				<Dialog.Content
					className={`modal-content ${sizeClasses[size]}`}
					aria-describedby={undefined}
					data-testid={dataTestId}
					onPointerDownOutside={(e) => {
						// Prevent Radix's default close behavior.
						// We handle click-outside via the Overlay's onClick instead.
						// This prevents false closes during React state transitions
						// (e.g., when a button unmounts during re-render).
						e.preventDefault();
					}}
					onFocusOutside={(e) => {
						// Prevent focus-escape from closing the modal.
						// Focus can escape temporarily when React unmounts elements during re-render
						// (e.g., clicking a button that changes state, removing the button from DOM).
						e.preventDefault();
					}}
					onInteractOutside={(e) => {
						// Prevent Radix's default dismissal on any outside interaction.
						// We handle click-outside explicitly via Overlay onClick.
						e.preventDefault();
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
