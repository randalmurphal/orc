/**
 * ExportDropdown component
 *
 * Action menu for exporting task data in various formats.
 * Uses Radix DropdownMenu for accessibility (keyboard navigation, ARIA).
 */

import { useState, useCallback } from 'react';
import * as DropdownMenu from '@radix-ui/react-dropdown-menu';
import { Icon } from '@/components/ui/Icon';
import { exportTask } from '@/lib/api';
import { toast } from '@/stores/uiStore';
import './ExportDropdown.css';

interface ExportDropdownProps {
	taskId: string;
}

export function ExportDropdown({ taskId }: ExportDropdownProps) {
	const [isOpen, setIsOpen] = useState(false);
	const [exporting, setExporting] = useState(false);

	const handleExport = useCallback(
		async (options: {
			task_definition?: boolean;
			final_state?: boolean;
			transcripts?: boolean;
			context_summary?: boolean;
			to_branch?: boolean;
		}) => {
			setExporting(true);
			try {
				const result = await exportTask(taskId, options);
				if (result.success) {
					toast.success(`Exported to ${result.exported_to}`);
				}
				setIsOpen(false);
			} catch (e) {
				toast.error(e instanceof Error ? e.message : 'Export failed');
			} finally {
				setExporting(false);
			}
		},
		[taskId]
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
							onSelect={() => handleExport({ task_definition: true })}
						>
							<Icon name="file-text" size={16} />
							<span>Task Definition</span>
						</DropdownMenu.Item>

						<DropdownMenu.Item
							className="export-option"
							disabled={exporting}
							onSelect={() => handleExport({ final_state: true })}
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
							onSelect={() => handleExport({ context_summary: true })}
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
									task_definition: true,
									final_state: true,
									transcripts: true,
									context_summary: true,
								})
							}
						>
							<Icon name="layers" size={16} />
							<span>Export All</span>
						</DropdownMenu.Item>

						<DropdownMenu.Item
							className="export-option"
							disabled={exporting}
							onSelect={() => handleExport({ to_branch: true })}
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
