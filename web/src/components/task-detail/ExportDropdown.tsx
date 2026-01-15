import { useState, useRef, useEffect, useCallback } from 'react';
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
	const dropdownRef = useRef<HTMLDivElement>(null);

	// Close dropdown when clicking outside
	useEffect(() => {
		const handleClickOutside = (e: MouseEvent) => {
			if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
				setIsOpen(false);
			}
		};

		if (isOpen) {
			document.addEventListener('mousedown', handleClickOutside);
		}

		return () => {
			document.removeEventListener('mousedown', handleClickOutside);
		};
	}, [isOpen]);

	const handleExport = useCallback(async (options: {
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
	}, [taskId]);

	return (
		<div className="export-dropdown" ref={dropdownRef}>
			<button
				className="export-trigger"
				onClick={() => setIsOpen(!isOpen)}
				disabled={exporting}
				title="Export task"
			>
				<Icon name="export" size={18} />
				<Icon name={isOpen ? 'chevron-up' : 'chevron-down'} size={14} />
			</button>

			{isOpen && (
				<div className="export-menu">
					<div className="export-menu-header">Export Options</div>
					<button
						className="export-option"
						onClick={() => handleExport({ task_definition: true })}
						disabled={exporting}
					>
						<Icon name="file-text" size={16} />
						<span>Task Definition</span>
					</button>
					<button
						className="export-option"
						onClick={() => handleExport({ final_state: true })}
						disabled={exporting}
					>
						<Icon name="database" size={16} />
						<span>Final State</span>
					</button>
					<button
						className="export-option"
						onClick={() => handleExport({ transcripts: true })}
						disabled={exporting}
					>
						<Icon name="terminal" size={16} />
						<span>Transcripts</span>
					</button>
					<button
						className="export-option"
						onClick={() => handleExport({ context_summary: true })}
						disabled={exporting}
					>
						<Icon name="clipboard" size={16} />
						<span>Context Summary</span>
					</button>
					<div className="export-menu-divider" />
					<button
						className="export-option"
						onClick={() => handleExport({
							task_definition: true,
							final_state: true,
							transcripts: true,
							context_summary: true,
						})}
						disabled={exporting}
					>
						<Icon name="layers" size={16} />
						<span>Export All</span>
					</button>
					<button
						className="export-option"
						onClick={() => handleExport({ to_branch: true })}
						disabled={exporting}
					>
						<Icon name="branch" size={16} />
						<span>Commit to Branch</span>
					</button>
				</div>
			)}
		</div>
	);
}
