/**
 * ExportDropdown component
 *
 * Action menu for exporting task data in various formats.
 * Uses Radix DropdownMenu for accessibility (keyboard navigation, ARIA).
 */

import { useState, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import * as DropdownMenu from '@radix-ui/react-dropdown-menu';
import { Icon } from '@/components/ui/Icon';
import { taskClient } from '@/lib/client';
import { ExportTaskRequestSchema } from '@/gen/orc/v1/task_pb';
import { toast } from '@/stores/uiStore';
import { useCurrentProjectId } from '@/stores';
import './ExportDropdown.css';

interface ExportDropdownProps {
	taskId: string;
}

export function ExportDropdown({ taskId }: ExportDropdownProps) {
	const projectId = useCurrentProjectId();
	const [isOpen, setIsOpen] = useState(false);
	const [exporting, setExporting] = useState(false);

	const handleExport = useCallback(
		async (options: {
			taskDefinition?: boolean;
			finalState?: boolean;
			transcripts?: boolean;
			contextSummary?: boolean;
			toBranch?: boolean;
		}) => {
			if (!projectId) return;
			setExporting(true);
			try {
				const result = await taskClient.exportTask(
					create(ExportTaskRequestSchema, {
						projectId,
						taskId,
						taskDefinition: options.taskDefinition,
						finalState: options.finalState,
						transcripts: options.transcripts,
						contextSummary: options.contextSummary,
						toBranch: options.toBranch ?? false,
					})
				);
				if (result.success) {
					toast.success(`Exported to ${result.exportedTo}`);
				}
				setIsOpen(false);
			} catch (e) {
				toast.error(e instanceof Error ? e.message : 'Export failed');
			} finally {
				setExporting(false);
			}
		},
		[projectId, taskId]
	);

	return (
		<div className="export-dropdown">
			<DropdownMenu.Root open={isOpen} onOpenChange={setIsOpen}>
				<DropdownMenu.Trigger
					className="export-trigger"
					disabled={exporting}
					aria-label="Export task"
				>
					<Icon name="export" size={18} />
					<Icon name="chevron-down" size={14} className="chevron" />
				</DropdownMenu.Trigger>

				<DropdownMenu.Portal>
					<DropdownMenu.Content
						className="export-menu"
						sideOffset={4}
						align="end"
						onCloseAutoFocus={(e) => e.preventDefault()}
					>
						<DropdownMenu.Label className="export-menu-header">
							Export Options
						</DropdownMenu.Label>

						<DropdownMenu.Item
							className="export-option"
							disabled={exporting}
							onSelect={() => handleExport({ taskDefinition: true })}
						>
							<Icon name="file-text" size={16} />
							<span>Task Definition</span>
						</DropdownMenu.Item>

						<DropdownMenu.Item
							className="export-option"
							disabled={exporting}
							onSelect={() => handleExport({ finalState: true })}
						>
							<Icon name="database" size={16} />
							<span>Final State</span>
						</DropdownMenu.Item>

						<DropdownMenu.Item
							className="export-option"
							disabled={exporting}
							onSelect={() => handleExport({ transcripts: true })}
						>
							<Icon name="terminal" size={16} />
							<span>Transcripts</span>
						</DropdownMenu.Item>

						<DropdownMenu.Item
							className="export-option"
							disabled={exporting}
							onSelect={() => handleExport({ contextSummary: true })}
						>
							<Icon name="clipboard" size={16} />
							<span>Context Summary</span>
						</DropdownMenu.Item>

						<DropdownMenu.Separator className="export-menu-divider" />

						<DropdownMenu.Item
							className="export-option"
							disabled={exporting}
							onSelect={() =>
								handleExport({
									taskDefinition: true,
									finalState: true,
									transcripts: true,
									contextSummary: true,
								})
							}
						>
							<Icon name="layers" size={16} />
							<span>Export All</span>
						</DropdownMenu.Item>

						<DropdownMenu.Item
							className="export-option"
							disabled={exporting}
							onSelect={() => handleExport({ toBranch: true })}
						>
							<Icon name="branch" size={16} />
							<span>Commit to Branch</span>
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Portal>
			</DropdownMenu.Root>
		</div>
	);
}
