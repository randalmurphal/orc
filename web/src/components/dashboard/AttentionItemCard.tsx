import type { AttentionItem } from '@/gen/orc/v1/attention_dashboard_pb';
import {
	AttentionAction,
	AttentionItemType,
} from '@/gen/orc/v1/attention_dashboard_pb';

export interface AttentionItemCardProps {
	item: AttentionItem;
	projectName: string;
	pendingAction?: AttentionAction;
	onOpen: (item: AttentionItem) => void;
	onAction: (item: AttentionItem, action: AttentionAction, decisionOptionId?: string) => Promise<void>;
}

function actionLabel(action: AttentionAction): string {
	switch (action) {
		case AttentionAction.APPROVE:
			return 'Approve';
		case AttentionAction.REJECT:
			return 'Reject';
		case AttentionAction.SKIP:
			return 'Skip';
		case AttentionAction.FORCE:
			return 'Force';
		case AttentionAction.RETRY:
			return 'Retry';
		case AttentionAction.RESOLVE:
			return 'Resolve';
		case AttentionAction.VIEW:
			return 'Open';
		default:
			return action.toString();
	}
}

function itemTypeLabel(item: AttentionItem): string {
	if (item.signalKind === 'discussion_needed') {
		return 'Discussion';
	}
	if (item.signalKind === 'verification_summary') {
		return 'Review';
	}

	switch (item.type) {
		case AttentionItemType.BLOCKED_TASK:
			return 'Blocked';
		case AttentionItemType.PENDING_DECISION:
			return 'Decision';
		case AttentionItemType.GATE_APPROVAL:
			return 'Gate';
		case AttentionItemType.FAILED_TASK:
			return 'Failed';
		case AttentionItemType.ERROR_STATE:
			return 'Follow-up';
		default:
			return 'Attention';
	}
}

function itemDetail(item: AttentionItem): string {
	return item.description || item.blockedReason || item.gateQuestion || item.errorMessage || 'Needs operator attention.';
}

export function AttentionItemCard({
	item,
	projectName,
	pendingAction,
	onOpen,
	onAction,
}: AttentionItemCardProps) {
	const actionInFlight = pendingAction !== undefined;

	return (
		<article className="command-center-attention-item">
			<button
				type="button"
				className="command-center-attention-item__body"
				onClick={() => onOpen(item)}
			>
				<div className="command-center-attention-item__header">
					<span className="command-center-attention-item__type">{itemTypeLabel(item)}</span>
					<span className="command-center-attention-item__project">{projectName}</span>
				</div>
				<div className="command-center-attention-item__title">{item.title}</div>
				<div className="command-center-attention-item__detail">{itemDetail(item)}</div>
			</button>

			<div className="command-center-attention-item__actions">
				{item.availableActions.map((action) => {
					if (action === AttentionAction.UNSPECIFIED) {
						return null;
					}

					if (action === AttentionAction.APPROVE && item.decisionOptions.length > 0) {
						return item.decisionOptions.map((option) => (
							<button
								key={`${item.id}-${option.id}`}
								type="button"
								className="command-center-attention-item__action"
								disabled={actionInFlight}
								onClick={() => void onAction(item, action, option.id)}
							>
								{actionInFlight ? 'Working…' : option.label}
							</button>
						));
					}

					return (
						<button
							key={`${item.id}-${action}`}
							type="button"
							className="command-center-attention-item__action"
							disabled={actionInFlight}
							onClick={() => {
								if (action === AttentionAction.VIEW) {
									onOpen(item);
									return;
								}
								void onAction(item, action);
							}}
						>
							{actionInFlight ? 'Working…' : actionLabel(action)}
						</button>
					);
				})}
			</div>
		</article>
	);
}
