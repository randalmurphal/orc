/**
 * Modal component for overlays
 */

import { useEffect, useCallback, type ReactNode } from 'react';
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

export function Modal({ open, onClose, size = 'md', title, showClose = true, children }: ModalProps) {
	// Handle escape key
	const handleKeydown = useCallback(
		(e: KeyboardEvent) => {
			if (e.key === 'Escape') {
				onClose();
			}
		},
		[onClose]
	);

	useEffect(() => {
		if (open) {
			window.addEventListener('keydown', handleKeydown);
			return () => window.removeEventListener('keydown', handleKeydown);
		}
	}, [open, handleKeydown]);

	// Handle backdrop click
	const handleBackdropClick = (e: React.MouseEvent<HTMLDivElement>) => {
		if (e.target === e.currentTarget) {
			onClose();
		}
	};

	if (!open) return null;

	return (
		<div
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
							<button className="modal-close" onClick={onClose} aria-label="Close modal" title="Close (Esc)">
								<svg
									xmlns="http://www.w3.org/2000/svg"
									width="18"
									height="18"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									strokeWidth="2"
									strokeLinecap="round"
									strokeLinejoin="round"
								>
									<line x1="18" y1="6" x2="6" y2="18" />
									<line x1="6" y1="6" x2="18" y2="18" />
								</svg>
							</button>
						)}
					</div>
				)}
				<div className="modal-body">{children}</div>
			</div>
		</div>
	);
}
