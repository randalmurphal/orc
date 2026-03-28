import { useCallback, useState } from 'react';
import * as DropdownMenu from '@radix-ui/react-dropdown-menu';
import { Icon } from '@/components/ui/Icon';
import { generateHandoff } from '@/lib/api/handoff';
import { useCurrentProjectId } from '@/stores/projectStore';
import { toast } from '@/stores/uiStore';
import {
	HandoffSourceType,
	HandoffTarget,
} from '@/gen/orc/v1/handoff_pb';
import './HandoffActions.css';

interface HandoffActionsProps {
	projectId?: string;
	sourceType: HandoffSourceType;
	sourceId: string;
}

type HandoffCopyAction = 'claude' | 'codex' | 'bootstrap' | 'context';

export function HandoffActions({ projectId, sourceType, sourceId }: HandoffActionsProps) {
	const currentProjectId = useCurrentProjectId();
	const effectiveProjectId = projectId ?? currentProjectId ?? '';
	const [isOpen, setIsOpen] = useState(false);
	const [loadingAction, setLoadingAction] = useState<HandoffCopyAction | null>(null);

	const handleCopy = useCallback(async (action: HandoffCopyAction) => {
		if (!effectiveProjectId) {
			toast.error('No project selected');
			return;
		}

		const target = action === 'codex'
			? HandoffTarget.CODEX
			: HandoffTarget.CLAUDE_CODE;

		setLoadingAction(action);
		try {
			const response = await generateHandoff(effectiveProjectId, sourceType, sourceId, target);
			const text = copiedTextForAction(action, response);
			await navigator.clipboard.writeText(text);
			toast.success(successMessage(action));
			setIsOpen(false);
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Failed to copy handoff content');
		} finally {
			setLoadingAction(null);
		}
	}, [effectiveProjectId, sourceId, sourceType]);

	return (
		<div className="handoff-actions">
			<DropdownMenu.Root open={isOpen} onOpenChange={setIsOpen}>
				<DropdownMenu.Trigger
					className="handoff-trigger"
					disabled={loadingAction !== null}
					aria-label="Handoff actions"
				>
					<Icon name="copy" size={16} />
					<span>Handoff</span>
					<Icon name="chevron-down" size={14} className="handoff-trigger__chevron" />
				</DropdownMenu.Trigger>

				<DropdownMenu.Portal>
					<DropdownMenu.Content
						className="handoff-menu"
						sideOffset={4}
						align="end"
						onCloseAutoFocus={(event) => event.preventDefault()}
					>
						<DropdownMenu.Label className="handoff-menu__header">
							Copy Handoff
						</DropdownMenu.Label>

						<DropdownMenu.Item
							className="handoff-menu__item"
							disabled={loadingAction !== null}
							onSelect={() => void handleCopy('claude')}
						>
							<Icon name="claude" size={16} />
							<span>{loadingLabel('claude', loadingAction, 'Copy Claude command')}</span>
						</DropdownMenu.Item>

						<DropdownMenu.Item
							className="handoff-menu__item"
							disabled={loadingAction !== null}
							onSelect={() => void handleCopy('codex')}
						>
							<Icon name="terminal" size={16} />
							<span>{loadingLabel('codex', loadingAction, 'Copy Codex command')}</span>
						</DropdownMenu.Item>

						<DropdownMenu.Item
							className="handoff-menu__item"
							disabled={loadingAction !== null}
							onSelect={() => void handleCopy('bootstrap')}
						>
							<Icon name="sparkles" size={16} />
							<span>{loadingLabel('bootstrap', loadingAction, 'Copy bootstrap prompt')}</span>
						</DropdownMenu.Item>

						<DropdownMenu.Item
							className="handoff-menu__item"
							disabled={loadingAction !== null}
							onSelect={() => void handleCopy('context')}
						>
							<Icon name="clipboard" size={16} />
							<span>{loadingLabel('context', loadingAction, 'Copy context pack')}</span>
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Portal>
			</DropdownMenu.Root>
		</div>
	);
}

function copiedTextForAction(
	action: HandoffCopyAction,
	response: {
		cliCommand: string;
		bootstrapPrompt: string;
		contextPack: string;
	},
): string {
	switch (action) {
		case 'claude':
		case 'codex':
			return response.cliCommand;
		case 'bootstrap':
			return response.bootstrapPrompt;
		case 'context':
			return response.contextPack;
	}
}

function successMessage(action: HandoffCopyAction): string {
	switch (action) {
		case 'claude':
			return 'Claude handoff command copied';
		case 'codex':
			return 'Codex handoff command copied';
		case 'bootstrap':
			return 'Bootstrap prompt copied';
		case 'context':
			return 'Context pack copied';
	}
}

function loadingLabel(
	action: HandoffCopyAction,
	loadingAction: HandoffCopyAction | null,
	label: string,
): string {
	if (loadingAction === action) {
		return 'Copying...';
	}
	return label;
}
