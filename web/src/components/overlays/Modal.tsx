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

import type { ReactNode } from 'react';
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
}: ModalProps) {
	return (
		<Dialog.Root open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
			<Dialog.Portal>
				<Dialog.Overlay className="modal-backdrop" />
				<Dialog.Content
					className={`modal-content ${sizeClasses[size]}`}
					aria-describedby={undefined}
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
