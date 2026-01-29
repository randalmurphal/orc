/**
 * DeletePhaseDialog - Confirmation dialog for phase deletion (SC-4, SC-5)
 *
 * Features:
 * - Shows phase name in confirmation message
 * - Cancel and Remove Phase buttons
 * - Loading state during delete operation
 * - Escape key to cancel
 * - Destructive button styling for confirm action
 */

import { Modal } from '@/components/overlays/Modal';
import { Button } from '@/components/ui/Button';
import './DeletePhaseDialog.css';

interface DeletePhaseDialogProps {
	open: boolean;
	phaseName: string;
	onConfirm: () => void;
	onCancel: () => void;
	loading?: boolean;
}

export function DeletePhaseDialog({
	open,
	phaseName,
	onConfirm,
	onCancel,
	loading = false,
}: DeletePhaseDialogProps) {
	if (!open) {
		return null;
	}

	return (
		<Modal
			open={open}
			onClose={onCancel}
			title="Confirm Deletion"
			size="sm"
		>
			<div className="delete-phase-dialog">
				<p className="delete-phase-dialog__message">
					Remove phase{' '}
					<strong className="delete-phase-dialog__phase-name">{phaseName}</strong>?
					This action cannot be undone.
				</p>
				<div className="delete-phase-dialog__actions">
					<Button
						variant="secondary"
						onClick={onCancel}
						disabled={loading}
						tabIndex={0}
					>
						Cancel
					</Button>
					<Button
						variant="danger"
						onClick={onConfirm}
						loading={loading}
						disabled={loading}
						tabIndex={0}
						aria-label="Remove Phase"
					>
						Delete
					</Button>
				</div>
			</div>
		</Modal>
	);
}
